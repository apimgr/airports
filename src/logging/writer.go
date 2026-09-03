package logging

import (
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogWriter owns a single log file: appending raw plain-text lines to it,
// and rotating/pruning it against its configured policy. It is safe for
// concurrent use. The last-rotation timestamp is tracked in memory only —
// the owning Manager is created once at process startup and lives for the
// life of the process (per AI.md, the scheduler runs continuously from
// startup to shutdown), so no on-disk sidecar file is needed, and none is
// ever left behind — even when the log's retention policy is "none".
type LogWriter struct {
	mu           sync.Mutex
	dir          string
	name         string
	cfg          FileConfig
	file         *os.File
	lastRotation time.Time
}

// NewWriter creates a LogWriter for the given FileConfig rooted at dir (the
// project's logs directory, e.g. from paths.GetDefaultDirs). name is a
// short identifier used only in warning messages (e.g. "access", "audit").
// It does not open the underlying file — the file opens lazily on first
// Write, so a disabled or not-yet-used log never creates an empty file. If
// a log file already exists on disk (e.g. left over from a previous
// process), its modification time seeds the initial rotation baseline.
func NewWriter(dir, name string, cfg FileConfig) *LogWriter {
	w := &LogWriter{dir: dir, name: name, cfg: cfg}
	if info, err := os.Stat(filepath.Join(dir, cfg.Filename)); err == nil {
		w.lastRotation = info.ModTime()
	}
	return w
}

// Path returns the absolute path of this writer's active log file.
func (w *LogWriter) Path() string {
	return filepath.Join(w.dir, w.cfg.Filename)
}

// ensureOpen opens (creating parent dirs as needed) the active log file for
// appending, if it is not already open. Caller must hold w.mu.
func (w *LogWriter) ensureOpen() error {
	if w.file != nil {
		return nil
	}
	if err := os.MkdirAll(w.dir, 0700); err != nil {
		return fmt.Errorf("logging: create log dir %q: %w", w.dir, err)
	}
	f, err := os.OpenFile(w.Path(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return fmt.Errorf("logging: open log file %q: %w", w.Path(), err)
	}
	w.file = f
	return nil
}

// Write appends a single raw plain-text line to the log file. A trailing
// newline is added if line does not already end with one. Per AI.md,
// log files never contain ANSI escapes, emojis, or control characters
// other than the line-ending "\n" — callers are responsible for producing
// clean text; Write does not reformat or sanitize the line itself.
func (w *LogWriter) Write(line string) error {
	if !w.cfg.Enabled {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.ensureOpen(); err != nil {
		return err
	}
	if !strings.HasSuffix(line, "\n") {
		line += "\n"
	}
	_, err := w.file.WriteString(line)
	return err
}

// Close closes the underlying file handle, if open.
func (w *LogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// RotateIfDue rotates this log file's active contents if its configured
// rotate policy is due, relative to now. It is idempotent: calling it more
// often than the configured cadence is a no-op until the policy is
// actually due. It never returns an error for conditions that are safe to
// skip (missing/empty log file, disabled log) — only genuine I/O failures
// are returned, and even those are expected to be logged as warnings by
// the caller rather than treated as fatal.
func (w *LogWriter) RotateIfDue(now time.Time) (bool, error) {
	if !w.cfg.Enabled {
		return false, nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	info, err := os.Stat(w.Path())
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("logging: stat %q: %w", w.Path(), err)
	}
	if info.Size() == 0 {
		return false, nil
	}

	policy, err := ParseRotatePolicy(w.cfg.Rotate)
	if err != nil {
		return false, err
	}

	due := false
	if policy.Interval != IntervalNever {
		if w.lastRotation.IsZero() || now.Sub(w.lastRotation) >= policy.Interval.Duration() {
			due = true
		}
	}
	if policy.SizeBytes > 0 && info.Size() >= policy.SizeBytes {
		due = true
	}
	if !due {
		return false, nil
	}

	if err := w.rotateLocked(now); err != nil {
		return false, err
	}
	return true, nil
}

// rotateLocked performs the actual rename/reopen/compress/prune sequence.
// Caller must hold w.mu.
func (w *LogWriter) rotateLocked(now time.Time) error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return fmt.Errorf("logging: close %q before rotation: %w", w.Path(), err)
		}
		w.file = nil
	}

	rotatedPath := w.uniqueRotatedPath(now)
	if err := os.Rename(w.Path(), rotatedPath); err != nil {
		return fmt.Errorf("logging: rename %q to %q: %w", w.Path(), rotatedPath, err)
	}
	w.lastRotation = now

	retention, err := ParseRetentionPolicy(w.cfg.Keep)
	if err != nil {
		return err
	}

	if retention.Mode == RetentionNone {
		if err := os.Remove(rotatedPath); err != nil {
			return fmt.Errorf("logging: remove rotated backup %q: %w", rotatedPath, err)
		}
		return nil
	}

	if w.cfg.Compress {
		if _, err := gzipFile(rotatedPath); err != nil {
			return fmt.Errorf("logging: compress rotated backup %q: %w", rotatedPath, err)
		}
	}

	return w.applyRetention(retention)
}

// uniqueRotatedPath returns a rotated-backup filename for this log file
// that does not already exist, appending a numeric suffix on collision
// (e.g. two rotations triggered within the same second).
func (w *LogWriter) uniqueRotatedPath(now time.Time) string {
	base := w.Path() + "." + now.UTC().Format("2006-01-02T15-04-05")
	candidate := base
	for i := 1; ; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = fmt.Sprintf("%s.%d", base, i)
	}
}

// gzipFile compresses src to src+".gz" and removes src on success,
// returning the compressed file's path.
func gzipFile(src string) (string, error) {
	dstPath := src + ".gz"
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()

	out, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0640)
	if err != nil {
		return "", err
	}

	gw := gzip.NewWriter(out)
	if _, err := io.Copy(gw, in); err != nil {
		gw.Close()
		out.Close()
		os.Remove(dstPath)
		return "", err
	}
	if err := gw.Close(); err != nil {
		out.Close()
		os.Remove(dstPath)
		return "", err
	}
	if err := out.Close(); err != nil {
		os.Remove(dstPath)
		return "", err
	}
	if err := os.Remove(src); err != nil {
		return "", err
	}
	return dstPath, nil
}

// rotatedBackup describes one previously-rotated log backup discovered on
// disk, for retention pruning.
type rotatedBackup struct {
	path    string
	modTime time.Time
}

// listRotatedBackups returns every rotated backup for this log file,
// sorted oldest first. It matches "{filename}.<anything>".
func (w *LogWriter) listRotatedBackups() ([]rotatedBackup, error) {
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return nil, err
	}

	prefix := w.cfg.Filename + "."
	var backups []rotatedBackup
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		backups = append(backups, rotatedBackup{path: filepath.Join(w.dir, name), modTime: info.ModTime()})
	}

	sort.Slice(backups, func(i, j int) bool {
		return backups[i].modTime.Before(backups[j].modTime)
	})
	return backups, nil
}

// applyRetention prunes rotated backups for this log file down to the
// given retention policy, deleting whichever backups fall outside it.
func (w *LogWriter) applyRetention(policy RetentionPolicy) error {
	if policy.Mode == RetentionForever {
		return nil
	}

	backups, err := w.listRotatedBackups()
	if err != nil {
		return fmt.Errorf("logging: list rotated backups for %q: %w", w.cfg.Filename, err)
	}

	var toDelete []rotatedBackup
	switch policy.Mode {
	case RetentionNone:
		toDelete = backups
	case RetentionCount:
		if policy.N < 0 {
			policy.N = 0
		}
		if len(backups) > policy.N {
			toDelete = backups[:len(backups)-policy.N]
		}
	case RetentionDays, RetentionWeeks, RetentionMonths:
		cutoff := time.Now().Add(-policy.MaxAge())
		for _, b := range backups {
			if b.modTime.Before(cutoff) {
				toDelete = append(toDelete, b)
			}
		}
	}

	var firstErr error
	for _, b := range toDelete {
		if err := os.Remove(b.path); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// warnf logs a non-fatal warning to the standard logger, per AI.md's
// requirement that a single log file's failure never stops rotation of
// the others.
func warnf(format string, args ...any) {
	log.Printf("logging: warning: "+format, args...)
}

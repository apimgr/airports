package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/subtle"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RestoreAuth is the outcome of AuthorizeRestore, per AI.md PART 21
// "Restore Authorization".
type RestoreAuth int

const (
	// RestoreAllowedEmptyDB: database is empty (first-run) — nothing to
	// protect, always allowed.
	RestoreAllowedEmptyDB RestoreAuth = iota
	// RestoreAllowedRoot: running as root — allowed, caller must still
	// obtain interactive confirmation before proceeding.
	RestoreAllowedRoot
	// RestoreRequiresToken: running as the service user — allowed only if
	// providedToken matches configuredToken.
	RestoreRequiresToken
	// RestoreDenied: none of the above conditions were met.
	RestoreDenied
)

// ErrRestoreDenied is returned by AuthorizeRestore when no authorization
// path applies.
var ErrRestoreDenied = errors.New("backup: restore denied — requires empty database, root, or a valid operator token")

// ErrInvalidToken is returned by AuthorizeRestore when a token was provided
// but does not match the configured server.token.
var ErrInvalidToken = errors.New("backup: restore denied — invalid operator token")

// AuthorizeRestore implements AI.md PART 21 "Restore Authorization" table:
// empty DB and root are always allowed; a non-root process must present a
// token matching the configured server.token (compared in constant time,
// per backend-rules.md's ban on == / bytes.Equal / strings.EqualFold for
// secret comparisons); anything else is denied.
func AuthorizeRestore(dbIsEmpty, runningAsRoot bool, providedToken, configuredToken string) (RestoreAuth, error) {
	if dbIsEmpty {
		return RestoreAllowedEmptyDB, nil
	}
	if runningAsRoot {
		return RestoreAllowedRoot, nil
	}
	if configuredToken == "" {
		return RestoreDenied, ErrRestoreDenied
	}
	if providedToken == "" {
		return RestoreDenied, ErrRestoreDenied
	}
	if subtle.ConstantTimeCompare([]byte(providedToken), []byte(configuredToken)) != 1 {
		return RestoreDenied, ErrInvalidToken
	}
	return RestoreRequiresToken, nil
}

// ErrPasswordRequired is returned by Restore when the backup is encrypted
// but no password was supplied, matching the API's documented
// "password_required" error condition.
var ErrPasswordRequired = errors.New("backup: encrypted backup requires a password")

var (
	ErrBaseMissing = errors.New("backup: incremental base backup is missing")
	ErrBaseCorrupt = errors.New("backup: incremental base backup checksum mismatch")
)

// VersionMismatchWarning is returned (never as a hard failure) by Restore
// alongside a successful restore when the backup's app_version differs from
// the running app's version, per AI.md PART 21 "Restore Verification" ->
// "Version compatible | ... | Warning: version mismatch". Callers should
// surface .Error() to the operator/audit log but must still treat the
// restore as successful.
type VersionMismatchWarning struct {
	BackupVersion  string
	RunningVersion string
}

func (w *VersionMismatchWarning) Error() string {
	return fmt.Sprintf("backup: version mismatch — backup was created by app_version %q, running version is %q; schema updates will be applied if needed", w.BackupVersion, w.RunningVersion)
}

// BackupRestoreOptions configures Restore.
type BackupRestoreOptions struct {
	ConfigDir  string
	DataDir    string
	Password   string
	AppVersion string
	Limits     RestoreLimits
}

// RestoreLimits bounds compressed input and expanded archive contents.
type RestoreLimits struct {
	MaxArchiveSize  int64
	MaxExpandedSize int64
	MaxFiles        int
	MaxFileSize     int64
	MaxRatio        int
}

func normalizedRestoreLimits(limits RestoreLimits) RestoreLimits {
	if limits.MaxArchiveSize <= 0 {
		limits.MaxArchiveSize = 1 << 30
	}
	if limits.MaxExpandedSize <= 0 {
		limits.MaxExpandedSize = 4 << 30
	}
	if limits.MaxFiles <= 0 {
		limits.MaxFiles = 10000
	}
	if limits.MaxFileSize <= 0 {
		limits.MaxFileSize = 512 << 20
	}
	if limits.MaxRatio <= 0 {
		limits.MaxRatio = 100
	}
	return limits
}

// Restore implements AI.md PART 21 "Restore Verification" + "Restore
// Behavior": runs the full verification chain, then extracts the archive
// directly into ConfigDir/DataDir (overwriting current config and
// database), guarding against zip-slip path traversal. Authorization
// (AuthorizeRestore) is a separate, earlier step — callers must call it
// before Restore. A non-nil *VersionMismatchWarning returned alongside a
// nil error indicates a successful restore with a version mismatch that the
// caller should surface, not a failure.
func RestoreBackupArchive(archivePath string, opts BackupRestoreOptions) error {
	if strings.HasSuffix(strings.ToLower(archivePath), ".enc") && opts.Password == "" {
		return ErrPasswordRequired
	}
	info, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("restore: file exists: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("restore: file readable: backup file is empty")
	}
	limits := normalizedRestoreLimits(opts.Limits)
	if info.Size() > limits.MaxArchiveSize {
		return fmt.Errorf("restore: archive exceeds server.restore.max_archive_size")
	}
	compressedSize := info.Size()

	manifest, plaintext, err := decodeBackup(archivePath, opts.Password)
	if err != nil {
		return fmt.Errorf("restore: format valid: %w", err)
	}

	if err := validateExpandedArchive(plaintext, info.Size(), limits); err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	if err := verifyChecksum(manifest, plaintext); err != nil {
		return fmt.Errorf("restore: checksum valid: %w", err)
	}
	if manifest.Type == "incremental" {
		basePath := filepath.Join(filepath.Dir(archivePath), manifest.BaseBackup)
		if filepath.Base(basePath) != manifest.BaseBackup {
			return fmt.Errorf("restore: invalid base backup name")
		}
		baseInfo, statErr := os.Stat(basePath)
		if statErr != nil {
			return fmt.Errorf("restore: %w", ErrBaseMissing)
		}
		if baseInfo.Size() > limits.MaxArchiveSize-compressedSize {
			return fmt.Errorf("restore: combined archives exceed server.restore.max_archive_size")
		}
		compressedSize += baseInfo.Size()
		baseManifest, basePlaintext, baseErr := decodeBackup(basePath, opts.Password)
		if baseErr != nil {
			if os.IsNotExist(baseErr) {
				return fmt.Errorf("restore: %w", ErrBaseMissing)
			}
			return fmt.Errorf("restore: load incremental base: %w", baseErr)
		}
		if baseManifest.Type != "full" {
			return fmt.Errorf("restore: %w: base is not a full backup", ErrBaseCorrupt)
		}
		if err := validateExpandedArchive(basePlaintext, baseInfo.Size(), limits); err != nil {
			return fmt.Errorf("restore: base archive: %w", err)
		}
		if err := verifyChecksum(baseManifest, basePlaintext); err != nil {
			return fmt.Errorf("restore: %w: %v", ErrBaseCorrupt, err)
		}
		if baseManifest.Checksum != manifest.BaseSHA256 {
			return fmt.Errorf("restore: %w", ErrBaseCorrupt)
		}
		if err := validateCombinedManifestLimits(baseManifest, manifest, compressedSize, limits); err != nil {
			return fmt.Errorf("restore: combined incremental: %w", err)
		}
		plaintext, err = mergeIncrementalArchives(basePlaintext, plaintext, manifest.Deleted)
		if err != nil {
			return fmt.Errorf("restore: merge incremental: %w", err)
		}
	}
	if err := validateExpandedArchive(plaintext, compressedSize, limits); err != nil {
		return fmt.Errorf("restore: %w", err)
	}

	if err := os.MkdirAll(opts.ConfigDir, 0o755); err != nil {
		return fmt.Errorf("restore: prepare config dir: %w", err)
	}
	if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
		return fmt.Errorf("restore: prepare data dir: %w", err)
	}

	if err := extractRestoreArchive(plaintext, opts.ConfigDir, opts.DataDir); err != nil {
		return fmt.Errorf("restore: extraction: %w", err)
	}

	if opts.AppVersion != "" && manifest.AppVersion != "" && manifest.AppVersion != opts.AppVersion {
		return &VersionMismatchWarning{BackupVersion: manifest.AppVersion, RunningVersion: opts.AppVersion}
	}

	return nil
}

func validateExpandedArchive(archive []byte, compressedSize int64, limits RestoreLimits) error {
	limits = normalizedRestoreLimits(limits)
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("invalid expanded archive: %w", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	var total int64
	files := 0
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("invalid expanded archive: %w", err)
		}
		if err := validateArchiveHeader(h); err != nil {
			return err
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		files++
		if files > limits.MaxFiles {
			return fmt.Errorf("archive exceeds server.restore.max_files")
		}
		if h.Size < 0 {
			return fmt.Errorf("invalid negative size for %q", h.Name)
		}
		if h.Size > limits.MaxFileSize {
			return fmt.Errorf("file %q exceeds server.restore.max_file_size", h.Name)
		}
		copied, copyErr := io.CopyN(io.Discard, tr, limits.MaxFileSize+1)
		if copyErr != nil && copyErr != io.EOF {
			return fmt.Errorf("invalid archive file %q: %w", h.Name, copyErr)
		}
		if copied > limits.MaxFileSize {
			return fmt.Errorf("file %q exceeds server.restore.max_file_size", h.Name)
		}
		if copied > limits.MaxExpandedSize-total {
			return fmt.Errorf("archive exceeds server.restore.max_expanded_size")
		}
		total += copied
	}
	return validateExpansionRatio(total, compressedSize, limits.MaxRatio)
}

func validateCombinedManifestLimits(base, incremental *Manifest, compressedSize int64, limits RestoreLimits) error {
	limits = normalizedRestoreLimits(limits)
	files := make(map[string]BackupFile, len(base.Files)+len(incremental.Files))
	for _, file := range base.Files {
		files[file.Path] = file
	}
	for _, file := range incremental.Files {
		files[file.Path] = file
	}
	for _, name := range incremental.Deleted {
		delete(files, name)
	}
	if len(files) > limits.MaxFiles {
		return fmt.Errorf("archive exceeds server.restore.max_files")
	}
	var total int64
	for _, file := range files {
		if file.Size < 0 || file.Size > limits.MaxFileSize {
			return fmt.Errorf("file %q exceeds server.restore.max_file_size", file.Path)
		}
		if file.Size > limits.MaxExpandedSize-total {
			return fmt.Errorf("archive exceeds server.restore.max_expanded_size")
		}
		total += file.Size
	}
	return validateExpansionRatio(total, compressedSize, limits.MaxRatio)
}

func validateExpansionRatio(expanded, compressed int64, maxRatio int) error {
	if compressed <= 0 {
		return fmt.Errorf("archive exceeds server.restore.max_ratio")
	}
	maxInt64 := int64(^uint64(0) >> 1)
	ratioLimit := maxInt64
	if int64(maxRatio) <= maxInt64/compressed {
		ratioLimit = int64(maxRatio) * compressed
	}
	if expanded > ratioLimit {
		return fmt.Errorf("archive exceeds server.restore.max_ratio")
	}
	return nil
}

func verifyChecksum(manifest *Manifest, plaintext []byte) error {
	if manifest.legacy {
		checksum, err := legacyArchiveChecksum(plaintext)
		if err != nil {
			return err
		}
		if !strings.EqualFold(checksum, manifest.Checksum) {
			return fmt.Errorf("legacy checksum mismatch (want %s, got %s)", manifest.Checksum, checksum)
		}
		return nil
	}
	files, sum, err := archiveFileManifest(plaintext)
	if err != nil {
		return err
	}
	if sum != manifest.Checksum {
		return fmt.Errorf("mismatch (want %s, got %s)", manifest.Checksum, sum)
	}
	if len(files) != len(manifest.Files) {
		return fmt.Errorf("manifest file list does not match archive")
	}
	for i := range files {
		if files[i] != manifest.Files[i] {
			return fmt.Errorf("manifest file entry does not match archive at %q", files[i].Path)
		}
	}
	return nil
}

// extractRestoreArchive extracts a plaintext tar.gz archive produced by
// buildTarGz back onto disk, mapping the archive's "config/" and "data/"
// prefixes back to configDir/dataDir respectively. It reuses the exact
// zip-slip guard pattern from the previous main.go restoreBackup
// implementation: any entry that would resolve outside its target root is
// rejected outright.
func extractRestoreArchive(archive []byte, configDir, dataDir string) error {
	_, err := extractTarGzTo(archive, func(name string) (destPath, root string, ok bool) {
		switch {
		case name == "config" || strings.HasPrefix(name, "config/"):
			rel := strings.TrimPrefix(name, "config")
			rel = strings.TrimPrefix(rel, "/")
			return filepath.Join(configDir, filepath.FromSlash(rel)), configDir, true
		case name == "data" || strings.HasPrefix(name, "data/"):
			rel := strings.TrimPrefix(name, "data")
			rel = strings.TrimPrefix(rel, "/")
			return filepath.Join(dataDir, filepath.FromSlash(rel)), dataDir, true
		default:
			return "", "", false
		}
	})
	return err
}

type archiveEntry struct {
	header  *tar.Header
	content []byte
}

func mergeIncrementalArchives(base, incremental []byte, deleted []string) ([]byte, error) {
	entries := make(map[string]archiveEntry)
	read := func(archive []byte) error {
		gz, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			return err
		}
		defer func() { _ = gz.Close() }()
		tr := tar.NewReader(gz)
		for {
			h, err := tr.Next()
			if err == io.EOF {
				return nil
			}
			if err != nil {
				return err
			}
			if h.Name == "manifest.json" {
				continue
			}
			data, err := io.ReadAll(tr)
			if err != nil {
				return err
			}
			copyHeader := *h
			entries[h.Name] = archiveEntry{header: &copyHeader, content: data}
		}
	}
	if err := read(base); err != nil {
		return nil, err
	}
	if err := read(incremental); err != nil {
		return nil, err
	}
	for _, name := range deleted {
		delete(entries, name)
	}
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	tw := tar.NewWriter(zw)
	for _, name := range names {
		entry := entries[name]
		h := *entry.header
		h.Size = int64(len(entry.content))
		if err := tw.WriteHeader(&h); err != nil {
			return nil, err
		}
		if _, err := tw.Write(entry.content); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

package backup

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// RestoreOptions configures Restore.
type RestoreOptions struct {
	ConfigDir  string
	DataDir    string
	Password   string
	AppVersion string
}

// Restore implements AI.md PART 21 "Restore Verification" + "Restore
// Behavior": runs the full verification chain, then extracts the archive
// directly into ConfigDir/DataDir (overwriting current config and
// database), guarding against zip-slip path traversal. Authorization
// (AuthorizeRestore) is a separate, earlier step — callers must call it
// before Restore. A non-nil *VersionMismatchWarning returned alongside a
// nil error indicates a successful restore with a version mismatch that the
// caller should surface, not a failure.
func Restore(archivePath string, opts RestoreOptions) error {
	info, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("restore: file exists: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("restore: file readable: backup file is empty")
	}

	manifest, payload, err := readContainer(archivePath)
	if err != nil {
		return fmt.Errorf("restore: format valid: %w", err)
	}

	plaintext := payload
	if manifest.Encrypted {
		if opts.Password == "" {
			return ErrPasswordRequired
		}
		plaintext, err = DecryptArchive(opts.Password, payload)
		if err != nil {
			return fmt.Errorf("restore: decrypt test: %w", err)
		}
	}

	if err := verifyChecksum(manifest, plaintext); err != nil {
		return fmt.Errorf("restore: checksum valid: %w", err)
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

func verifyChecksum(manifest *Manifest, plaintext []byte) error {
	sum := checksumOf(plaintext)
	if !strings.EqualFold(sum, manifest.Checksum) {
		return fmt.Errorf("mismatch (want %s, got %s)", manifest.Checksum, sum)
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

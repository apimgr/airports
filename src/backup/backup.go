package backup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/airports/src/db"
	paths "github.com/apimgr/airports/src/paths"
	_ "modernc.org/sqlite"
)

// ErrEncryptionRequired is returned by Create when a compliance standard is
// enabled but no encryption password was supplied, per AI.md PART 21
// "Compliance Mode Enforcement".
var ErrEncryptionRequired = errors.New("backup: compliance mode requires an encryption password to be set (server.backup.encryption_password)")

// ErrDiskFull is returned by Create when the preflight free-space check
// fails, per AI.md PART 21 "Backup Creation Flow" step 2.
var ErrDiskFull = errors.New("backup: insufficient free space or disk usage above threshold")

// containerMagic identifies the legacy unencrypted manifest wrapper accepted by readContainer.
// New backups use a tar.gz archive with manifest.json inside it.
var containerMagic = [4]byte{'A', 'B', 'K', 'P'}

// BackupArchiveOptions configures Create.
type BackupArchiveOptions struct {
	ConfigDir          string
	DataDir            string
	DatabaseSnapshot   func(context.Context, string) error
	IncludeSSL         bool
	IncludeData        bool
	EncryptionPassword string
	ComplianceEnabled  bool
	AppVersion         string
	CreatedBy          string
	BackupType         string
	SnapshotAt         time.Time
	BaseBackup         string
	BaseArchivePath    string
	BaseSHA256         string
	Deleted            []string
	SnapshotArchive    []byte
	SnapshotContents   []string
	// Contents optionally overrides the auto-discovered file list (mainly
	// for tests); when nil, Create walks ConfigDir/DataDir itself.
	Contents []string
}

// diskThresholdPercent is the default disk-usage-percent above which backup
// creation is skipped, per AI.md PART 21 "Backup Creation Flow" step 2.
const diskThresholdPercent = 90

// Create builds a backup archive at destPath following AI.md PART 21
// "Backup Creation Flow" / "How Encryption Works": the tar.gz is built
// entirely in memory, optionally AES-256-GCM/Argon2id encrypted, and only
// then written to disk atomically (tmp file + rename).
func CreateBackupArchive(destPath string, opts BackupArchiveOptions) error {
	if opts.ComplianceEnabled && opts.EncryptionPassword == "" {
		return ErrEncryptionRequired
	}

	if err := preflightDiskSpace(filepath.Dir(destPath)); err != nil {
		return err
	}
	snapshot := opts.SnapshotAt
	if snapshot.IsZero() {
		snapshot = time.Now().UTC()
	}

	archiveBytes, contents := opts.SnapshotArchive, opts.SnapshotContents
	var err error
	if archiveBytes == nil {
		archiveBytes, contents, err = buildTarGz(opts)
		if err != nil {
			return fmt.Errorf("backup: build archive: %w", err)
		}
	}

	backupType := opts.BackupType
	if backupType == "" {
		backupType = "full"
	}
	if backupType != "full" && backupType != "incremental" {
		return fmt.Errorf("backup: invalid backup type %q", backupType)
	}
	if backupType == "incremental" {
		if opts.BaseBackup == "" || opts.BaseArchivePath == "" {
			return fmt.Errorf("backup: incremental requires base backup name and path")
		}
		baseManifest, baseArchive, err := decodeBackup(opts.BaseArchivePath, opts.EncryptionPassword)
		if err != nil {
			return fmt.Errorf("backup: read incremental base: %w", err)
		}
		if baseManifest.Type != "full" {
			return fmt.Errorf("backup: incremental base is not a full backup")
		}
		if err := validateExpandedArchive(baseArchive, 1<<30, RestoreLimits{}); err != nil {
			return fmt.Errorf("backup: incremental base is corrupt: %w", err)
		}
		if err := verifyChecksum(baseManifest, baseArchive); err != nil {
			return fmt.Errorf("backup: incremental base is corrupt: %w", err)
		}
		if filepath.Base(opts.BaseArchivePath) != opts.BaseBackup {
			return fmt.Errorf("backup: incremental base name does not match base path")
		}
		baseFiles := baseManifest.Files
		if baseManifest.legacy {
			baseFiles, _, err = archiveFileManifest(baseArchive)
			if err != nil {
				return fmt.Errorf("backup: read legacy base file list: %w", err)
			}
		}
		archiveBytes, opts.Deleted, err = filterIncrementalArchive(archiveBytes, baseFiles)
		if err != nil {
			return fmt.Errorf("backup: filter incremental archive: %w", err)
		}
		opts.BaseSHA256 = baseManifest.Checksum
	}
	files, checksum, err := archiveFileManifest(archiveBytes)
	if err != nil {
		return fmt.Errorf("backup: checksum archive contents: %w", err)
	}

	manifest := &Manifest{
		Version:    ManifestVersion,
		CreatedAt:  snapshot.Format(time.RFC3339),
		CreatedBy:  opts.CreatedBy,
		AppVersion: opts.AppVersion,
		Contents:   contents,
		Encrypted:  opts.EncryptionPassword != "",
		Type:       backupType,
		SnapshotAt: snapshot.Format(time.RFC3339),
		Files:      files,
		BaseBackup: opts.BaseBackup,
		BaseSHA256: opts.BaseSHA256,
		Deleted:    opts.Deleted,
		Checksum:   checksum,
	}
	if manifest.CreatedBy == "" {
		manifest.CreatedBy = "operator"
	}

	if opts.EncryptionPassword != "" {
		manifest.EncryptionMethod = "AES-256-GCM"
	}

	manifestJSON, err := manifest.Marshal()
	if err != nil {
		return err
	}
	archiveBytes, err = appendManifest(archiveBytes, manifestJSON)
	if err != nil {
		return fmt.Errorf("backup: include manifest: %w", err)
	}
	payload := archiveBytes
	if opts.EncryptionPassword != "" {
		payload, err = EncryptArchiveWithBase(opts.EncryptionPassword, archiveBytes, opts.BaseBackup)
		if err != nil {
			return fmt.Errorf("backup: encrypt archive: %w", err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("backup: create backup directory: %w", err)
	}

	tmpPath := destPath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0o600); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("backup: write archive: %w", err)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("backup: finalize backup file: %w", err)
	}

	return nil
}

// CreateBackupPair captures source files and the database once, then writes
// the incremental and full archives from that identical in-memory snapshot.
func CreateBackupPair(incrementalPath, fullPath string, opts BackupArchiveOptions) error {
	if opts.ComplianceEnabled && opts.EncryptionPassword == "" {
		return ErrEncryptionRequired
	}
	if err := preflightDiskSpace(filepath.Dir(incrementalPath)); err != nil {
		return err
	}
	if err := preflightDiskSpace(filepath.Dir(fullPath)); err != nil {
		return err
	}
	snapshot := opts.SnapshotAt
	if snapshot.IsZero() {
		snapshot = time.Now().UTC()
	}
	opts.SnapshotAt = snapshot
	archive, contents, err := buildTarGz(opts)
	if err != nil {
		return fmt.Errorf("backup: build shared snapshot: %w", err)
	}
	incOpts := opts
	incOpts.BackupType = "incremental"
	incOpts.SnapshotArchive = archive
	incOpts.SnapshotContents = contents
	if err := CreateBackupArchive(incrementalPath, incOpts); err != nil {
		return err
	}
	fullOpts := opts
	fullOpts.BackupType = "full"
	fullOpts.BaseBackup = ""
	fullOpts.BaseArchivePath = ""
	fullOpts.BaseSHA256 = ""
	fullOpts.Deleted = nil
	fullOpts.SnapshotArchive = archive
	fullOpts.SnapshotContents = contents
	if err := CreateBackupArchive(fullPath, fullOpts); err != nil {
		_ = os.Remove(incrementalPath)
		return err
	}
	return nil
}

// writeContainer writes the legacy unencrypted wrapper for compatibility tests.
func writeContainer(path string, manifestJSON, payload []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("backup: create backup file: %w", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := f.Write(containerMagic[:]); err != nil {
		return fmt.Errorf("backup: write header: %w", err)
	}
	var lenBuf [4]byte
	binary.BigEndian.PutUint32(lenBuf[:], uint32(len(manifestJSON)))
	if _, err := f.Write(lenBuf[:]); err != nil {
		return fmt.Errorf("backup: write header: %w", err)
	}
	if _, err := f.Write(manifestJSON); err != nil {
		return fmt.Errorf("backup: write manifest: %w", err)
	}
	if _, err := f.Write(payload); err != nil {
		return fmt.Errorf("backup: write payload: %w", err)
	}
	return nil
}

// readContainer reads a legacy unencrypted manifest wrapper or a tar.gz archive.
func readContainer(path string) (*Manifest, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("backup: read backup file: %w", err)
	}
	if len(data) < 8 {
		return nil, nil, fmt.Errorf("backup: file too small to be a valid backup")
	}
	if [4]byte{data[0], data[1], data[2], data[3]} != containerMagic {
		manifest, err := manifestFromArchive(data)
		if err != nil {
			return nil, nil, err
		}
		return manifest, data, nil
	}
	manifestLen := binary.BigEndian.Uint32(data[4:8])
	if int(8+manifestLen) > len(data) {
		return nil, nil, fmt.Errorf("backup: corrupted manifest length")
	}
	manifestJSON := data[8 : 8+manifestLen]
	payload := data[8+manifestLen:]

	manifest, err := UnmarshalManifest(manifestJSON)
	if err != nil {
		return nil, nil, err
	}
	return manifest, payload, nil
}

func manifestFromArchive(archive []byte) (*Manifest, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("backup: open tar.gz: %w", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("backup: read archive: %w", err)
		}
		if h.Name == "manifest.json" {
			data, err := io.ReadAll(io.LimitReader(tr, 1<<20))
			if err != nil {
				return nil, fmt.Errorf("backup: read manifest: %w", err)
			}
			return UnmarshalManifest(data)
		}
	}
	return nil, fmt.Errorf("backup: manifest.json is missing from archive")
}

func appendManifest(archive, manifest []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	tw := tar.NewWriter(zw)
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if h.Name == "manifest.json" {
			return nil, fmt.Errorf("manifest.json already exists in archive")
		}
		if err := tw.WriteHeader(h); err != nil {
			return nil, err
		}
		if _, err := io.Copy(tw, tr); err != nil {
			return nil, err
		}
	}
	if err := tw.WriteHeader(&tar.Header{Name: "manifest.json", Mode: 0o600, Size: int64(len(manifest)), Typeflag: tar.TypeReg, ModTime: time.Now().UTC()}); err != nil {
		return nil, err
	}
	if _, err := tw.Write(manifest); err != nil {
		return nil, err
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func archiveFileManifest(archive []byte) ([]BackupFile, string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	files := make([]BackupFile, 0)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, "", err
		}
		if err := validateArchiveHeader(header); err != nil {
			return nil, "", err
		}
		if header.Name == "manifest.json" || header.Typeflag == tar.TypeDir {
			continue
		}
		if header.Typeflag != tar.TypeReg {
			continue
		}
		contentHash := sha256.New()
		size, err := io.Copy(contentHash, tr)
		if err != nil {
			return nil, "", err
		}
		files = append(files, BackupFile{Path: header.Name, Size: size, MTime: header.ModTime.Unix(), SHA256: hex.EncodeToString(contentHash.Sum(nil))})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	hash := sha256.New()
	for _, file := range files {
		_, _ = fmt.Fprintf(hash, "%s  %s\n", file.SHA256, file.Path)
	}
	return files, hex.EncodeToString(hash.Sum(nil)), nil
}

func legacyArchiveChecksum(archive []byte) (string, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return "", err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	hash := sha256.New()
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if header.Name == "manifest.json" {
			continue
		}
		if _, err := fmt.Fprintf(hash, "%s\x00%d\x00%d\x00%d\x00%q\x00", header.Name, header.Typeflag, header.Size, header.Mode, header.Linkname); err != nil {
			return "", err
		}
		if _, err := io.Copy(hash, tr); err != nil {
			return "", err
		}
	}
	return "sha256:" + hex.EncodeToString(hash.Sum(nil)), nil
}

func filterIncrementalArchive(archive []byte, baseFiles []BackupFile) ([]byte, []string, error) {
	currentFiles, _, err := archiveFileManifest(archive)
	if err != nil {
		return nil, nil, err
	}
	base := make(map[string]BackupFile, len(baseFiles))
	current := make(map[string]BackupFile, len(currentFiles))
	for _, f := range baseFiles {
		base[f.Path] = f
	}
	for _, f := range currentFiles {
		current[f.Path] = f
	}
	changed := make(map[string]bool)
	for _, f := range currentFiles {
		old, ok := base[f.Path]
		if !ok || old.Size != f.Size || old.MTime != f.MTime || old.SHA256 != f.SHA256 {
			changed[f.Path] = true
		}
	}
	deleted := make([]string, 0)
	for _, f := range baseFiles {
		if _, ok := current[f.Path]; !ok {
			deleted = append(deleted, f.Path)
		}
	}
	sort.Strings(deleted)
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = gz.Close() }()
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
	tw := tar.NewWriter(zw)
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if h.Name == "manifest.json" || h.Typeflag != tar.TypeDir && !changed[h.Name] {
			continue
		}
		if err := tw.WriteHeader(h); err != nil {
			return nil, nil, err
		}
		if h.Typeflag != tar.TypeDir {
			if _, err := io.Copy(tw, tr); err != nil {
				return nil, nil, err
			}
		}
	}
	if err := tw.Close(); err != nil {
		return nil, nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, nil, err
	}
	return out.Bytes(), deleted, nil
}

func decodeBackup(path, password string) (*Manifest, []byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("backup: read backup file: %w", err)
	}
	legacy := len(data) >= 4 && [4]byte{data[0], data[1], data[2], data[3]} == containerMagic
	if legacy {
		manifest, payload, err := readContainer(path)
		if err != nil {
			return nil, nil, err
		}
		if manifest.Encrypted {
			if password == "" {
				return nil, nil, fmt.Errorf("backup: encrypted backup requires a password")
			}
			payload, err = DecryptArchive(password, payload)
			if err != nil {
				return nil, nil, err
			}
		}
		return manifest, payload, nil
	}
	encrypted := strings.HasSuffix(strings.ToLower(path), ".enc")
	if encrypted {
		if password == "" {
			return nil, nil, fmt.Errorf("backup: encrypted backup requires a password")
		}
		data, err = DecryptArchive(password, data)
		if err != nil {
			return nil, nil, err
		}
	}
	manifest, err := manifestFromArchive(data)
	if err != nil {
		return nil, nil, err
	}
	if manifest.Encrypted != encrypted {
		return nil, nil, fmt.Errorf("backup: manifest encryption status does not match filename")
	}
	return manifest, data, nil
}

// ReadManifest returns validated manifest metadata for a backup archive.
func ReadManifest(path, password string) (*Manifest, error) {
	manifest, _, err := decodeBackup(path, password)
	return manifest, err
}

// buildTarGz walks ConfigDir/DataDir per AI.md PART 21 "Backup Contents" and
// returns the gzip-compressed tar bytes plus the top-level contents list
// used in the manifest.
func buildTarGz(opts BackupArchiveOptions) ([]byte, []string, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	var contents []string
	addEntry := func(label string) { contents = append(contents, label) }

	addFile := func(srcPath, tarName string) error {
		info, err := os.Stat(srcPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		return addPathToTar(tw, srcPath, tarName, info)
	}

	addDir := func(srcDir, tarPrefix string) error {
		info, err := os.Stat(srcDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			return nil
		}
		return filepath.Walk(srcDir, func(p string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(srcDir, p)
			if err != nil {
				return err
			}
			if tarPrefix == "data" && (rel == filepath.Join("cache", "branding") || strings.HasPrefix(rel, filepath.Join("cache", "branding")+string(filepath.Separator))) {
				if fi.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			name := tarPrefix
			if rel != "." {
				name = filepath.Join(tarPrefix, rel)
			}
			return addPathToTar(tw, p, name, fi)
		})
	}

	// Always included, per "Backup Contents" table.
	if err := addFile(filepath.Join(opts.ConfigDir, "server.yml"), "config/server.yml"); err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(filepath.Join(opts.ConfigDir, "server.yml")); err == nil {
		addEntry("server.yml")
	}

	dbPath := findServerDB(opts.DataDir, opts.ConfigDir)
	if dbPath != "" || opts.DatabaseSnapshot != nil {
		tempRoot := filepath.Join(os.TempDir(), paths.OrgName)
		if err := os.MkdirAll(tempRoot, 0o700); err != nil {
			return nil, nil, fmt.Errorf("create snapshot temp root: %w", err)
		}
		snapshotDir, err := os.MkdirTemp(tempRoot, "airports-")
		if err != nil {
			return nil, nil, fmt.Errorf("create snapshot temp directory: %w", err)
		}
		defer func() { _ = os.RemoveAll(snapshotDir) }()
		snapshotPath := filepath.Join(snapshotDir, "server.db")
		if opts.DatabaseSnapshot != nil {
			if err := opts.DatabaseSnapshot(context.Background(), snapshotPath); err != nil {
				return nil, nil, fmt.Errorf("create database backup snapshot: %w", err)
			}
		} else {
			store, err := db.OpenSQLite(dbPath, 4)
			if err != nil {
				return nil, nil, fmt.Errorf("open database for backup snapshot: %w", err)
			}
			snapshotErr := store.Snapshot(context.Background(), snapshotPath)
			closeErr := store.Close()
			if snapshotErr != nil || closeErr != nil {
				return nil, nil, errors.Join(snapshotErr, closeErr)
			}
		}
		if err := addFile(snapshotPath, "data/db/sqlite/server.db"); err != nil {
			return nil, nil, err
		}
		addEntry("server.db")
	}

	if err := addDir(filepath.Join(opts.ConfigDir, "template"), "config/template"); err != nil {
		return nil, nil, err
	}
	if info, statErr := os.Stat(filepath.Join(opts.ConfigDir, "template")); statErr == nil && info.IsDir() {
		addEntry("template/")
	}

	for _, directory := range []struct {
		name  string
		label string
	}{
		{name: "yaml.d", label: "yaml.d/"},
		{name: "assets", label: "assets/"},
		{name: "security", label: "security/"},
	} {
		path := filepath.Join(opts.ConfigDir, directory.name)
		if err := addDir(path, filepath.Join("config", directory.name)); err != nil {
			return nil, nil, err
		}
		if info, statErr := os.Stat(path); statErr == nil && info.IsDir() {
			addEntry(directory.label)
		}
	}
	if err := addFile(filepath.Join(opts.ConfigDir, "secrets.yml"), "config/secrets.yml"); err != nil {
		return nil, nil, err
	}
	if _, err := os.Stat(filepath.Join(opts.ConfigDir, "secrets.yml")); err == nil {
		addEntry("secrets.yml")
	}

	if err := addDir(filepath.Join(opts.ConfigDir, "theme"), "config/theme"); err != nil {
		return nil, nil, err
	}
	if info, statErr := os.Stat(filepath.Join(opts.ConfigDir, "theme")); statErr == nil && info.IsDir() {
		addEntry("theme/")
	}

	if opts.IncludeSSL {
		if err := addDir(filepath.Join(opts.ConfigDir, "ssl"), "config/ssl"); err != nil {
			return nil, nil, err
		}
		if info, statErr := os.Stat(filepath.Join(opts.ConfigDir, "ssl")); statErr == nil && info.IsDir() {
			addEntry("ssl/")
		}
	}

	if opts.IncludeData {
		if err := addDir(opts.DataDir, "data"); err != nil {
			return nil, nil, err
		}
		if info, statErr := os.Stat(opts.DataDir); statErr == nil && info.IsDir() {
			addEntry("data/")
		}
	}

	if err := tw.Close(); err != nil {
		return nil, nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, nil, err
	}

	if opts.Contents != nil {
		contents = opts.Contents
	}
	return buf.Bytes(), contents, nil
}

// findServerDB locates server.db under the conventional
// {data_dir}/db/sqlite/server.db location, falling back to a couple of
// legacy/simple layouts so backups still work regardless of exact DB path
// resolution used elsewhere in the codebase.
func findServerDB(dataDir, configDir string) string {
	candidates := []string{
		filepath.Join(dataDir, "db", "sqlite", "server.db"),
		filepath.Join(dataDir, "server.db"),
		filepath.Join(configDir, "server.db"),
	}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && !info.IsDir() {
			return c
		}
	}
	return ""
}

func addPathToTar(tw *tar.Writer, srcPath, tarName string, info os.FileInfo) error {
	header, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	header.Name = filepath.ToSlash(tarName)

	if err := tw.WriteHeader(header); err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	f, err := os.Open(srcPath)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	_, err = io.Copy(tw, f)
	return err
}

// Verify runs the 7 checks from AI.md PART 21 "Verification" table. It is a
// pure check — on failure it returns a descriptive error naming the failed
// check but never deletes anything; callers decide what to do with a
// failing backup file.
func Verify(path string, password string) error {
	return VerifyWithLimits(path, password, RestoreLimits{})
}

// VerifyWithLimits verifies the archive and any incremental base using the
// configured restore expansion limits.
func VerifyWithLimits(path string, password string, limits RestoreLimits) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("verify: file exists: %w", err)
	}
	if info.Size() == 0 {
		return fmt.Errorf("verify: size > 0: backup file is empty")
	}
	limits = normalizedRestoreLimits(limits)
	if info.Size() > limits.MaxArchiveSize {
		return fmt.Errorf("verify: archive exceeds server.restore.max_archive_size")
	}
	compressedSize := info.Size()

	manifest, plaintext, err := decodeBackup(path, password)
	if err != nil {
		return fmt.Errorf("verify: manifest readable: %w", err)
	}
	if err := validateExpandedArchive(plaintext, info.Size(), limits); err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	if err := verifyChecksum(manifest, plaintext); err != nil {
		return fmt.Errorf("verify: checksum valid: %w", err)
	}
	if manifest.Type == "incremental" {
		if filepath.Base(manifest.BaseBackup) != manifest.BaseBackup {
			return fmt.Errorf("verify: invalid base backup name")
		}
		basePath := filepath.Join(filepath.Dir(path), manifest.BaseBackup)
		baseInfo, statErr := os.Stat(basePath)
		if statErr != nil {
			return fmt.Errorf("verify: %w", ErrBaseMissing)
		}
		if baseInfo.Size() > limits.MaxArchiveSize-compressedSize {
			return fmt.Errorf("verify: combined archives exceed server.restore.max_archive_size")
		}
		compressedSize += baseInfo.Size()
		baseManifest, basePlain, err := decodeBackup(basePath, password)
		if err != nil {
			return fmt.Errorf("verify: %w: %v", ErrBaseMissing, err)
		}
		if baseManifest.Type != "full" {
			return fmt.Errorf("verify: %w", ErrBaseCorrupt)
		}
		if err := validateExpandedArchive(basePlain, baseInfo.Size(), limits); err != nil {
			return fmt.Errorf("verify: base archive: %w", err)
		}
		if verifyChecksum(baseManifest, basePlain) != nil || baseManifest.Checksum != manifest.BaseSHA256 {
			return fmt.Errorf("verify: %w", ErrBaseCorrupt)
		}
		if err := validateCombinedManifestLimits(baseManifest, manifest, compressedSize, limits); err != nil {
			return fmt.Errorf("verify: combined incremental: %w", err)
		}
		plaintext, err = mergeIncrementalArchives(basePlain, plaintext, manifest.Deleted)
		if err != nil {
			return fmt.Errorf("verify: incremental expansion: %w", err)
		}
	}
	if err := validateExpandedArchive(plaintext, compressedSize, limits); err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	baseDir := filepath.Join(os.TempDir(), paths.OrgName)
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return fmt.Errorf("verify: content extraction: create temp base: %w", err)
	}
	extractDir, err := os.MkdirTemp(baseDir, "airports-verify-*")
	if err != nil {
		return fmt.Errorf("verify: content extraction: create temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(extractDir); err != nil {
			log.Printf("Warning: could not remove backup verification directory: %v", err)
		}
	}()

	dbPath, err := extractTarGz(plaintext, extractDir)
	if err != nil {
		return fmt.Errorf("verify: content extraction: %w", err)
	}

	if dbPath != "" {
		if err := verifySQLiteIntegrity(dbPath); err != nil {
			return fmt.Errorf("verify: database integrity: %w", err)
		}
	}

	return nil
}

// extractTarGz extracts a plaintext tar.gz archive to a single destDir
// (used by Verify's disposable scratch extraction), guarding against
// zip-slip path traversal, and returns the extracted server.db path if one
// was present.
func extractTarGz(archive []byte, destDir string) (dbPath string, err error) {
	return extractTarGzTo(archive, func(name string) (destPath, root string, ok bool) {
		return filepath.Join(destDir, filepath.FromSlash(name)), destDir, true
	})
}

// extractTarGzTo extracts a plaintext tar.gz archive entry-by-entry, using
// mapPath to resolve each archive entry name to both a real destination
// path and the root directory it must stay confined to (restore.go uses
// this to split "config/" and "data/" prefixed entries into two separate
// target roots, each root checked independently). An entry such as
// "config/../../etc/passwd" is rejected before anything is written — the
// same protection the project's previous main.go restoreBackup
// implementation applied. Returns the extracted server.db path if present.
func extractTarGzTo(archive []byte, mapPath func(name string) (destPath, root string, ok bool)) (dbPath string, err error) {
	if err := validateArchiveEntries(archive); err != nil {
		return "", err
	}
	gzReader, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return "", fmt.Errorf("read gzip: %w", err)
	}
	defer func() { _ = gzReader.Close() }()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar: %w", err)
		}
		if err := validateArchiveHeader(header); err != nil {
			return "", err
		}

		destPath, root, ok := mapPath(header.Name)
		if !ok {
			// Entry does not belong to any known root (config/ or data/
			// prefix) — skip it rather than writing it somewhere
			// unexpected.
			continue
		}

		cleanRoot := filepath.Clean(root)
		destPath = filepath.Clean(destPath)
		if destPath != cleanRoot && !strings.HasPrefix(destPath, cleanRoot+string(os.PathSeparator)) {
			return "", fmt.Errorf("refusing to extract entry outside target directory: %q", header.Name)
		}
		if err := rejectSymlinkComponents(cleanRoot, destPath); err != nil {
			return "", fmt.Errorf("refusing to extract through a symlink for %q: %w", header.Name, err)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, 0o755); err != nil {
				return "", err
			}
		default:
			if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
				return "", err
			}
			// Preserve the archived file's permission bits rather than
			// falling back to 0666&umask, so restored secret-bearing files
			// (server.yml, server.db) keep their restrictive perms.
			mode := os.FileMode(header.Mode).Perm() & 0o666
			if mode == 0 {
				mode = 0o600
			}
			outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
			if err != nil {
				return "", err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				_ = outFile.Close()
				return "", err
			}
			if err := outFile.Close(); err != nil {
				return "", err
			}
			if err := os.Chmod(destPath, mode); err != nil {
				return "", err
			}
			if strings.HasSuffix(header.Name, "server.db") {
				dbPath = destPath
			}
		}
	}
	return dbPath, nil
}

func validateArchiveEntries(archive []byte) error {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return fmt.Errorf("read gzip: %w", err)
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if err := validateArchiveHeader(header); err != nil {
			return err
		}
	}
}

func validateArchiveHeader(header *tar.Header) error {
	name := header.Name
	cleanName := strings.TrimSuffix(name, "/")
	if cleanName == "" || filepath.IsAbs(name) || strings.Contains(name, "\\") || len(cleanName) >= 2 && ((cleanName[0] >= 'A' && cleanName[0] <= 'Z') || (cleanName[0] >= 'a' && cleanName[0] <= 'z')) && cleanName[1] == ':' {
		return fmt.Errorf("unsafe archive path %q", name)
	}
	for _, component := range strings.Split(cleanName, "/") {
		if component == ".." || component == "" {
			return fmt.Errorf("unsafe archive path %q", name)
		}
	}
	if header.Mode&0o6000 != 0 {
		return fmt.Errorf("unsafe setuid/setgid mode on %q", name)
	}
	if header.Typeflag != tar.TypeDir && header.Typeflag != tar.TypeReg {
		return fmt.Errorf("unsupported archive entry type for %q", name)
	}
	return nil
}

func rejectSymlinkComponents(root, target string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	target, err = filepath.Abs(target)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, target)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("target is outside extraction root")
	}
	current := root
	paths := []string{current}
	if relative != "." {
		for _, component := range strings.Split(relative, string(os.PathSeparator)) {
			current = filepath.Join(current, component)
			paths = append(paths, current)
		}
	}
	for _, path := range paths {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			break
		}
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("%s is a symlink", path)
		}
	}
	return nil
}

// verifySQLiteIntegrity opens the extracted server.db read-only and runs
// PRAGMA integrity_check, per AI.md PART 21 "Database integrity" check.
// integrity_check scans the entire database file, so it is bounded by the
// "Bulk operations" tier (60s) from AI.md PART 10's query-timeout table,
// not the 5s simple-SELECT tier.
func verifySQLiteIntegrity(dbPath string) error {
	dsn := "file:" + dbPath + "?mode=ro"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = sqlDB.Close() }()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var result string
	query := "PRAGMA integrity_check"
	started := time.Now()
	err = sqlDB.QueryRowContext(ctx, query).Scan(&result)
	rows := int64(0)
	if err == nil {
		rows = 1
	}
	db.LogQuery(query, 0, started, rows, err)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("run integrity_check: timed out: %w", err)
		}
		return fmt.Errorf("run integrity_check: %w", err)
	}
	if !strings.EqualFold(result, "ok") {
		return fmt.Errorf("integrity_check reported: %s", result)
	}
	return nil
}

// RetentionConfig mirrors server.backup.retention per AI.md PART 21
// "Backup Retention".
type RetentionConfig struct {
	MaxBackups   int
	KeepWeekly   int
	KeepMonthly  int
	KeepYearly   int
	MaxTotalSize int64 // bytes; 0 = disabled
}

// backupFile is a parsed backup filename with the date it represents (for
// dated daily/weekly/monthly/yearly/manual files) used for retention
// classification, per AI.md PART 21 "Backup Cleanup Logic". Dates are
// parsed from the filename itself, never file mtime, so retention behaves
// identically regardless of filesystem timestamp behavior.
type backupFile struct {
	name string
	path string
	size int64
	date time.Time // zero value for daily/hourly incrementals (undated)
	kind string    // "yearly", "monthly", "weekly", "daily", "incremental"
}

// ApplyRetention implements AI.md PART 21 "Backup Cleanup Logic": classifies
// every backup file the app creates by date-derived priority (yearly >
// monthly > weekly > daily), keeps up to each bucket's configured count
// (oldest deleted first), always keeps exactly the current daily/hourly
// incremental, and finally enforces MaxTotalSize by deleting the oldest
// remaining files across all types. Returns the names of deleted files.
func ApplyRetention(backupDir string, projectName string, cfg RetentionConfig) ([]string, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("backup: read backup dir: %w", err)
	}

	var files []backupFile
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(name, ".tmp") {
			continue
		}
		if !isBackupName(name, projectName) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		bf := backupFile{name: name, path: filepath.Join(backupDir, name), size: info.Size()}
		if strings.Contains(name, "-daily.tar.gz") {
			bf.kind = "incremental-daily"
		} else if strings.Contains(name, "-hourly.tar.gz") {
			bf.kind = "incremental-hourly"
		} else if d, ok := parseBackupDate(name, projectName); ok {
			bf.date = d
			kind := classifyDate(d)
			// A weekly/monthly/yearly bucket that is disabled (limit <= 0)
			// must never claim a backup away from the always-on daily
			// bucket — otherwise a backup created on a Sunday or the 1st
			// of the month would be discarded immediately by retention
			// even though max_backups alone would have kept it. Per
			// AI.md PART 21, retention tiers are additive on top of the
			// daily bucket, not mutually exclusive substitutes for it.
			switch kind {
			case "weekly":
				if cfg.KeepWeekly <= 0 {
					kind = "daily"
				}
			case "monthly":
				if cfg.KeepMonthly <= 0 {
					kind = "daily"
				}
			case "yearly":
				if cfg.KeepYearly <= 0 {
					kind = "daily"
				}
			}
			bf.kind = kind
		} else {
			// Unclassified but matches the app's naming convention: per
			// spec, always treated as daily.
			bf.kind = "daily"
		}
		files = append(files, bf)
	}

	sort.Slice(files, func(i, j int) bool { return files[i].date.Before(files[j].date) })

	keep := map[string]bool{}
	toDelete := map[string]backupFile{}

	keepBucket := func(kind string, limit int) {
		var bucket []backupFile
		for _, f := range files {
			if f.kind == kind {
				bucket = append(bucket, f)
			}
		}
		// Oldest first already (files is sorted); keep the newest `limit`.
		start := len(bucket) - limit
		if start < 0 {
			start = 0
		}
		for i, f := range bucket {
			if i >= start && limit > 0 {
				keep[f.name] = true
			}
		}
	}

	keepBucket("yearly", cfg.KeepYearly)
	keepBucket("monthly", cfg.KeepMonthly)
	keepBucket("weekly", cfg.KeepWeekly)
	keepBucket("daily", cfg.MaxBackups)

	// Incrementals are always exactly 1 file; if duplicates exist somehow,
	// keep only the newest of each kind.
	keepBucket("incremental-daily", 1)
	keepBucket("incremental-hourly", 1)

	baseByIncremental := make(map[string]string)
	for _, f := range files {
		if !keep[f.name] || f.kind != "incremental-daily" && f.kind != "incremental-hourly" {
			continue
		}
		baseName, err := incrementalBaseName(f.path)
		if err == nil && baseName != "" {
			baseByIncremental[f.name] = baseName
		}
	}
	protectedBases := func() map[string]bool {
		protected := make(map[string]bool)
		for incrementalName, baseName := range baseByIncremental {
			if keep[incrementalName] {
				protected[baseName] = true
			}
		}
		return protected
	}
	for name := range protectedBases() {
		for _, f := range files {
			if f.name == name {
				keep[name] = true
			}
		}
	}

	for _, f := range files {
		if !keep[f.name] {
			toDelete[f.name] = f
		}
	}

	// Enforce max_total_size across whatever remains after count pruning,
	// oldest first, size cap overrides count limits.
	if cfg.MaxTotalSize > 0 {
		var remaining []backupFile
		var total int64
		for _, f := range files {
			if keep[f.name] {
				remaining = append(remaining, f)
				total += f.size
			}
		}
		sort.Slice(remaining, func(i, j int) bool { return remaining[i].date.Before(remaining[j].date) })
		for total > cfg.MaxTotalSize {
			progress := false
			for _, f := range remaining {
				if total <= cfg.MaxTotalSize {
					break
				}
				if !keep[f.name] || protectedBases()[f.name] {
					continue
				}
				delete(keep, f.name)
				toDelete[f.name] = f
				total -= f.size
				progress = true
			}
			if !progress {
				break
			}
		}
	}
	if cfg.MaxTotalSize > 0 {
		var total int64
		for _, f := range files {
			if keep[f.name] {
				total += f.size
			}
		}
		if total > cfg.MaxTotalSize {
			log.Printf("WARN: backup.cap_exceeded_protected_base")
		}
	}

	var deleted []string
	for name, f := range toDelete {
		if err := os.Remove(f.path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return deleted, fmt.Errorf("backup: delete %s: %w", name, err)
		}
		deleted = append(deleted, name)
	}
	sort.Strings(deleted)
	return deleted, nil
}

func incrementalBaseName(path string) (string, error) {
	if strings.HasSuffix(strings.ToLower(path), ".enc") {
		blob, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		header, err := ParseEncryptionHeader(blob)
		if err != nil {
			return "", err
		}
		return header.BaseName, nil
	}
	manifest, err := ReadManifest(path, "")
	if err != nil {
		return "", err
	}
	if manifest.Type != "incremental" {
		return "", nil
	}
	return manifest.BaseBackup, nil
}

// isBackupName reports whether name matches one of the app's own backup
// naming patterns, per AI.md PART 21 "Backup Cleanup Logic".
func isBackupName(name, projectName string) bool {
	prefixDaily := projectName + "_backup_"
	prefixIncremental := projectName + "-"
	if strings.HasPrefix(name, prefixDaily) && (strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tar.gz.enc")) {
		return true
	}
	if strings.HasPrefix(name, prefixIncremental) && (strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tar.gz.enc")) {
		return true
	}
	return false
}

// parseBackupDate extracts the date embedded in a
// "{project}_backup_YYYY-MM-DD.tar.gz[.enc]" or
// "{project}_backup_YYYY-MM-DD_HHMMSS.tar.gz[.enc]" filename.
func parseBackupDate(name, projectName string) (time.Time, bool) {
	prefix := projectName + "_backup_"
	if !strings.HasPrefix(name, prefix) {
		return time.Time{}, false
	}
	rest := strings.TrimPrefix(name, prefix)
	rest = strings.TrimSuffix(rest, ".tar.gz.enc")
	rest = strings.TrimSuffix(rest, ".tar.gz")

	if len(rest) >= 10 {
		datePart := rest[:10]
		if t, err := time.Parse("2006-01-02", datePart); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// classifyDate returns the highest-priority retention bucket a date
// satisfies: yearly (Jan 1) > monthly (1st) > weekly (Sunday) > daily.
func classifyDate(d time.Time) string {
	switch {
	case d.Month() == time.January && d.Day() == 1:
		return "yearly"
	case d.Day() == 1:
		return "monthly"
	case d.Weekday() == time.Sunday:
		return "weekly"
	default:
		return "daily"
	}
}

// ParseMaxTotalSize parses server.backup.retention.max_total_size ("10%",
// "50G", "500M", a plain byte count, or a falsey value meaning disabled)
// into an absolute byte count for backupDir's filesystem. Exported for
// callers (e.g. main.go) that need to convert config's string form into the
// int64 RetentionConfig.MaxTotalSize field before calling ApplyRetention.
func ParseMaxTotalSize(value string, backupDir string) (int64, error) {
	return parseMaxTotalSize(value, backupDir)
}

// parseMaxTotalSize parses server.backup.retention.max_total_size ("10%",
// "50G", "500M", a plain byte count, or a falsey value meaning disabled)
// into an absolute byte count for backupDir's filesystem. Percent values
// resolve against the filesystem/volume containing backupDir.
func parseMaxTotalSize(value string, backupDir string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || isFalseyRetentionValue(trimmed) {
		return 0, nil
	}

	if strings.HasSuffix(trimmed, "%") {
		pctStr := strings.TrimSuffix(trimmed, "%")
		pct, err := strconv.ParseFloat(pctStr, 64)
		if err != nil || pct <= 0 {
			return 0, fmt.Errorf("backup: invalid max_total_size percent %q", value)
		}
		total, _, err := DiskUsage(backupDir)
		if err != nil {
			return 0, fmt.Errorf("backup: resolve disk size for max_total_size: %w", err)
		}
		return int64(float64(total) * pct / 100.0), nil
	}

	return parseByteSize(trimmed)
}

// isFalseyRetentionValue reports the retention-specific falsey set from
// AI.md PART 21 ("0, false, no, none, disable, disabled, off") — a superset
// of config.IsFalsy which does not include "none".
func isFalseyRetentionValue(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "0", "false", "no", "none", "disable", "disabled", "off":
		return true
	default:
		return false
	}
}

// parseByteSize parses an absolute size like "50G", "500M", "1024K", or a
// bare byte count.
func parseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0, fmt.Errorf("backup: empty size")
	}

	multiplier := int64(1)
	numPart := s
	switch {
	case strings.HasSuffix(s, "TB"), strings.HasSuffix(s, "T"):
		multiplier = 1 << 40
		numPart = strings.TrimSuffix(strings.TrimSuffix(s, "TB"), "T")
	case strings.HasSuffix(s, "GB"), strings.HasSuffix(s, "G"):
		multiplier = 1 << 30
		numPart = strings.TrimSuffix(strings.TrimSuffix(s, "GB"), "G")
	case strings.HasSuffix(s, "MB"), strings.HasSuffix(s, "M"):
		multiplier = 1 << 20
		numPart = strings.TrimSuffix(strings.TrimSuffix(s, "MB"), "M")
	case strings.HasSuffix(s, "KB"), strings.HasSuffix(s, "K"):
		multiplier = 1 << 10
		numPart = strings.TrimSuffix(strings.TrimSuffix(s, "KB"), "K")
	case strings.HasSuffix(s, "B"):
		numPart = strings.TrimSuffix(s, "B")
	}

	numPart = strings.TrimSpace(numPart)
	value, err := strconv.ParseFloat(numPart, 64)
	if err != nil {
		return 0, fmt.Errorf("backup: invalid size %q", s)
	}
	return int64(value * float64(multiplier)), nil
}

// preflightDiskSpace implements AI.md PART 21 "Backup Creation Flow" step 2:
// abort (without writing anything) if free space is critically low.
func preflightDiskSpace(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("backup: create backup directory: %w", err)
	}
	total, free, err := DiskUsage(dir)
	if err != nil {
		// Fail open on platforms/paths where disk stats are unavailable —
		// this is a best-effort guard, not a hard dependency.
		return nil
	}
	if total == 0 {
		return nil
	}

	usedPercent := float64(total-free) / float64(total) * 100.0
	if usedPercent > diskThresholdPercent {
		return ErrDiskFull
	}

	// free < 2x size of most recent backup, per AI.md PART 21
	// "Backup Creation Flow" step 2.
	if mostRecent := mostRecentBackupSize(dir); mostRecent > 0 {
		if free < uint64(mostRecent)*2 {
			return ErrDiskFull
		}
	}

	return nil
}

// mostRecentBackupSize returns the size in bytes of the most recently
// modified backup file (any *.tar.gz / *.tar.gz.enc) in dir, or 0 if none
// exist. Used only as an input to the free-space preflight check.
func mostRecentBackupSize(dir string) int64 {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	var newest os.FileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".tar.gz") && !strings.HasSuffix(name, ".tar.gz.enc") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if newest == nil || info.ModTime().After(newest.ModTime()) {
			newest = info
		}
	}
	if newest == nil {
		return 0
	}
	return newest.Size()
}

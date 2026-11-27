package paths

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsurePrivatePathsRepairsOnlyPrivateTrees(t *testing.T) {
	withRootOrAdmin(t, false)
	root := newScratchDir(t)
	configDir := filepath.Join(root, "config")
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	privateDirs := []string{
		filepath.Join(configDir, "yaml.d", "nested"),
		filepath.Join(configDir, "assets"),
		filepath.Join(configDir, "ssl"),
		filepath.Join(configDir, "tor"),
		filepath.Join(configDir, "i2p"),
		filepath.Join(configDir, "security", "key-backups"),
		filepath.Join(dataDir, "security", "geoip"),
		filepath.Join(dataDir, "tor", "site"),
	}
	for _, directory := range privateDirs {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	privateFiles := []string{
		filepath.Join(configDir, "server.yml"),
		filepath.Join(configDir, "secrets.yml"),
		filepath.Join(configDir, "yaml.d", "nested", "maintenance.yml"),
		filepath.Join(configDir, "assets", "branding.svg"),
		filepath.Join(configDir, "ssl", "certificate.pem"),
		filepath.Join(configDir, "tor", "torrc"),
		filepath.Join(configDir, "i2p", "i2pd.conf"),
		filepath.Join(configDir, "security", "pgp.priv.asc.enc"),
		filepath.Join(configDir, "security", "pgp.pub.asc"),
		filepath.Join(configDir, "security", "keyservers.state"),
		filepath.Join(configDir, "security", "key-backups", "pgp.priv.old.asc.enc"),
		filepath.Join(dataDir, "security", "geoip", "country.mmdb"),
		filepath.Join(dataDir, "tor", "site", "hostname"),
	}
	for _, file := range privateFiles {
		if err := os.WriteFile(file, []byte("private"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(file, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	nonPrivateFile := filepath.Join(dataDir, "public-index")
	if err := os.WriteFile(nonPrivateFile, []byte("public"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := EnsurePrivatePaths(configDir, dataDir); err != nil {
		t.Fatalf("EnsurePrivatePaths: %v", err)
	}
	for _, directory := range privateDirs {
		info, err := os.Stat(directory)
		if err != nil {
			t.Errorf("stat private directory %s: %v", directory, err)
		} else if info.Mode().Perm() != 0o700 {
			t.Errorf("private directory %s mode = %04o, want 0700", directory, info.Mode().Perm())
		}
	}
	for _, file := range privateFiles {
		info, err := os.Stat(file)
		if err != nil {
			t.Errorf("stat private file %s: %v", file, err)
		} else if info.Mode().Perm() != 0o600 {
			t.Errorf("private file %s mode = %04o, want 0600", file, info.Mode().Perm())
		}
	}
	publicInfo, err := os.Stat(nonPrivateFile)
	if err != nil {
		t.Errorf("stat non-private file: %v", err)
	} else if publicInfo.Mode().Perm() != 0o644 {
		t.Errorf("non-private file mode = %04o, want unchanged 0644", publicInfo.Mode().Perm())
	}
}

func TestEnsurePrivatePathsRepairsConfigSecurityFilesAndRunsACLHook(t *testing.T) {
	withRootOrAdmin(t, false)
	root := newScratchDir(t)
	configDir := filepath.Join(root, "config")
	dataDir := filepath.Join(root, "data")
	keyDir := filepath.Join(configDir, "security")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(keyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	keyFile := filepath.Join(keyDir, "pgp.priv.asc.enc")
	if err := os.WriteFile(keyFile, []byte("private key"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(keyFile, 0o644); err != nil {
		t.Fatal(err)
	}
	var aclPaths []string
	previousHook := privatePathACLHook
	privatePathACLHook = func(path string) error {
		aclPaths = append(aclPaths, path)
		return nil
	}
	defer func() { privatePathACLHook = previousHook }()

	if err := EnsurePrivatePaths(configDir, dataDir); err != nil {
		t.Fatalf("EnsurePrivatePaths: %v", err)
	}
	info, err := os.Stat(keyFile)
	if err != nil {
		t.Fatalf("stat encrypted PGP key: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("encrypted PGP key mode = %04o, want 0600", info.Mode().Perm())
	}
	for _, path := range aclPaths {
		if path == keyFile {
			return
		}
	}
	t.Fatalf("private-path ACL hook was not applied to nested key file %s; called for %v", keyFile, aclPaths)
}

func TestEnsurePrivatePathsRejectsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on some Windows installations")
	}
	root := newScratchDir(t)
	configDir := filepath.Join(root, "config")
	dataDir := filepath.Join(root, "data")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside")
	if err := os.Mkdir(outside, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(configDir, "assets")); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePrivatePaths(configDir, dataDir); err == nil {
		t.Fatal("EnsurePrivatePaths accepted a private-path symlink")
	}
}

func TestValidatePrivateRootsRejectsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires privileges on some Windows installations")
	}
	root := newScratchDir(t)
	actual := filepath.Join(root, "actual")
	if err := os.Mkdir(actual, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "config-link")
	if err := os.Symlink(actual, link); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePrivateRoots(link, actual); err == nil {
		t.Fatal("ValidatePrivateRoots accepted a symlink root")
	}
}

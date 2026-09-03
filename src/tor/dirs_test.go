package tor

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestEnsureDirCreatesWithRestrictedPerms(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "config", "tor")

	if err := ensureDir(target); err != nil {
		t.Fatalf("ensureDir() error = %v", err)
	}

	info, err := os.Stat(target)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if !info.IsDir() {
		t.Fatal("target is not a directory")
	}

	if runtime.GOOS != "windows" {
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("dir perm = %o, want 0700", perm)
		}
	}
}

func TestEnsureDirIdempotentOnExisting(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "tor")

	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("setup MkdirAll() error = %v", err)
	}

	if err := ensureDir(target); err != nil {
		t.Fatalf("ensureDir() error = %v", err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(target)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o700 {
			t.Errorf("dir perm = %o, want 0700 enforced on pre-existing dir", perm)
		}
	}
}

func TestEnsureFileCreatesOnceWithRestrictedPerms(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "tor", "torrc")

	created, err := ensureFile(path, []byte("# first write\n"))
	if err != nil {
		t.Fatalf("ensureFile() error = %v", err)
	}
	if !created {
		t.Error("created = false on first call, want true")
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("file perm = %o, want 0600", perm)
		}
	}

	// Second call must NOT overwrite content (torrc is persistent) but
	// must still report created = false and keep permissions enforced.
	created, err = ensureFile(path, []byte("# second write should be ignored\n"))
	if err != nil {
		t.Fatalf("ensureFile() second call error = %v", err)
	}
	if created {
		t.Error("created = true on second call, want false (file already existed)")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "# first write\n" {
		t.Errorf("file content = %q, want original content preserved", string(data))
	}
}

func TestWriteFileOverwritesExisting(t *testing.T) {
	base := t.TempDir()
	path := filepath.Join(base, "tor", "torrc")

	if _, err := ensureFile(path, []byte("original\n")); err != nil {
		t.Fatalf("ensureFile() error = %v", err)
	}

	if err := writeFile(path, []byte("updated\n")); err != nil {
		t.Fatalf("writeFile() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "updated\n" {
		t.Errorf("file content = %q, want %q", string(data), "updated\n")
	}
}

func TestResolveDirsShape(t *testing.T) {
	dirs := resolveDirs("airports-test")

	if filepath.Base(dirs.TorrcPath) != "torrc" {
		t.Errorf("TorrcPath = %s, want basename torrc", dirs.TorrcPath)
	}
	if filepath.Dir(dirs.TorrcPath) != dirs.ConfigDir {
		t.Errorf("TorrcPath parent = %s, want ConfigDir %s", filepath.Dir(dirs.TorrcPath), dirs.ConfigDir)
	}
	if filepath.Dir(dirs.SiteDir) != dirs.DataDir {
		t.Errorf("SiteDir parent = %s, want DataDir %s", filepath.Dir(dirs.SiteDir), dirs.DataDir)
	}
	if filepath.Dir(dirs.KeyPath) != dirs.SiteDir {
		t.Errorf("KeyPath parent = %s, want SiteDir %s", filepath.Dir(dirs.KeyPath), dirs.SiteDir)
	}
	if filepath.Base(dirs.LogFile) != "tor.log" {
		t.Errorf("LogFile = %s, want basename tor.log", dirs.LogFile)
	}
}

package paths

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

var privatePathACLHook = enforcePrivatePathACL

// EnsurePrivatePaths repairs permissions and Windows ACLs for every private
// configuration and security path before configuration is read or children start.
func EnsurePrivatePaths(configDir, dataDir string) error {
	if err := ValidatePrivateRoots(configDir, dataDir); err != nil {
		return err
	}
	for _, directory := range []string{
		filepath.Join(configDir, "yaml.d"),
		filepath.Join(configDir, "assets"),
		filepath.Join(configDir, "ssl"),
		filepath.Join(configDir, "tor"),
		filepath.Join(configDir, "i2p"),
		filepath.Join(configDir, "security"),
		filepath.Join(dataDir, "security"),
		filepath.Join(dataDir, "tor"),
	} {
		if err := ensurePrivateDirectory(directory); err != nil {
			return err
		}
	}
	for _, file := range []string{
		filepath.Join(configDir, "server.yml"),
		filepath.Join(configDir, "secrets.yml"),
	} {
		if err := enforcePrivateFile(file); err != nil {
			return err
		}
	}
	return nil
}

// ValidatePrivateRoots rejects configured roots that are symbolic links before
// the general directory setup can create or chmod through them.
func ValidatePrivateRoots(configDir, dataDir string) error {
	for _, root := range []string{configDir, dataDir} {
		info, err := os.Lstat(root)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return fmt.Errorf("inspect private root %s: %w", root, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("private root %s is a symbolic link", root)
		}
		if !info.IsDir() {
			return fmt.Errorf("private root %s is not a directory", root)
		}
	}
	return nil
}

func ensurePrivateDirectory(root string) error {
	info, err := os.Lstat(root)
	if errors.Is(err, fs.ErrNotExist) {
		if err := os.Mkdir(root, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
			return fmt.Errorf("create private directory %s: %w", root, err)
		}
		info, err = os.Lstat(root)
	}
	if err != nil {
		return fmt.Errorf("inspect private path %s: %w", root, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("private path %s is a symbolic link", root)
	}
	if !info.IsDir() {
		return fmt.Errorf("private path %s is not a directory", root)
	}
	if err := privatePathACLHook(root); err != nil {
		return fmt.Errorf("enforce private permissions on %s: %w", root, err)
	}
	if err := os.Chmod(root, 0o700); err != nil {
		return fmt.Errorf("set private directory mode on %s: %w", root, err)
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("read private directory %s: %w", root, err)
	}
	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("inspect private path %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("private path %s is a symbolic link", path)
		}
		if info.IsDir() {
			if err := ensurePrivateDirectory(path); err != nil {
				return err
			}
			continue
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("private path %s is not a regular file", path)
		}
		if err := privatePathACLHook(path); err != nil {
			return fmt.Errorf("enforce private permissions on %s: %w", path, err)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("set private file mode on %s: %w", path, err)
		}
	}
	return nil
}

func enforcePrivateFile(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("inspect private path %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("private path %s is a symbolic link", path)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("private path %s is not a regular file", path)
	}
	if err := privatePathACLHook(path); err != nil {
		return fmt.Errorf("enforce private permissions on %s: %w", path, err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("set private file mode on %s: %w", path, err)
	}
	return nil
}

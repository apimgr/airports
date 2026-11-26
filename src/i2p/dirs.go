package i2p

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// I2PDirs holds the application-owned I2P filesystem locations.
type I2PDirs struct {
	ConfigDir string
	DataDir   string
	LogDir    string
}

func (d I2PDirs) i2pConfigDir() string { return filepath.Join(d.ConfigDir, "i2p") }
func (d I2PDirs) i2pDataDir() string   { return filepath.Join(d.DataDir, "i2p") }
func (d I2PDirs) siteDir() string      { return filepath.Join(d.i2pDataDir(), "site") }
func (d I2PDirs) keysPath() string     { return filepath.Join(d.siteDir(), "site-keys.dat") }

func ensureDirs(d I2PDirs) error {
	for _, dir := range []string{d.i2pConfigDir(), d.i2pDataDir(), d.siteDir()} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("create i2p directory %s: %w", dir, err)
		}
		if runtime.GOOS != "windows" {
			if err := os.Chmod(dir, 0o700); err != nil {
				return fmt.Errorf("chmod i2p directory %s: %w", dir, err)
			}
			if err := os.Chown(dir, os.Getuid(), os.Getgid()); err != nil {
				return fmt.Errorf("chown i2p directory %s: %w", dir, err)
			}
		}
	}
	return nil
}

func writePrivateFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create parent directory for %s: %w", path, err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0o600); err != nil {
			return fmt.Errorf("chmod %s: %w", path, err)
		}
		if err := os.Chown(path, os.Getuid(), os.Getgid()); err != nil {
			return fmt.Errorf("chown %s: %w", path, err)
		}
	}
	return nil
}

//go:build !windows

package app

import "os"

func replaceBinary(currentPath, newBinaryPath string) error {
	return os.Rename(newBinaryPath, currentPath)
}

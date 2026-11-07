//go:build !windows

package main

import "os"

func replaceBinary(currentPath, newBinaryPath string) error {
	return os.Rename(newBinaryPath, currentPath)
}

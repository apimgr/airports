//go:build !windows

package main

import (
	"os"
	"syscall"
)

func replaceCLI(currentPath, newBinaryPath string) error {
	return os.Rename(newBinaryPath, currentPath)
}

func reexecCLI(binaryPath string, args, env []string) error {
	return syscall.Exec(binaryPath, args, env)
}

//go:build !windows

package main

import (
	"os"
	"syscall"
)

func execUpdatedBinary(path string) error {
	return syscall.Exec(path, os.Args, os.Environ())
}

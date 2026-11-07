//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

func replaceBinary(currentPath, newBinaryPath string) error {
	oldPath := currentPath + ".old"
	if err := os.Remove(oldPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove previous update backup: %w", err)
	}
	if err := os.Rename(currentPath, oldPath); err != nil {
		return fmt.Errorf("rename running binary to backup: %w", err)
	}
	if err := os.Rename(newBinaryPath, currentPath); err != nil {
		if restoreErr := os.Rename(oldPath, currentPath); restoreErr != nil {
			return fmt.Errorf("install updated binary: %w; restore previous binary: %v", err, restoreErr)
		}
		return fmt.Errorf("install updated binary: %w", err)
	}
	oldPathPointer, err := windows.UTF16PtrFromString(oldPath)
	if err != nil {
		return fmt.Errorf("prepare update backup cleanup: %w", err)
	}
	if err := windows.MoveFileEx(oldPathPointer, nil, windows.MOVEFILE_DELAY_UNTIL_REBOOT); err != nil {
		return fmt.Errorf("schedule previous binary cleanup: %w", err)
	}
	return nil
}

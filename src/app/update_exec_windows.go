//go:build windows

package app

import "errors"

func execUpdatedBinary(string) error {
	return errors.New("automatic staged updates are disabled on Windows")
}

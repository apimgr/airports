//go:build !windows

package paths

func enforcePrivateACL(string) error { return nil }

func enforcePrivatePathACL(string) error { return nil }

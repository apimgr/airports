package tor

import (
	"os"
	"os/exec"
	"runtime"
)

// commonTorPaths lists well-known per-OS install locations checked after
// PATH, per AI.md PART 31 "Tor Process Management" step 1.
func commonTorPaths() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{`C:\Program Files\Tor\tor.exe`, `C:\Program Files (x86)\Tor\tor.exe`}
	case "darwin":
		return []string{"/usr/local/bin/tor", "/opt/homebrew/bin/tor"}
	default:
		return []string{"/usr/bin/tor", "/usr/local/bin/tor"}
	}
}

// findTorBinary resolves the tor executable to use, in priority order:
// an explicitly configured path, PATH lookup, then common per-OS install
// locations. Returns ErrBinaryNotFound if none are usable.
func findTorBinary(configured string) (string, error) {
	if configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured, nil
		}
		return "", ErrBinaryNotFound
	}

	if path, err := exec.LookPath("tor"); err == nil {
		return path, nil
	}

	for _, candidate := range commonTorPaths() {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}

	return "", ErrBinaryNotFound
}

//go:build windows

package paths

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// enforcePrivateACL protects system-mode application and backup paths under
// ProgramData from the inherited BUILTIN\Users read access.
func enforcePrivateACL(path string) error {
	programData := strings.TrimSpace(os.Getenv("ProgramData"))
	if programData == "" {
		programData = `C:\ProgramData`
	}
	fullPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve private path: %w", err)
	}
	for _, privateRoot := range []string{
		filepath.Join(programData, OrgName, defaultProjectName),
		filepath.Join(programData, "Backups", OrgName, defaultProjectName),
	} {
		root, err := filepath.Abs(privateRoot)
		if err != nil {
			return fmt.Errorf("resolve private path root: %w", err)
		}
		relative, err := filepath.Rel(root, fullPath)
		if err != nil {
			return fmt.Errorf("compare private path: %w", err)
		}
		if relative != "." && (relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))) {
			continue
		}
		return applyPrivateACL(fullPath, true)
	}
	return nil
}

// enforcePrivatePathACL protects a private path, including configured path
// overrides, with the system-mode ACL required by AI.md PART 8.
func enforcePrivatePathACL(path string) error {
	if !startedElevated {
		return nil
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	return applyPrivateACL(path, info.IsDir())
}

func applyPrivateACL(path string, directory bool) error {
	service, system, admins := `NT SERVICE\`+defaultProjectName+`:F`, `NT AUTHORITY\SYSTEM:F`, `BUILTIN\Administrators:F`
	if directory {
		service = `NT SERVICE\` + defaultProjectName + `:(OI)(CI)F`
		system = `NT AUTHORITY\SYSTEM:(OI)(CI)F`
		admins = `BUILTIN\Administrators:(OI)(CI)F`
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	reset := exec.CommandContext(ctx, "icacls", path, "/reset")
	if output, err := reset.CombinedOutput(); err != nil {
		return fmt.Errorf("reset ACL: %w: %s", err, strings.TrimSpace(string(output)))
	}
	command := exec.CommandContext(ctx, "icacls", path, "/inheritance:r", "/grant:r", service, system, admins)
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("set ACL: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

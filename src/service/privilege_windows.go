//go:build windows

package service

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// IsElevated returns true when the process token is a member of the built-in
// Administrators alias, per AI.md PART 5 "Privileged Port Binding" §
// Binary Implementation item 1 (Windows).
func IsElevated() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer func() { _ = windows.FreeSid(sid) }()

	token := windows.Token(0)
	isMember, err := token.IsMember(sid)
	if err != nil {
		return false
	}
	return isMember
}

// isInAdminGroup checks if the process token's group list includes the
// Administrators alias, per AI.md PART 5 § Binary Implementation item 1.
func isInAdminGroup() bool {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid,
	)
	if err != nil {
		return false
	}
	defer func() { _ = windows.FreeSid(sid) }()

	token := windows.Token(0)
	groups, err := token.GetTokenGroups()
	if err != nil {
		return false
	}
	for _, g := range groups.AllGroups() {
		if windows.EqualSid(g.Sid, sid) {
			return true
		}
	}
	return false
}

// CanEscalate reports whether the calling user could plausibly obtain
// elevation via UAC, per AI.md PART 5 § Binary Implementation item 1.
func CanEscalate() bool {
	if IsElevated() {
		return true
	}
	return isInAdminGroup()
}

// ExecElevated re-launches the current binary with the given args under
// UAC elevation via the "runas" verb, per AI.md PART 5 § Binary
// Implementation item 1 (Windows).
func ExecElevated(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no arguments to elevate")
	}
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}
	cwd, _ := os.Getwd()

	verbPtr, _ := syscall.UTF16PtrFromString("runas")
	exePtr, _ := syscall.UTF16PtrFromString(exe)
	argStr := ""
	for i, a := range args[1:] {
		if i > 0 {
			argStr += " "
		}
		argStr += a
	}
	argsPtr, _ := syscall.UTF16PtrFromString(argStr)
	cwdPtr, _ := syscall.UTF16PtrFromString(cwd)

	return windows.ShellExecute(0, verbPtr, exePtr, argsPtr, cwdPtr, windows.SW_NORMAL)
}

// HandleEscalation implements the "smart escalation flow" from AI.md PART 5
// § Binary Implementation item 3 for Windows: no-op if already elevated; a
// hard, non-prompting error if the user cannot escalate at all; otherwise an
// interactive y/N prompt followed by a UAC re-launch.
func HandleEscalation(action string) error {
	if IsElevated() {
		return nil
	}

	if !CanEscalate() {
		return fmt.Errorf("%s requires administrator privileges\n\n"+
			"you do not have administrator access, contact your system administrator", action)
	}

	fmt.Printf("%s requires administrator privileges.\n", action)
	fmt.Print("Re-run elevated now? [Y/n] ")

	var response string
	_, _ = fmt.Scanln(&response)
	switch response {
	case "", "y", "Y", "yes", "Yes":
		// ShellExecute("runas") launches an independent elevated process; this
		// process must not fall through and repeat the action itself.
		if err := ExecElevated(os.Args); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
		return nil
	default:
		return fmt.Errorf("escalation declined")
	}
}

// daemonizedEnvVar marks the re-launched child process so it does not
// attempt to daemonize itself again (Daemonize is a no-op once this is set).
const daemonizedEnvVar = "AIRPORTS_DAEMONIZED"

// Daemonize detaches the current process from its console, per AI.md "Server
// Binary Commands" `--daemon`. Windows has no fork()/setsid() equivalent, so
// this re-launches the current binary with the same argv as a fully detached
// process (DETACHED_PROCESS: no console inherited, CREATE_NEW_PROCESS_GROUP:
// immune to the parent console's Ctrl+C/Ctrl+Break), with stdio redirected to
// NUL, prints the child's PID, and exits the parent. Returns nil without side
// effects when already running as the detached child.
func Daemonize() error {
	if os.Getenv(daemonizedEnvVar) == "1" {
		return nil
	}

	nul, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", os.DevNull, err)
	}
	defer nul.Close()

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	cmd := exec.Command(execPath, os.Args[1:]...)
	cmd.Env = append(os.Environ(), daemonizedEnvVar+"=1")
	cmd.Stdin = nul
	cmd.Stdout = nul
	cmd.Stderr = nul
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon child: %w", err)
	}

	fmt.Printf("Daemonized as PID %d\n", cmd.Process.Pid)
	os.Exit(0)
	return nil
}

// ServiceUserIDs is a no-op on Windows — the service runs under a Virtual
// Service Account (NT SERVICE\airports), which has no numeric UID/GID, per
// AI.md PART 23 "Windows Service Account". Callers on Windows never drop
// privileges via UID/GID switching.
func ServiceUserIDs() (uid int, gid int, err error) {
	return 0, 0, fmt.Errorf("ServiceUserIDs is not applicable on Windows (Virtual Service Account)")
}

// DropPrivileges is a no-op on Windows. Privilege containment is handled by
// the Virtual Service Account (NT SERVICE\airports) at service-install time,
// not by a runtime UID/GID switch, per AI.md PART 23 "Windows Service Account".
func DropPrivileges(uid, gid int) error {
	return nil
}

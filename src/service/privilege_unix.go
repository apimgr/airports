//go:build !windows

package service

import (
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

// IsElevated returns true when the process is running as root (Unix), per
// AI.md PART 5 "Privileged Port Binding" § Binary Implementation item 1.
func IsElevated() bool {
	return os.Geteuid() == 0
}

// escalationMethod identifies a privilege-escalation mechanism, per AI.md
// PART 23 "Escalation Detection by OS".
type escalationMethod string

const (
	methodNone      escalationMethod = ""
	methodSudo      escalationMethod = "sudo"
	methodSu        escalationMethod = "su"
	methodPkexec    escalationMethod = "pkexec"
	methodDoas      escalationMethod = "doas"
	methodOsascript escalationMethod = "osascript"
)

// escalationOrder returns the ordered list of escalation methods to probe
// for the given GOOS, per AI.md PART 23 "Escalation Detection by OS".
// "Already root" is handled separately by IsElevated and is not listed here.
func escalationOrder(goos string) []escalationMethod {
	switch goos {
	case "darwin":
		return []escalationMethod{methodSudo, methodOsascript}
	case "freebsd", "openbsd", "netbsd", "dragonfly":
		return []escalationMethod{methodDoas, methodSudo, methodSu}
	default:
		// Linux and other Unix-likes.
		return []escalationMethod{methodSudo, methodSu, methodPkexec, methodDoas}
	}
}

// canEscalateViaSudo reports whether the calling user could plausibly obtain
// root via sudo, either passwordlessly or via group membership
// (sudo/wheel/admin), per AI.md PART 23. sudo gets its own stricter check
// (rather than a bare LookPath) because, unlike su/pkexec/doas/osascript, an
// interactive attempt with no sudo access at all hangs waiting for a
// password the user cannot supply instead of failing fast.
func canEscalateViaSudo(lookPath func(string) (string, error)) bool {
	if _, err := lookPath("sudo"); err != nil {
		return false
	}

	// Passwordless sudo check — never prompts, never blocks.
	cmd := exec.Command("sudo", "-n", "true")
	if cmd.Run() == nil {
		return true
	}

	u, err := user.Current()
	if err != nil {
		return false
	}
	groups, err := u.GroupIds()
	if err != nil {
		return false
	}
	for _, gid := range groups {
		group, lookupErr := user.LookupGroupId(gid)
		if lookupErr != nil || group == nil {
			continue
		}
		switch group.Name {
		case "sudo", "wheel", "admin":
			return true
		}
	}
	return false
}

// detectEscalationMethod returns the first available escalation method for
// the given GOOS, per AI.md PART 23's OS-specific priority order. su,
// pkexec, doas, and osascript are considered available whenever their
// binary is present in PATH — only actually running the command can confirm
// the operator knows the target credential or is authorized by the
// OS-level policy prompt, so presence in PATH is the best static signal.
func detectEscalationMethod(goos string, lookPath func(string) (string, error)) escalationMethod {
	for _, m := range escalationOrder(goos) {
		if m == methodSudo {
			if canEscalateViaSudo(lookPath) {
				return methodSudo
			}
			continue
		}
		if _, err := lookPath(string(m)); err == nil {
			return m
		}
	}
	return methodNone
}

// CanEscalate reports whether the calling user could plausibly obtain root
// via one of the OS-appropriate escalation methods in AI.md PART 23
// "Escalation Detection by OS" (Linux: sudo → su → pkexec → doas; macOS:
// sudo → osascript; BSD: doas → sudo → su). Used to decide whether to
// prompt for escalation at all — a user who cannot escalate is never
// nagged.
func CanEscalate() bool {
	if IsElevated() {
		return true
	}
	return detectEscalationMethod(runtime.GOOS, exec.LookPath) != methodNone
}

// shellQuote wraps s in single quotes for safe inclusion in a POSIX shell
// command string (used to build the `su -c "..."` argument), escaping any
// embedded single quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// appleScriptQuote wraps s in double quotes for safe inclusion in an
// AppleScript string literal (used to build the `osascript -e "do shell
// script \"...\" with administrator privileges"` argument), escaping
// backslashes and embedded double quotes.
func appleScriptQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return `"` + s + `"`
}

// buildElevationCommand constructs the *exec.Cmd that re-executes args under
// the given escalation method, per AI.md PART 23 "Escalation Detection by
// OS". sudo/pkexec/doas take the target command as trailing argv directly;
// su requires a single shell command string via `-c`; osascript wraps that
// same shell command string in a `do shell script ... with administrator
// privileges` AppleScript for a GUI authorization prompt.
func buildElevationCommand(method escalationMethod, args []string) (*exec.Cmd, error) {
	switch method {
	case methodSudo, methodPkexec, methodDoas:
		full := append([]string{string(method)}, args...)
		return exec.Command(full[0], full[1:]...), nil
	case methodSu:
		quoted := make([]string, len(args))
		for i, a := range args {
			quoted[i] = shellQuote(a)
		}
		return exec.Command("su", "-c", strings.Join(quoted, " ")), nil
	case methodOsascript:
		quoted := make([]string, len(args))
		for i, a := range args {
			quoted[i] = shellQuote(a)
		}
		script := fmt.Sprintf("do shell script %s with administrator privileges",
			appleScriptQuote(strings.Join(quoted, " ")))
		return exec.Command("osascript", "-e", script), nil
	default:
		return nil, fmt.Errorf("no privilege escalation method available")
	}
}

// ExecElevated re-executes the current binary with the given args under the
// first available OS-appropriate escalation method (AI.md PART 23),
// inheriting stdio.
func ExecElevated(args []string) error {
	method := detectEscalationMethod(runtime.GOOS, exec.LookPath)
	cmd, err := buildElevationCommand(method, args)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// HandleEscalation implements the "smart escalation flow" from AI.md PART 5
// § Binary Implementation item 3 and PART 23 "Escalation Detection by OS":
// no-op if already elevated; a hard, non-prompting error if the user cannot
// escalate via any OS-appropriate method at all (never nag someone who has
// no path to root); otherwise an interactive y/N prompt followed by a
// re-exec under the first available method (sudo/su/pkexec/doas/osascript).
func HandleEscalation(action string) error {
	if IsElevated() {
		return nil
	}

	method := detectEscalationMethod(runtime.GOOS, exec.LookPath)
	if method == methodNone {
		return fmt.Errorf("%s requires administrator privileges\n\n"+
			"you do not have sudo/admin access, contact your system administrator", action)
	}

	fmt.Printf("%s requires administrator privileges.\n", action)
	fmt.Printf("Re-run with %s now? [Y/n] ", method)

	var response string
	_, _ = fmt.Scanln(&response)
	switch response {
	case "", "y", "Y", "yes", "Yes":
		// The elevated child process performs the actual action (inheriting
		// stdio), so this process must not fall through and repeat it —
		// exit with the child's outcome instead of returning to the caller.
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

// daemonizedEnvVar marks the re-exec'd child process so it does not attempt
// to daemonize itself again (Daemonize is a no-op once this is set).
const daemonizedEnvVar = "AIRPORTS_DAEMONIZED"

// Daemonize detaches the current process from its controlling terminal, per
// AI.md "Server Binary Commands" `--daemon`. It re-executes the current
// binary with the same argv in a new session (setsid) with stdio redirected
// to /dev/null, prints the child's PID, and exits the parent — the standard
// Unix double-fork-free daemonizing idiom achievable from Go (Go's runtime
// cannot safely fork() without exec, so a re-exec + Setsid takes its place).
// Returns nil without side effects when already running as the detached
// child (i.e. re-entrant calls after the re-exec).
func Daemonize() error {
	if os.Getenv(daemonizedEnvVar) == "1" {
		return nil
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", os.DevNull, err)
	}
	defer devNull.Close()

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to resolve executable path: %w", err)
	}

	cmd := exec.Command(execPath, os.Args[1:]...)
	cmd.Env = append(os.Environ(), daemonizedEnvVar+"=1")
	cmd.Stdin = devNull
	cmd.Stdout = devNull
	cmd.Stderr = devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon child: %w", err)
	}

	fmt.Printf("Daemonized as PID %d\n", cmd.Process.Pid)
	os.Exit(0)
	return nil
}

// ServiceUserIDs resolves the numeric UID/GID of the dedicated "airports"
// system account created by EnsureSystemUser, so callers can chown runtime
// directories to it and then drop privileges. Returns an error if the
// account does not exist yet (EnsureSystemUser must run first).
func ServiceUserIDs() (uid int, gid int, err error) {
	u, err := user.Lookup(appName)
	if err != nil {
		return 0, 0, fmt.Errorf("service user %q not found: %w", appName, err)
	}
	uid, err = strconv.Atoi(u.Uid)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid uid %q for service user: %w", u.Uid, err)
	}
	gid, err = strconv.Atoi(u.Gid)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid gid %q for service user: %w", u.Gid, err)
	}
	return uid, gid, nil
}

// DropPrivileges permanently switches the running process from root to the
// given unprivileged UID/GID, per AI.md PART 5 "Privileged Port Binding"
// step 5 ("root→user: DROP PRIVILEGES"). Supplementary groups are cleared
// first, then GID, then UID — UID must be dropped last since once it is
// dropped the process no longer has permission to change GID.
func DropPrivileges(uid, gid int) error {
	if err := syscall.Setgroups([]int{gid}); err != nil {
		return fmt.Errorf("failed to clear supplementary groups: %w", err)
	}
	if err := syscall.Setgid(gid); err != nil {
		return fmt.Errorf("failed to setgid(%d): %w", gid, err)
	}
	if err := syscall.Setuid(uid); err != nil {
		return fmt.Errorf("failed to setuid(%d): %w", uid, err)
	}
	return nil
}

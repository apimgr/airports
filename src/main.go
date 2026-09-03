package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata" // embed the IANA timezone database for TZ / scheduler.timezone (alpine ships no zoneinfo)

	"github.com/go-chi/chi/v5"
	"golang.org/x/term"

	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/backup"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/geoip"
	"github.com/apimgr/airports/src/logging"
	"github.com/apimgr/airports/src/mode"
	"github.com/apimgr/airports/src/notify"
	"github.com/apimgr/airports/src/path"
	"github.com/apimgr/airports/src/scheduler"
	"github.com/apimgr/airports/src/security/cve"
	"github.com/apimgr/airports/src/server"
	"github.com/apimgr/airports/src/service"
	"github.com/apimgr/airports/src/shellcompl"
	"github.com/apimgr/airports/src/ssl"
	"github.com/apimgr/airports/src/tor"
)

//go:embed data/airports.json
var airportsData []byte

var (
	// Injected at build time via ldflags
	Version  = "dev"
	CommitID = "unknown"
	// BuildDate is derived from BuildEpoch in init(); "unknown" when BuildEpoch is unset
	BuildDate = "unknown"
	// BuildEpoch is the Unix build timestamp (seconds, UTC) set via -ldflags; "0" when unset
	BuildEpoch   = "0"
	OfficialSite = ""

	// Project info
	ProjectName = "airports"
	ProjectOrg  = "apimgr"

	// serverShellCommands and serverShellFlags feed shellcompl's generators
	// (PART 32 "Shell Completions") so completion scripts stay in sync with
	// the flags/subcommands actually implemented below.
	serverShellCommands = []string{"update", "service", "maintenance", "backup", "restore", "shell", "email", "tor"}
	serverShellFlags    = []string{
		"--help", "-h", "--version", "-v", "--status", "--mode", "--port", "--address",
		"--config", "--data", "--cache", "--log", "--pid", "--baseurl", "--daemon",
		"--debug", "--color", "--lang", "--backup", "--restore",
		"--service", "--maintenance", "--update", "--shell",
	}

	// backupDirOverride holds the value of --backup {backup_dir} (PART 8/21:
	// --backup SETS the backup directory, it does not run a backup). Consulted
	// by resolveBackupDir() alongside BACKUP_DIR and the OS-standard default.
	backupDirOverride = ""
)

// buildEpoch parses the embedded BuildEpoch ldflag; 0 when unset or invalid.
// Passed to the updater's release-freshness check so a rolling channel tag
// can be compared against this binary's own build time.
func buildEpoch() int64 {
	n, err := strconv.ParseInt(BuildEpoch, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// init derives BuildDate (RFC 3339 UTC) from the embedded BuildEpoch.
func init() {
	if n := buildEpoch(); n > 0 {
		BuildDate = time.Unix(n, 0).UTC().Format("2006-01-02T15:04:05Z")
	}
}

func main() {
	// Command line flags
	portFlag := flag.String("port", "", "HTTP port")
	addressFlag := flag.String("address", "", "Listen address")
	configDirFlag := flag.String("config", "", "Configuration directory")
	dataDirFlag := flag.String("data", "", "Data directory")
	modeFlag := flag.String("mode", "", "Application mode: production, development")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.BoolVar(showVersion, "v", false, "Show version and exit")
	showStatus := flag.Bool("status", false, "Show server status and exit")
	showHelp := flag.Bool("help", false, "Show help message")
	flag.BoolVar(showHelp, "h", false, "Show help message")
	debugFlag := flag.Bool("debug", false, "Enable debug mode")
	colorFlag := flag.String("color", "auto", "Color output: auto, yes, no")
	langFlag := flag.String("lang", "", "Language code (e.g. en, es, zh, fr, ar, de, ja)")
	backupDirFlag := flag.String("backup", "", "Backup directory")
	restorePath := flag.String("restore", "", "Restore from backup archive at <path>")
	cacheDirFlag := flag.String("cache", "", "Cache directory")
	logDirFlag := flag.String("log", "", "Log directory")
	pidFileFlag := flag.String("pid", "", "PID file path")
	baseURLFlag := flag.String("baseurl", "", "URL path prefix (default: /)")
	daemonFlag := flag.Bool("daemon", false, "Daemonize (detach from terminal)")

	// Service commands
	serviceCmd := flag.String("service", "", "Service management: install|uninstall|start|stop|restart|status|enable|disable|logs")

	// --maintenance starts the server in maintenance (read-only) mode per spec.
	maintenanceMode := flag.Bool("maintenance", false, "Start in maintenance mode (read-only; serves maintenance page)")

	// --update [check|yes|branch {stable|beta|daily}] is a value/subcommand flag,
	// not a plain boolean (PART 22). The stdlib flag package has no notion of an
	// optional value, so extract it from argv before flag.Parse() runs.
	updateFound, updateAction, updateBranchArg, afterUpdateArgs := extractUpdateFlag(os.Args[1:])

	// --shell {completions|init|help} [SHELL] is likewise a value/subcommand
	// flag (PART 32 "Shell Completions") extracted before flag.Parse() runs.
	shellFound, shellSubcmd, shellName, afterShellArgs := shellcompl.Extract(afterUpdateArgs)

	// `scheduler list|show <id>|run <id>|enable <id>|disable <id>|history <id>`
	// is a positional subcommand (PART 18 "CLI Commands"), not a --flag, so it
	// is extracted the same way before flag.Parse() runs.
	schedulerFound, schedulerAction, schedulerTaskID, afterSchedulerArgs := extractSchedulerCommand(afterShellArgs)

	// `email test [address]` (PART 17 "CLI Commands") is likewise a
	// positional subcommand, not a --flag, extracted the same way.
	emailFound, emailAction, emailAddress, afterEmailArgs := extractEmailFlag(afterSchedulerArgs)

	// `tor status|validate|restart|regenerate|vanity start <prefix>|
	// vanity apply|import-keys <path>` (PART 31 "CLI-to-running-server
	// control channel") is likewise a positional subcommand, not a --flag,
	// extracted the same way.
	torFound, torAction, torSubAction, torArg, afterTorArgs := extractTorCommand(afterEmailArgs)

	// `--maintenance backup [filename]` / `--maintenance restore <file>`
	// (PART 21 CLI surface) are value/subcommand aliases extracted the same
	// way, before flag.Parse() runs, so bare `--maintenance` keeps working
	// as the existing maintenance-mode boolean flag.
	maintenanceSubcommandFound, maintenanceAction, maintenanceTarget, remainingArgs := extractMaintenanceFlag(afterTorArgs)

	if err := flag.CommandLine.Parse(remainingArgs); err != nil {
		os.Exit(2)
	}

	// --backup {backup_dir} sets the backup directory for the whole process
	// (PART 8/21); running a backup immediately is done via
	// `--maintenance backup [file]`, handled separately below.
	if *backupDirFlag != "" {
		backupDirOverride = *backupDirFlag
	}

	// Handle --shell completions|init|help (can run without privileges).
	if shellFound {
		binaryName := filepath.Base(os.Args[0])
		os.Exit(shellcompl.Handle(os.Stdout, os.Stderr, shellSubcmd, shellName, binaryName, serverShellCommands, serverShellFlags))
	}

	// Handle `scheduler ...` (can run without starting the full server).
	if schedulerFound {
		os.Exit(handleSchedulerCLI(schedulerAction, schedulerTaskID))
	}

	// Handle `email test [address]` (can run without starting the full server).
	if emailFound {
		os.Exit(handleEmailCLI(emailAction, emailAddress))
	}

	// Handle `tor ...` (can run without starting the full server; mutating
	// actions require one already running — see handleTorCLI).
	if torFound {
		os.Exit(handleTorCLI(torAction, torSubAction, torArg))
	}

	// Handle --maintenance backup|restore (PART 21 CLI surface alias).
	if maintenanceSubcommandFound {
		switch maintenanceAction {
		case "backup":
			if err := createBackup(maintenanceTarget, true); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "restore":
			if err := restoreBackup(maintenanceTarget, true); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
		return
	}

	// Handle direct restore flag (spec: --restore <path>). --backup only sets
	// the backup directory (handled above) — it does not run a backup.
	if *restorePath != "" {
		if err := restoreBackup(*restorePath, true); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle help (can run without privileges)
	if *showHelp {
		printHelp(filepath.Base(os.Args[0]))
		return
	}

	// Handle version (can run without privileges)
	if *showVersion {
		fmt.Printf("%s %s (commit %s, built %s)\n", filepath.Base(os.Args[0]), Version, CommitID, BuildDate)
		return
	}

	// Handle status (can run without privileges)
	if *showStatus {
		exitCode := checkStatus()
		os.Exit(exitCode)
	}

	// Handle mode flag (sets mode and exits)
	if *modeFlag != "" {
		if err := setApplicationMode(*modeFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Handle --update [check|yes|branch {stable|beta|daily}] (PART 22).
	if updateFound {
		switch updateAction {
		case "check":
			if err := checkForUpdate(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "branch":
			if err := setUpdateBranch(updateBranchArg); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "yes":
			if err := checkAndUpdate(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		default:
			fmt.Fprintf(os.Stderr, "Error: invalid --update value %q (expected check, yes, or branch {stable|beta|daily})\n", updateAction)
			os.Exit(1)
		}
		return
	}

	// Handle service commands
	if *serviceCmd != "" {
		if err := handleServiceCommand(*serviceCmd); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Start server (possibly in maintenance mode)
	if err := run(*portFlag, *addressFlag, *configDirFlag, *dataDirFlag, *cacheDirFlag, *logDirFlag, *pidFileFlag, *baseURLFlag, *debugFlag, *colorFlag, *langFlag, *maintenanceMode, *daemonFlag); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

// extractUpdateFlag scans argv for --update/-update before flag.Parse() runs,
// since the stdlib flag package has no concept of an optional value (PART 22:
// --update [check|yes|branch {stable|beta|daily}]). It returns whether the
// flag was present, the resolved action ("check", "yes", or "branch"), the
// branch name when action is "branch", and the remaining args with every
// consumed token removed so flag.CommandLine.Parse never sees them.
func extractUpdateFlag(args []string) (found bool, action string, branch string, remaining []string) {
	action = "yes"
	remaining = make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]

		name := arg
		inlineValue := ""
		hasInlineValue := false
		if strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-") {
			trimmed := strings.TrimLeft(arg, "-")
			if idx := strings.Index(trimmed, "="); idx != -1 {
				name = trimmed[:idx]
				inlineValue = trimmed[idx+1:]
				hasInlineValue = true
			} else {
				name = trimmed
			}
		}

		if name != "update" {
			remaining = append(remaining, arg)
			continue
		}

		found = true

		value := inlineValue
		consumedNext := false
		if !hasInlineValue && i+1 < len(args) {
			next := args[i+1]
			switch next {
			case "check", "yes", "branch":
				value = next
				consumedNext = true
			}
		}

		switch value {
		case "", "yes":
			action = "yes"
		case "check":
			action = "check"
		case "branch":
			action = "branch"
			if consumedNext && i+2 < len(args) {
				branch = args[i+2]
				i++
			}
		default:
			action = value
		}

		if consumedNext {
			i++
		}
	}

	return found, action, branch, remaining
}

// extractSchedulerCommand recognizes the positional `scheduler` subcommand
// (PART 18 "CLI Commands": `list|show <id>|run <id>|enable <id>|disable <id>|history <id>`)
// before flag.Parse() runs, since it is not a --flag.
func extractSchedulerCommand(args []string) (found bool, action string, taskID string, remaining []string) {
	if len(args) == 0 || args[0] != "scheduler" {
		return false, "", "", args
	}
	found = true
	if len(args) > 1 {
		action = args[1]
	}
	if len(args) > 2 {
		taskID = args[2]
	}
	return found, action, taskID, args[len(args):]
}

// extractEmailFlag recognizes the positional `email` subcommand (PART 17
// "CLI Commands": `email test [address]`) before flag.Parse() runs, since
// it is not a --flag. Modeled directly on extractSchedulerCommand.
func extractEmailFlag(args []string) (found bool, action string, address string, remaining []string) {
	if len(args) == 0 || args[0] != "email" {
		return false, "", "", args
	}
	found = true
	if len(args) > 1 {
		action = args[1]
	}
	if len(args) > 2 {
		address = args[2]
	}
	return found, action, address, args[len(args):]
}

// extractTorCommand recognizes the positional `tor` subcommand (PART 31
// "CLI-to-running-server control channel": `status|validate|restart|
// regenerate|vanity start <prefix>|vanity apply|import-keys <path>`) before
// flag.Parse() runs, since it is not a --flag. Modeled directly on
// extractSchedulerCommand.
func extractTorCommand(args []string) (found bool, action, subAction, arg string, remaining []string) {
	if len(args) == 0 || args[0] != "tor" {
		return false, "", "", "", args
	}
	found = true
	if len(args) > 1 {
		action = args[1]
	}
	switch action {
	case "vanity":
		if len(args) > 2 {
			subAction = args[2]
		}
		if subAction == "start" && len(args) > 3 {
			arg = args[3]
		}
	case "import-keys":
		if len(args) > 2 {
			arg = args[2]
		}
	}
	return found, action, subAction, arg, args[len(args):]
}

// extractMaintenanceFlag scans argv for --maintenance before flag.Parse() runs.
// Bare `--maintenance` (no following value) keeps its existing meaning: start
// the server in read-only maintenance mode. `--maintenance backup [filename]`
// and `--maintenance restore <file>` (PART 21 "Backup & Restore" CLI surface)
// are value/subcommand aliases for the same `backup.Create`/`backup.Restore`
// flow as the standalone `--backup`/`--restore` flags. Any other value falls
// through to the bare-flag (maintenance-mode) behavior so `flag.Bool` still
// parses it normally downstream.
func extractMaintenanceFlag(args []string) (subcommandFound bool, action string, target string, remaining []string) {
	remaining = make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]

		name := arg
		if strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-") {
			trimmed := strings.TrimLeft(arg, "-")
			if idx := strings.Index(trimmed, "="); idx == -1 {
				name = trimmed
			} else {
				name = trimmed[:idx]
			}
		}

		if name != "maintenance" {
			remaining = append(remaining, arg)
			continue
		}

		if i+1 < len(args) {
			next := args[i+1]
			if next == "backup" || next == "restore" {
				subcommandFound = true
				action = next
				i++
				if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
					target = args[i+1]
					i++
				}
				continue
			}
		}

		remaining = append(remaining, arg)
	}

	return subcommandFound, action, target, remaining
}

// handleSchedulerCLI implements `{project_name} scheduler list|show|run|enable|disable|history`
// per AI.md PART 18 "CLI Commands". It opens server.db directly, registers
// the same 11 built-in tasks (without starting the ticker), and operates on
// persisted state — safe to run alongside an already-running server since
// all reads/writes go through server.db.
func handleSchedulerCLI(action, taskID string) int {
	applyTZEnv()

	defaultConfigDir, _, defaultLogsDir := paths.GetDefaultDirs(ProjectName)
	configDir := firstNonEmpty(os.Getenv("CONFIG_DIR"), defaultConfigDir)
	logsDir := firstNonEmpty(os.Getenv("LOG_DIR"), defaultLogsDir)
	configPath := filepath.Join(configDir, "server.yml")

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load configuration: %v\n", err)
		return 1
	}
	cfg.Server.FQDN = resolveFQDN(cfg.Server.FQDN)

	dbHandle, err := db.Open(cfg.Server.Database, ProjectName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open database: %v\n", err)
		return 1
	}
	defer func() { _ = dbHandle.Close() }()

	if err := scheduler.EnsureSchema(dbHandle); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize scheduler schema: %v\n", err)
		return 1
	}

	catchUpWindow, parseErr := time.ParseDuration(cfg.Server.Scheduler.CatchUpWindow)
	if parseErr != nil || catchUpWindow <= 0 {
		catchUpWindow = time.Hour
	}
	sched := scheduler.NewWithLocation(dbHandle, catchUpWindow, resolveSchedulerLocation(cfg.Server.Scheduler.Timezone))
	blockStore := server.NewBlockStore(cfg.Server.Security.BlockedIPs)
	portInt, _ := strconv.Atoi(cfg.Server.Port)
	torMgr := tor.NewManager(ProjectName, portInt, cfg.Server.Tor)
	deps := buildSchedulerDeps(cfg, configDir, logsDir, nil, blockStore, cfg.Server.Port, torMgr)
	registerSchedulerTasks(sched, cfg, deps)

	switch action {
	case "", "list":
		printSchedulerList(sched)
	case "show":
		if taskID == "" {
			fmt.Fprintln(os.Stderr, "Error: scheduler show requires a task id")
			return 1
		}
		st, ok := sched.Get(taskID)
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: unknown task %q\n", taskID)
			return 1
		}
		printSchedulerTask(st)
	case "run":
		if taskID == "" {
			fmt.Fprintln(os.Stderr, "Error: scheduler run requires a task id")
			return 1
		}
		if err := sched.RunNow(taskID); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Printf("Task %q executed\n", taskID)
	case "enable":
		if taskID == "" {
			fmt.Fprintln(os.Stderr, "Error: scheduler enable requires a task id")
			return 1
		}
		if err := sched.Enable(taskID); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Printf("Task %q enabled\n", taskID)
	case "disable":
		if taskID == "" {
			fmt.Fprintln(os.Stderr, "Error: scheduler disable requires a task id")
			return 1
		}
		if err := sched.Disable(taskID); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Printf("Task %q disabled\n", taskID)
	case "history":
		if taskID == "" {
			fmt.Fprintln(os.Stderr, "Error: scheduler history requires a task id")
			return 1
		}
		entries, err := sched.History(taskID, 20)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		printSchedulerHistory(taskID, entries)
	default:
		fmt.Fprintf(os.Stderr, "Error: unknown scheduler command %q (expected list, show, run, enable, disable, or history)\n", action)
		return 1
	}
	return 0
}

// notifyRecipient resolves the operator notification recipient per AI.md
// PART 17: server.notifications.email.reply_to > server.notifications.
// email.from.email > server.web.security.admin (security.txt contact).
// Returns "" when none are configured, in which case notify.Send correctly
// errors on the empty recipient and the caller should just log a warning.
func notifyRecipient(cfg *config.Config) string {
	return firstNonEmpty(
		cfg.Server.Notifications.Email.ReplyTo,
		cfg.Server.Notifications.Email.From.Email,
		cfg.Web.Security.Admin)
}

// handleEmailCLI implements `{project_name} email test [address]` per AI.md
// PART 17 "CLI Commands". It loads server.yml, resolves a recipient address
// (explicit argument > server.notifications.email.reply_to >
// server.notifications.email.from.email > server.web.security.admin), and
// sends the "test" template through notify.Send — the exact same SMTP path
// real notifications use.
func handleEmailCLI(action, address string) int {
	if action != "" && action != "test" {
		fmt.Fprintf(os.Stderr, "Error: unknown email command %q (expected test)\n", action)
		return 1
	}

	defaultConfigDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configDir := firstNonEmpty(os.Getenv("CONFIG_DIR"), defaultConfigDir)
	configPath := filepath.Join(configDir, "server.yml")

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load configuration: %v\n", err)
		return 1
	}

	recipient := firstNonEmpty(address, notifyRecipient(cfg))
	if recipient == "" {
		fmt.Fprintln(os.Stderr, "Error: no recipient address given and none configured (reply_to, from.email, web.security.admin are all empty)")
		return 1
	}

	if !notify.CanSend(cfg) {
		fmt.Fprintln(os.Stderr, "Error: email notifications are disabled — no SMTP host configured or notifications not enabled")
		return 1
	}

	if err := notify.Send(cfg, configDir, "test", recipient, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to send test email: %v\n", err)
		return 1
	}

	fmt.Printf("Test email sent to %s\n", recipient)
	return 0
}

// errNoRunningTorServer is the exact literal AI.md PART 31 requires for
// tor restart/regenerate/vanity start/vanity apply/import-keys when no
// running server is detected — printed verbatim via the existing
// `fmt.Fprintf(os.Stderr, "Error: %v\n", err)` convention used throughout
// this file, not a paraphrase.
var errNoRunningTorServer = errors.New("no running server detected — start the server first")

// torServerBaseURL resolves the running server's loopback base URL using the
// identical discovery mechanism as --status (PART 31: "identical to how
// --status locates the running server ... no new discovery mechanism is
// introduced"): server.yml's configured port, probed over 127.0.0.1. Returns
// ("", false) when no server answers.
func torServerBaseURL() (string, bool) {
	defaultConfigDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configDir := firstNonEmpty(os.Getenv("CONFIG_DIR"), defaultConfigDir)
	cfg, err := config.Load(filepath.Join(configDir, "server.yml"))
	if err != nil || cfg.Server.Port == "" {
		return "", false
	}
	baseURL := fmt.Sprintf("http://127.0.0.1:%s", cfg.Server.Port)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL + "/server/tor/status")
	if err != nil {
		return "", false
	}
	defer func() { _ = resp.Body.Close() }()
	return baseURL, true
}

// torControlEnvelope mirrors the canonical {"ok":true,"data":...} /
// {"ok":false,"error","message"} response shape (AI.md PART 14) returned by
// every /server/tor/* handler.
type torControlEnvelope struct {
	OK      bool            `json:"ok"`
	Data    json.RawMessage `json:"data"`
	Error   string          `json:"error"`
	Message string          `json:"message"`
}

// torControlRequest issues method against {baseURL}{path} (with an optional
// raw body) and decodes the canonical envelope, printing the operator-
// facing error message on failure.
func torControlRequest(method, baseURL, path string, body io.Reader) (torControlEnvelope, error) {
	req, err := http.NewRequest(method, baseURL+path, body)
	if err != nil {
		return torControlEnvelope{}, err
	}
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return torControlEnvelope{}, err
	}
	defer func() { _ = resp.Body.Close() }()

	var env torControlEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return torControlEnvelope{}, fmt.Errorf("failed to decode response: %w", err)
	}
	if !env.OK {
		return env, fmt.Errorf("%s: %s", env.Error, env.Message)
	}
	return env, nil
}

// printTorStatusData prints a decoded tor.Status payload in the same
// key/value style as the Tor section of `--status` output (PART 31 "CLI"
// table).
func printTorStatusData(data json.RawMessage) {
	var st struct {
		Enabled      bool   `json:"enabled"`
		Running      bool   `json:"running"`
		OnionAddress string `json:"onion_address"`
		VirtualPort  int    `json:"virtual_port"`
		ServerPort   int    `json:"server_port"`
	}
	_ = json.Unmarshal(data, &st)

	switch {
	case !st.Enabled:
		fmt.Println("Tor: disabled")
	case st.Running:
		fmt.Println("Tor: running")
		if st.OnionAddress != "" {
			fmt.Printf("Address: %s.onion\n", st.OnionAddress)
		}
		if st.VirtualPort != 0 {
			fmt.Printf("Virtual Port: %d\n", st.VirtualPort)
		}
	default:
		fmt.Println("Tor: enabled, not running")
	}
}

// handleTorCLI implements `{project_name} tor status|validate|restart|
// regenerate|vanity start <prefix>|vanity apply|import-keys <path>` per
// AI.md PART 31 "CLI-to-running-server control channel". status/validate
// fall back to on-disk state when no server is running (both are read-only);
// every other action requires a live server and exits 1 with the exact
// literal error text PART 31 specifies when none is detected.
func handleTorCLI(action, subAction, arg string) int {
	baseURL, running := torServerBaseURL()

	switch action {
	case "", "status":
		if running {
			env, err := torControlRequest(http.MethodGet, baseURL, "/server/tor/status", nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return 1
			}
			printTorStatusData(env.Data)
			return 0
		}
		printTorStatusFromDisk()
		return 0

	case "validate":
		if running {
			if _, err := torControlRequest(http.MethodPost, baseURL, "/server/tor/validate", nil); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return 1
			}
			fmt.Println("Tor configuration is valid")
			return 0
		}
		return validateTorFromDisk()

	case "restart":
		if !running {
			fmt.Fprintf(os.Stderr, "Error: %v\n", errNoRunningTorServer)
			return 1
		}
		env, err := torControlRequest(http.MethodPost, baseURL, "/server/tor/restart", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Println("Tor restarted")
		printTorStatusData(env.Data)
		return 0

	case "regenerate":
		if !running {
			fmt.Fprintf(os.Stderr, "Error: %v\n", errNoRunningTorServer)
			return 1
		}
		env, err := torControlRequest(http.MethodPost, baseURL, "/server/tor/regenerate", nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Println("Tor address regenerated")
		printTorStatusData(env.Data)
		return 0

	case "vanity":
		if !running {
			fmt.Fprintf(os.Stderr, "Error: %v\n", errNoRunningTorServer)
			return 1
		}
		switch subAction {
		case "start":
			if arg == "" {
				fmt.Fprintln(os.Stderr, "Error: tor vanity start requires a prefix")
				return 1
			}
			if _, err := torControlRequest(http.MethodPost, baseURL, "/server/tor/vanity/start?prefix="+url.QueryEscape(arg), nil); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return 1
			}
			fmt.Printf("Vanity address search started for prefix %q\n", arg)
			return 0
		case "apply":
			env, err := torControlRequest(http.MethodPost, baseURL, "/server/tor/vanity/apply", nil)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return 1
			}
			fmt.Println("Vanity address applied")
			printTorStatusData(env.Data)
			return 0
		default:
			fmt.Fprintf(os.Stderr, "Error: unknown tor vanity command %q (expected start or apply)\n", subAction)
			return 1
		}

	case "import-keys":
		if !running {
			fmt.Fprintf(os.Stderr, "Error: %v\n", errNoRunningTorServer)
			return 1
		}
		if arg == "" {
			fmt.Fprintln(os.Stderr, "Error: tor import-keys requires a file path")
			return 1
		}
		data, err := os.ReadFile(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to read key file: %v\n", err)
			return 1
		}
		env, err := torControlRequest(http.MethodPost, baseURL, "/server/tor/import-keys", bytes.NewReader(data))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			return 1
		}
		fmt.Println("Tor keys imported")
		printTorStatusData(env.Data)
		return 0

	default:
		fmt.Fprintf(os.Stderr, "Error: unknown tor command %q (expected status, validate, restart, regenerate, vanity, or import-keys)\n", action)
		return 1
	}
}

// printTorStatusFromDisk implements the `tor status` read-only fallback
// (PART 31: "MAY fall back to reading on-disk state ... when no server is
// running") by reading {data_dir}/tor/site/hostname directly.
func printTorStatusFromDisk() {
	_, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	hostname, err := os.ReadFile(filepath.Join(dataDir, "tor", "site", "hostname"))
	if err != nil {
		fmt.Println("Tor: disabled or not yet initialized")
		return
	}
	fmt.Println("Tor: not running (server stopped)")
	fmt.Printf("Address: %s\n", strings.TrimSpace(string(hostname)))
}

// validateTorFromDisk implements the `tor validate` read-only fallback by
// checking that {config_dir}/tor/torrc exists and is non-empty.
func validateTorFromDisk() int {
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	info, err := os.Stat(filepath.Join(configDir, "tor", "torrc"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: no torrc found — Tor has not been configured")
		return 1
	}
	if info.Size() == 0 {
		fmt.Fprintln(os.Stderr, "Error: torrc is empty")
		return 1
	}
	fmt.Println("Tor configuration is valid")
	return 0
}

func printSchedulerList(sched *scheduler.Scheduler) {
	states := sched.List()
	fmt.Printf("%-20s %-8s %-14s %-20s %-9s\n", "TASK", "ENABLED", "STATUS", "NEXT RUN", "RUNS/FAILS")
	for _, st := range states {
		next := "-"
		if !st.NextRun.IsZero() {
			next = st.NextRun.Format("2006-01-02 15:04:05")
		}
		status := st.LastStatus
		if status == "" {
			status = "never run"
		}
		fmt.Printf("%-20s %-8t %-14s %-20s %d/%d\n", st.ID, st.Enabled, status, next, st.RunCount, st.FailCount)
	}
}

func printSchedulerTask(st scheduler.TaskState) {
	fmt.Printf("Task:        %s\n", st.ID)
	fmt.Printf("Name:        %s\n", st.Name)
	fmt.Printf("Schedule:    %s\n", st.Schedule)
	fmt.Printf("Enabled:     %t\n", st.Enabled)
	if !st.LastRun.IsZero() {
		fmt.Printf("Last run:    %s\n", st.LastRun.Format(time.RFC3339))
	} else {
		fmt.Println("Last run:    never")
	}
	fmt.Printf("Last status: %s\n", st.LastStatus)
	if st.LastError != "" {
		fmt.Printf("Last error:  %s\n", st.LastError)
	}
	if !st.NextRun.IsZero() {
		fmt.Printf("Next run:    %s\n", st.NextRun.Format(time.RFC3339))
	}
	fmt.Printf("Run count:   %d\n", st.RunCount)
	fmt.Printf("Fail count:  %d\n", st.FailCount)
}

func printSchedulerHistory(taskID string, entries []scheduler.HistoryEntry) {
	fmt.Printf("History for %q:\n", taskID)
	if len(entries) == 0 {
		fmt.Println("  (no history)")
		return
	}
	for _, h := range entries {
		line := fmt.Sprintf("  %s  %-7s  %dms", h.RanAt.Format(time.RFC3339), h.Status, h.DurationMS)
		if h.Error != "" {
			line += "  error: " + h.Error
		}
		fmt.Println(line)
	}
}

func run(portFlag, addressFlag, configDirFlag, dataDirFlag, cacheDirFlag, logDirFlag, pidFileFlag, baseURLFlag string, debug bool, color string, lang string, maintenance bool, daemon bool) error {
	// Apply the TZ env var (AI.md PART 8 "Runtime Variables") as early as
	// possible so every subsequent log timestamp and time.Now() call uses
	// it; applyTZEnv silently keeps the system default on an invalid value
	// per config-rules.md "invalid config value -> warn, never crash".
	applyTZEnv()

	// --daemon detaches from the terminal before any other startup work
	// (PART 8 "Server Binary Commands": "Daemonize (detach from terminal)").
	if daemon {
		if err := service.Daemonize(); err != nil {
			return fmt.Errorf("failed to daemonize: %w", err)
		}
	}

	// Propagate build-time version info into the server package so that
	// /server/about and /healthz return real values, not "dev"/"unknown".
	server.Version = Version
	server.CommitID = CommitID
	server.BuildDate = BuildDate

	// lang carries the requested UI language (server-side i18n consumes this
	// via config once locale infrastructure is wired up); currently just
	// validated for shape and passed through for future use.
	_ = lang

	// Handle color flag (auto, yes, no); resolveColorSetting also stores the
	// validated value into colorSetting for getEmoji to consult.
	if _, err := resolveColorSetting(color); err != nil {
		return err
	}

	if maintenance {
		log.Printf("Starting %s API server v%s in MAINTENANCE mode (read-only)", ProjectName, Version)
	} else {
		log.Printf("Starting %s API server v%s", ProjectName, Version)
	}
	log.Printf("Commit: %s, Built: %s", CommitID, BuildDate)
	if OfficialSite != "" {
		log.Printf("Official site: %s", OfficialSite)
	}

	// Get OS-specific default directories
	defaultConfigDir, defaultDataDir, defaultLogsDir := paths.GetDefaultDirs(ProjectName)

	// Allow overrides via flags or environment variables
	configDir := firstNonEmpty(configDirFlag, os.Getenv("CONFIG_DIR"), defaultConfigDir)
	dataDir := firstNonEmpty(dataDirFlag, os.Getenv("DATA_DIR"), defaultDataDir)
	logsDir := firstNonEmpty(logDirFlag, os.Getenv("LOG_DIR"), defaultLogsDir)
	cacheDir := firstNonEmpty(cacheDirFlag, os.Getenv("CACHE_DIR"), paths.GetCacheDir(ProjectName))
	pidPath := firstNonEmpty(pidFileFlag, os.Getenv("PID_FILE"), paths.GetPIDFile(ProjectName))
	baseURL := normalizeBaseURL(baseURLFlag)
	paths.SetBinaryBaseName(ProjectName)

	// If running as root, create the dedicated system user/group BEFORE
	// creating directories, per AI.md PART 9/23 "Server Startup Sequence"
	// step 8a — the binary handles user/group creation during normal
	// startup; --service --install never does this itself. No-op on
	// Windows (Virtual Service Account, managed by the SCM) and skipped
	// entirely when not root, since useradd/dscl require privileges.
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		if err := service.EnsureSystemUser(); err != nil {
			log.Printf("Warning: failed to create system user: %v", err)
		}
	}

	// Ensure directories exist
	if err := paths.EnsureDirs(configDir, dataDir, logsDir); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}
	if err := paths.EnsureDir(cacheDir); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	if err := paths.EnsureDir(filepath.Dir(pidPath)); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	log.Printf("Config directory: %s", configDir)
	log.Printf("Data directory: %s", dataDir)
	log.Printf("Logs directory: %s", logsDir)
	log.Printf("Cache directory: %s", cacheDir)

	// Load configuration from YAML file (.yml per BASE.md)
	configPath := filepath.Join(configDir, "server.yml")
	log.Printf("Loading configuration from: %s", configPath)

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	log.Println("Configuration loaded successfully")

	// DOMAIN env var is the highest-priority FQDN override (after only the
	// per-request reverse-proxy headers handled in server.publicBaseURL),
	// per AI.md PART 8 "Runtime Variables" and api-rules.md FQDN resolution
	// order. Falls back through os.Hostname() -> $HOSTNAME -> outbound IP
	// -> localhost when neither DOMAIN nor server.yml set a value.
	cfg.Server.FQDN = resolveFQDN(cfg.Server.FQDN)
	log.Printf("FQDN: %s", cfg.Server.FQDN)

	// SMTP autodetection / connection test, per AI.md PART 17 "SMTP
	// Auto-Detection Process" and "Connection Test (when host is set)". A
	// newly detected host:port is persisted to server.yml (email features
	// enabled); a configured-but-failing connection is disabled in memory
	// ONLY for this run (never persisted), so the next startup retries.
	if cfg.Server.Notifications.Email.SMTP.Host == "" {
		if host, smtpPort, ok := notify.Autodetect(cfg.Server.FQDN); ok {
			cfg.Server.Notifications.Email.SMTP.Host = host
			cfg.Server.Notifications.Email.SMTP.Port = smtpPort
			cfg.Server.Notifications.Email.Enabled = true
			log.Printf("SMTP auto-detected: %s:%d", host, smtpPort)
			if saveErr := config.SaveCurrent(); saveErr != nil {
				log.Printf("Warning: failed to persist auto-detected SMTP settings: %v", saveErr)
			}
		} else {
			log.Println("SMTP auto-detection found no server — email notifications disabled")
		}
	} else {
		smtpCfg := cfg.Server.Notifications.Email.SMTP
		if testErr := notify.TestConnection(smtpCfg.Host, smtpCfg.Port, smtpCfg.Username, smtpCfg.Password, smtpCfg.TLS); testErr != nil {
			log.Printf("Warning: configured SMTP connection test failed (%s:%d): %v — email disabled for this run, will retry next startup", smtpCfg.Host, smtpCfg.Port, testErr)
			cfg.Server.Notifications.Email.Enabled = false
		} else {
			cfg.Server.Notifications.Email.Enabled = true
		}
	}
	log.Printf("email.configured=%t", notify.CanSend(cfg))

	if err := notify.Send(cfg, configDir, "startup", notifyRecipient(cfg), nil); err != nil {
		log.Printf("Warning: failed to send startup notification email: %v", err)
	}

	// Resolve application mode per AI.md PART 6 priority: --mode CLI flag is a
	// persist-and-exit maintenance action (see setApplicationMode), so the
	// highest-priority source available here is the value it persisted to
	// server.yml; Initialize falls back to the MODE env var, then production.
	if err := mode.Initialize(cfg.Server.Mode); err != nil {
		log.Printf("Warning: invalid persisted mode %q: %v", cfg.Server.Mode, err)
	}

	// Resolve debug flag per AI.md PART 6 priority: --debug flag > DEBUG env
	// var > --mode debug/MODE=debug alias (already applied by Initialize
	// above) > default false.
	mode.InitializeDebug(debug)
	if mode.IsDebug() {
		log.Println("Debug mode enabled")
	}

	// Check if running in container
	inContainer := paths.IsRunningInContainer()

	// Determine port: Flag > Config > ENV > (container default 80) > Random.
	// Per AI.md PART 4 "Container Port Behavior": the container's internal
	// default is always 0.0.0.0:80 (Docker maps a random 64xxx host port to
	// it) — containers never fall through to the host/local random-64xxx
	// port-selection behavior.
	port := firstNonEmpty(portFlag, cfg.Server.Port, os.Getenv("PORT"))
	if port == "" && inContainer {
		port = "80"
	}
	if port == "" {
		port, err = findRandomPort()
		if err != nil {
			return fmt.Errorf("failed to find available port: %w", err)
		}
		log.Printf("Selected random available port: %s", port)
		// Save to config for persistence
		if err := config.SetPort(port); err != nil {
			log.Printf("Warning: Failed to save port to config: %v", err)
		}
	}

	// Determine listen address
	address := firstNonEmpty(addressFlag, cfg.Server.Address, os.Getenv("LISTEN"), "0.0.0.0")

	if inContainer {
		log.Println("Running in container environment")
	}

	// Bind the listen address while still privileged, then permanently drop
	// to the dedicated service user, per AI.md PART 5 "Privileged Port
	// Binding" Unix-Like Platforms steps 4-5: root binds any port, then
	// drops to an unprivileged UID/GID before data/DB loading and serving
	// begin. No-op on Windows, which uses a Virtual Service Account instead
	// of a runtime UID/GID switch (AI.md PART 23 "Windows Service Account").
	// Captured before any privilege drop below — service.IsElevated() reflects
	// the *live* EUID, which DropPrivileges permanently changes, so this must
	// be recorded now to know whether the process *started* elevated (used
	// later for the PID file permission decision, PART 8 Directory Validation
	// Rules: 0644 when root/system mode, 0600 in user mode).
	startedElevated := service.IsElevated()

	var privilegedListener net.Listener
	if runtime.GOOS != "windows" && service.IsElevated() {
		privilegedListener, err = net.Listen("tcp", address+":"+port)
		if err != nil {
			return fmt.Errorf("failed to bind %s:%s: %w", address, port, err)
		}

		uid, gid, idErr := service.ServiceUserIDs()
		if idErr != nil {
			return fmt.Errorf("failed to resolve service user for privilege drop: %w", idErr)
		}
		for _, dir := range []string{configDir, dataDir, logsDir} {
			if chownErr := os.Chown(dir, uid, gid); chownErr != nil {
				log.Printf("Warning: failed to chown %s to service user: %v", dir, chownErr)
			}
		}
		if dropErr := service.DropPrivileges(uid, gid); dropErr != nil {
			return fmt.Errorf("failed to drop privileges: %w", dropErr)
		}
		log.Printf("Dropped privileges to user %q (uid=%d, gid=%d)", ProjectName, uid, gid)
	}

	// Load airport data
	log.Println("Loading airport database...")
	airportSvc, err := airports.NewService(airportsData)
	if err != nil {
		return fmt.Errorf("failed to load airports: %w", err)
	}
	stats := airportSvc.Stats()
	log.Printf("Loaded %d airports from %d countries", stats["total_airports"], stats["countries"])

	// Load GeoIP data (optional - continue without if fails). The GeoIP
	// service joins "geoip" onto the base directory it is given, so the base
	// must be {data_dir}/security to land databases in the spec-mandated
	// {data_dir}/security/geoip path (AI.md PART 19).
	log.Println("Loading GeoIP databases...")
	geoipSvc, err := geoip.NewService(filepath.Join(dataDir, "security"))
	if err != nil {
		log.Printf("Warning: GeoIP initialization failed: %v", err)
		log.Println("GeoIP features will be unavailable")
		geoipSvc = nil
	} else {
		defer geoipSvc.Close()
		log.Println("GeoIP databases loaded successfully")
	}

	// Open server.db and initialize the scheduler's persistent state per
	// AI.md PART 18 "Persistent State" — task status must survive restarts.
	dbHandle, err := db.Open(cfg.Server.Database, ProjectName)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer func() {
		if closeErr := dbHandle.Close(); closeErr != nil {
			log.Printf("Warning: error closing database: %v", closeErr)
		}
	}()
	if err := scheduler.EnsureSchema(dbHandle); err != nil {
		return fmt.Errorf("failed to initialize scheduler schema: %w", err)
	}

	catchUpWindow, parseErr := time.ParseDuration(cfg.Server.Scheduler.CatchUpWindow)
	if parseErr != nil || catchUpWindow <= 0 {
		catchUpWindow = time.Hour
	}
	sched := scheduler.NewWithLocation(dbHandle, catchUpWindow, resolveSchedulerLocation(cfg.Server.Scheduler.Timezone))

	// Apply maintenance mode flag to config before creating server.
	if maintenance {
		cfg.Server.Mode = "maintenance"
	}

	// Construct the Tor manager once, before the server, so the exact same
	// *tor.Manager instance can be shared between server.New() (which
	// reports live Tor state via /server/healthz, AI.md PART 13) and the
	// scheduler deps (which start/stop the process, AI.md PART 31).
	portInt, _ := strconv.Atoi(port)
	torMgr := tor.NewManager(ProjectName, portInt, cfg.Server.Tor)

	// Create HTTP server
	srv := server.New(airportSvc, geoipSvc, cfg, sched, dbHandle, dataDir, configDir, torMgr)
	defer func() {
		if closeErr := srv.CloseCache(); closeErr != nil {
			log.Printf("Warning: error closing cache backend: %v", closeErr)
		}
	}()

	// registerSchedulerTasks needs srv.BlockStore() so the scheduled
	// blocklist_update task shares the exact in-memory store the live HTTP
	// middleware enforces against — no restart required for updates to
	// take effect.
	schedDeps := buildSchedulerDeps(cfg, configDir, logsDir, geoipSvc, srv.BlockStore(), port, torMgr)
	registerSchedulerTasks(sched, cfg, schedDeps)

	// Start scheduler
	sched.Start()
	defer sched.Stop()
	log.Println("Scheduler started")

	// Start the dedicated Tor process (per AI.md PART 31) if enabled in
	// config. Non-fatal on failure — the server continues serving over
	// clearnet even if the hidden service could not start.
	if schedDeps.torMgr != nil && schedDeps.torMgr.Enabled() {
		// Bound the bootstrap wait by the configurable
		// server.tor.bootstrap_timeout (seconds) rather than a hardcoded
		// value; Tor is best-effort and never blocks or fails startup
		// (AI.md PART 31). Fall back to the 180s default if unset/invalid.
		bootstrapTimeout := time.Duration(cfg.Server.Tor.BootstrapTimeout) * time.Second
		if bootstrapTimeout <= 0 {
			bootstrapTimeout = 180 * time.Second
		}
		torCtx, torCancel := context.WithTimeout(context.Background(), bootstrapTimeout)
		if startErr := schedDeps.torMgr.Start(torCtx); startErr != nil {
			log.Printf("Warning: Tor hidden service failed to start: %v", startErr)
		} else {
			log.Printf("Tor hidden service started: %s", schedDeps.torMgr.OnionAddress())
		}
		torCancel()
	}
	// --baseurl {path} mounts the entire router under a URL path prefix per
	// AI.md "Server Binary Commands" (default "/", i.e. no prefix/no-op).
	rootHandler := srv.Router()
	if baseURL != "/" {
		prefixed := chi.NewRouter()
		prefixed.Mount(baseURL, rootHandler)
		rootHandler = prefixed
		log.Printf("Mounted under base URL: %s", baseURL)
	}

	httpServer := &http.Server{
		Addr:         address + ":" + port,
		Handler:      rootHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Skip the PID file entirely inside containers — the container runtime
	// (Docker/Podman/CRI-O) supervises the process and PIDs are
	// namespace-local, so a PID file is meaningless there (AI.md PART 8).
	if !inContainer {
		// Stale PID detection (AI.md PART 8 "PID File Handling"): a crash or
		// kill -9 leaves the old PID file behind. Refuse to start only when
		// the file still points at a live instance of this same binary;
		// anything else (missing, corrupt, dead process, PID reused by an
		// unrelated program) is treated as stale and silently cleared.
		if running, existingPID, err := paths.CheckPIDFile(pidPath); err != nil {
			log.Printf("Warning: could not check PID file %s: %v", pidPath, err)
		} else if running {
			return fmt.Errorf("already running (pid %d)", existingPID)
		}

		// Write PID file (enabled by default per spec). Permission per PART 8
		// Directory Validation Rules: 0644 when running as root/system mode
		// (readable by monitoring tools), 0600 in unprivileged user mode.
		pidPerm := os.FileMode(0600)
		if startedElevated {
			pidPerm = 0644
		}
		if err := os.WriteFile(pidPath, fmt.Appendf(nil, "%d\n", os.Getpid()), pidPerm); err != nil {
			log.Printf("Warning: could not write PID file %s: %v", pidPath, err)
		} else {
			defer func() {
				if rmErr := os.Remove(pidPath); rmErr != nil {
					log.Printf("Warning: could not remove PID file %s: %v", pidPath, rmErr)
				}
			}()
		}
	}

	// Start server in goroutine
	go func() {
		url := getAccessibleURL(port)
		log.Printf("Server listening on %s", url)
		fmt.Printf("\n  %s Airports API Server v%s (commit %s, built %s)\n", getEmoji("plane"), Version, CommitID, BuildDate)
		fmt.Printf("  %s %s\n\n", getEmoji("link"), url)

		var serveErr error
		if privilegedListener != nil {
			// Port was already bound as root before dropping privileges;
			// Server.ListenAndServe would try (and fail) to re-bind it.
			serveErr = httpServer.Serve(privilegedListener)
		} else {
			serveErr = httpServer.ListenAndServe()
		}
		if serveErr != nil && serveErr != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Server error: %v\n", serveErr)
			os.Exit(1)
		}
	}()

	// Start automatic config file watcher — polls server.yml for external
	// edits and hot-reloads without requiring SIGHUP, per AI.md PART 5
	// "Live Reload" (file is the sole source of truth; no runtime API
	// mutation; changes apply immediately except port/address/ssl/database).
	configWatchDone := make(chan struct{})
	defer close(configWatchDone)
	go watchConfigFile(configPath, cfg, srv, configWatchDone)

	// Wait for a shutdown / reload / log-reopen / status-dump signal.
	// Per AI.md PART 8: SIGINT/SIGTERM/SIGQUIT/SIGRTMIN+3 → graceful
	// shutdown; SIGHUP → reload config; SIGUSR1 → reopen logs; SIGUSR2 →
	// status dump. The exact signal sets are platform-specific (Unix vs
	// Windows) and live in signals_unix.go / signals_windows.go.
	quit := make(chan os.Signal, 1)
	reload := make(chan os.Signal, 1)
	reopen := make(chan os.Signal, 1)
	statusDump := make(chan os.Signal, 1)
	signal.Notify(quit, shutdownSignals()...)
	// SIGHUP is defined on every platform we build for; on Windows os/signal
	// never delivers it, so this Notify is a harmless no-op there.
	signal.Notify(reload, syscall.SIGHUP)
	if sigs := logReopenSignals(); len(sigs) > 0 {
		signal.Notify(reopen, sigs...)
	}
	if sigs := statusDumpSignals(); len(sigs) > 0 {
		signal.Notify(statusDump, sigs...)
	}

	for {
		select {
		case <-reload:
			log.Println("SIGHUP received — reloading configuration")
			if newCfg, loadErr := config.Load(configPath); loadErr != nil {
				log.Printf("Warning: config reload failed: %v — keeping current config", loadErr)
			} else {
				cfg = newCfg
				srv.ReloadConfig(cfg)
				log.Println("Configuration reloaded successfully")
			}
		case <-reopen:
			// SIGUSR1: external log-rotation tools have rotated the files and
			// expect the process to reopen them (AI.md PART 8). A standalone
			// reopen path does not exist yet; the scheduler's log_rotation
			// task owns actual rotation, so receipt is acknowledged here.
			log.Println("SIGUSR1 received — log files will be reopened on the next rotation cycle")
		case <-statusDump:
			// SIGUSR2: dump a concise runtime status snapshot to the log
			// (AI.md PART 8).
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			log.Printf("SIGUSR2 status dump — version=%s commit=%s goroutines=%d heap_alloc=%dKB",
				Version, CommitID, runtime.NumGoroutine(), mem.HeapAlloc/1024)
		case <-quit:
			goto shutdown
		}
	}
shutdown:

	log.Println("Shutting down server...")

	if err := notify.Send(cfg, configDir, "shutdown", notifyRecipient(cfg), nil); err != nil {
		log.Printf("Warning: failed to send shutdown notification email: %v", err)
	}

	// Graceful shutdown with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown error: %w", err)
	}

	if schedDeps.torMgr != nil && schedDeps.torMgr.Enabled() {
		if stopErr := schedDeps.torMgr.Stop(); stopErr != nil {
			log.Printf("Warning: error stopping Tor hidden service: %v", stopErr)
		}
	}

	log.Println("Server stopped")
	return nil
}

// watchConfigFile polls configPath for external mtime changes every 5
// seconds and hot-reloads the running server, per AI.md PART 5 "Live
// Reload": server.yml is the sole source of truth and changes must apply
// without a restart. Settings that require a restart to fully take effect
// (port/address/ssl/database driver) are still applied to the in-memory
// config for consistency, but a warning is logged since the bound
// listener/connection pool cannot change without a process restart.
func watchConfigFile(configPath string, initial *config.Config, srv *server.Server, done <-chan struct{}) {
	lastMod := time.Time{}
	if info, err := os.Stat(configPath); err == nil {
		lastMod = info.ModTime()
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	current := initial
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			info, err := os.Stat(configPath)
			if err != nil || info.ModTime().Equal(lastMod) {
				continue
			}
			lastMod = info.ModTime()

			newCfg, loadErr := config.Load(configPath)
			if loadErr != nil {
				log.Printf("Config file watcher: reload failed: %v — keeping current config", loadErr)
				continue
			}

			warnRestartRequiredChanges(current, newCfg)
			srv.ReloadConfig(newCfg)
			current = newCfg
			log.Println("Configuration hot-reloaded from file change")
		}
	}
}

// warnRestartRequiredChanges logs a warning for settings that changed but
// require a full process restart to take effect, per AI.md PART 5 "Live
// Reload" exceptions: listen address/port and database driver change.
func warnRestartRequiredChanges(oldCfg, newCfg *config.Config) {
	var changed []string
	if oldCfg.Server.Port != newCfg.Server.Port {
		changed = append(changed, "server.port")
	}
	if oldCfg.Server.Address != newCfg.Server.Address {
		changed = append(changed, "server.address")
	}
	if oldCfg.Server.SSL.Enabled != newCfg.Server.SSL.Enabled {
		changed = append(changed, "server.ssl.enabled")
	}
	if oldCfg.Server.Database.Driver != newCfg.Server.Database.Driver {
		changed = append(changed, "server.database.driver")
	}
	if oldCfg.Server.Daemonize != newCfg.Server.Daemonize {
		changed = append(changed, "server.daemonize")
	}
	if oldCfg.Server.Cache.Type != newCfg.Server.Cache.Type || oldCfg.Server.Cache.URL != newCfg.Server.Cache.URL ||
		oldCfg.Server.Cache.Host != newCfg.Server.Cache.Host || oldCfg.Server.Cache.Port != newCfg.Server.Cache.Port {
		changed = append(changed, "server.cache")
	}
	if len(changed) > 0 {
		log.Printf("Warning: config changes require a restart to fully take effect: %s", strings.Join(changed, ", "))
	}
}

func printHelp(binaryName string) {
	fmt.Printf("%s - Airport location information API server\n", binaryName)
	fmt.Println()
	fmt.Printf("Usage: %s [OPTIONS]\n", binaryName)
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --help, -h            Show this help message")
	fmt.Println("  --version, -v         Show version information")
	fmt.Println("  --status              Show server status and exit with code")
	fmt.Println("  --mode MODE           Set application mode (production, development)")
	fmt.Println("  --port PORT           Set port (default: random 64xxx)")
	fmt.Println("  --address ADDR        Listen address (default: 0.0.0.0)")
	fmt.Println("  --config DIR          Configuration directory")
	fmt.Println("  --data DIR            Data directory")
	fmt.Println("  --cache DIR           Cache directory")
	fmt.Println("  --log DIR             Log directory")
	fmt.Println("  --backup DIR          Backup directory")
	fmt.Println("  --pid FILE            PID file path")
	fmt.Println("  --baseurl PATH        URL path prefix (default: /)")
	fmt.Println("  --daemon              Daemonize (detach from terminal)")
	fmt.Println("  --debug               Enable debug mode")
	fmt.Println("  --color {auto|yes|no} Color output (default: auto)")
	fmt.Println("  --lang CODE           Language code (e.g. en, es, zh, fr, ar, de, ja)")
	fmt.Println()
	fmt.Println("Shell Integration:")
	fmt.Println("  --shell completions [SHELL]  Print shell completions")
	fmt.Println("  --shell init [SHELL]         Print shell init command")
	fmt.Println("  --shell help                 Show shell help")
	fmt.Println()
	fmt.Println("Update Commands:")
	fmt.Println("  --update check        Check for updates")
	fmt.Println("  --update yes          Install available updates")
	fmt.Println("  --update branch NAME  Set update branch (stable, beta, daily)")
	fmt.Println()
	fmt.Println("Service Commands:")
	fmt.Println("  --service start       Start the service")
	fmt.Println("  --service stop        Stop the service")
	fmt.Println("  --service restart     Restart the service")
	fmt.Println("  --service reload      Reload configuration")
	fmt.Println("  --service status      Show service status")
	fmt.Println("  --service enable      Enable service at boot")
	fmt.Println("  --service disable     Disable service at boot")
	fmt.Println("  --service logs        Tail recent service logs")
	fmt.Println("  --service --install   Install as system service")
	fmt.Println("  --service --uninstall Remove system service")
	fmt.Println("  --service --help      Show service help")
	fmt.Println()
	fmt.Println("Scheduler Commands:")
	fmt.Println("  scheduler list                List all scheduled tasks")
	fmt.Println("  scheduler show <id>           Show a task's detailed status")
	fmt.Println("  scheduler run <id>            Run a task immediately")
	fmt.Println("  scheduler enable <id>         Enable a task")
	fmt.Println("  scheduler disable <id>        Disable a task")
	fmt.Println("  scheduler history <id>        Show a task's execution history")
	fmt.Println()
	fmt.Println("Email Commands:")
	fmt.Println("  email test [address]          Send a test email (default recipient from config)")
	fmt.Println()
	fmt.Println("Tor Commands:")
	fmt.Println("  tor status                    Show Tor hidden service status")
	fmt.Println("  tor validate                  Validate the Tor configuration")
	fmt.Println("  tor restart                   Restart the Tor process")
	fmt.Println("  tor regenerate                Generate a new onion address")
	fmt.Println("  tor vanity start <prefix>     Search for a vanity onion address")
	fmt.Println("  tor vanity apply              Apply the completed vanity address search result")
	fmt.Println("  tor import-keys <path>        Import an existing onion service key file")
	fmt.Println()
	fmt.Println("Maintenance / Admin:")
	fmt.Println("  --maintenance                 Start in maintenance mode (read-only)")
	fmt.Println("  --maintenance backup [path]   Run a backup now")
	fmt.Println("  --restore <path>              Restore from backup archive")
	fmt.Println()
	fmt.Println("Environment Variables:")
	fmt.Println("  CONFIG_DIR            Configuration directory path")
	fmt.Println("  DATA_DIR              Data directory path")
	fmt.Println("  CACHE_DIR             Cache directory path")
	fmt.Println("  LOG_DIR               Logs directory path")
	fmt.Println("  BACKUP_DIR            Backup directory path")
	fmt.Println("  DATABASE_DIR          SQLite database directory path")
	fmt.Println("  PID_FILE              PID file path")
	fmt.Println("  PORT                  Server port")
	fmt.Println("  LISTEN                Listen address")
	fmt.Println("  MODE                  Application mode (production, development)")
	fmt.Println("  DOMAIN                FQDN override (comma-separated, first entry wins)")
	fmt.Println("  TZ                    Server timezone (IANA name, e.g. America/New_York)")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Printf("  Root: /etc/%s/%s/server.yml\n", ProjectOrg, ProjectName)
	fmt.Printf("  User: ~/.config/%s/server.yml\n", ProjectName)
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  %s                          # Start with defaults\n", binaryName)
	fmt.Printf("  %s --port 8080              # Start on port 8080\n", binaryName)
	fmt.Printf("  %s --maintenance            # Start in maintenance mode\n", binaryName)
	fmt.Printf("  %s --maintenance backup      # Create backup now\n", binaryName)
	fmt.Printf("  %s --service install        # Install as service\n", binaryName)
}

func checkStatus() int {
	// Try to connect to health endpoint
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configPath := filepath.Join(configDir, "server.yml")

	cfg, err := config.Load(configPath)
	if err != nil || cfg.Server.Port == "" {
		fmt.Println("Status: Not configured")
		return 1
	}

	statusClient := &http.Client{Timeout: 5 * time.Second}
	resp, err := statusClient.Get(fmt.Sprintf("http://127.0.0.1:%s/healthz", cfg.Server.Port))
	if err != nil {
		fmt.Printf("Status: Not running (port %s)\n", cfg.Server.Port)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		var health map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&health)
		fmt.Printf("Status: Running on port %s\n", cfg.Server.Port)
		if data, ok := health["data"].(map[string]interface{}); ok {
			if status, ok := data["status"].(string); ok {
				fmt.Printf("Health: %s\n", status)
			}
		}
		return 0
	}

	fmt.Printf("Status: Unhealthy (HTTP %d)\n", resp.StatusCode)
	return 1
}

// serviceEscalationActions maps each service subcommand needing root/admin
// to a human-readable action description used in escalation prompts, per
// AI.md PART 5 "Commands Requiring Escalation" table — every entry there
// is ⚠️ Check except `status`, which never escalates (read-only).
var serviceEscalationActions = map[string]string{
	"start":     "Starting the service",
	"stop":      "Stopping the service",
	"restart":   "Restarting the service",
	"reload":    "Reloading the service",
	"enable":    "Enabling the service",
	"disable":   "Disabling the service",
	"install":   "Installing the service",
	"uninstall": "Uninstalling the service",
}

func handleServiceCommand(cmd string) error {
	if action, needsEscalation := serviceEscalationActions[cmd]; needsEscalation {
		if err := service.HandleEscalation(action); err != nil {
			return err
		}
	}

	switch cmd {
	case "start":
		return service.Start()
	case "stop":
		return service.Stop()
	case "restart":
		return service.Restart()
	case "reload":
		return service.Reload()
	case "status":
		return service.Status()
	case "enable":
		return service.Enable()
	case "disable":
		return service.Disable()
	case "logs":
		return service.Logs()
	case "install":
		// Per AI.md PART 23 the install flow must detect the init system,
		// write the service file, enable it, AND start it. Install() writes
		// and enables; start it immediately afterwards.
		if err := service.Install(); err != nil {
			return err
		}
		return service.Start()
	case "uninstall":
		return service.Uninstall()
	default:
		return fmt.Errorf("unknown service command: %s", cmd)
	}
}

// schedulerDeps bundles every subsystem a scheduled task handler may need,
// per AI.md PART 18 "Built-in Tasks". Constructed once at startup (see
// buildSchedulerDeps) and shared by both call sites: the live server's run()
// and the standalone `scheduler run <id>` CLI path in handleSchedulerCLI.
type schedulerDeps struct {
	geoipSvc   *geoip.Service
	cveSvc     *cve.Service
	blockStore *server.BlockStore
	logMgr     *logging.Manager
	torMgr     *tor.Manager
	sslMgr     *ssl.Manager
	fqdn       string
	healthURL  string
	configDir  string
}

// toLoggingConfig bridges config.LoggingConfig (server.yml's `server.logs`
// schema, PART 11 "Logging" § Configuration) to logging.Config, the log
// rotation subsystem's own shape. Only Audit and Debug expose an `enabled`
// toggle in server.yml (matching PART 11 — every other log type is always
// on), so Access/Server/Error/Security are always mapped Enabled: true.
func toLoggingConfig(lc config.LoggingConfig) logging.Config {
	return logging.Config{
		Level: lc.Level,
		Access: logging.AccessFileConfig{
			FileConfig: logging.FileConfig{
				Enabled:  true,
				Filename: lc.Access.Filename,
				Format:   lc.Access.Format,
				Custom:   lc.Access.Custom,
				Rotate:   lc.Access.Rotate,
				Keep:     lc.Access.Keep,
			},
			LogHealthChecks: lc.Access.LogHealthChecks,
		},
		Server: logging.FileConfig{
			Enabled:  true,
			Filename: lc.Server.Filename,
			Format:   lc.Server.Format,
			Custom:   lc.Server.Custom,
			Rotate:   lc.Server.Rotate,
			Keep:     lc.Server.Keep,
		},
		Error: logging.FileConfig{
			Enabled:  true,
			Filename: lc.Error.Filename,
			Format:   lc.Error.Format,
			Custom:   lc.Error.Custom,
			Rotate:   lc.Error.Rotate,
			Keep:     lc.Error.Keep,
		},
		Audit: logging.AuditFileConfig{
			FileConfig: logging.FileConfig{
				Enabled:  lc.Audit.Enabled,
				Filename: lc.Audit.Filename,
				Format:   lc.Audit.Format,
				Custom:   lc.Audit.Custom,
				Rotate:   lc.Audit.Rotate,
				Keep:     lc.Audit.Keep,
				Compress: lc.Audit.Compress,
			},
		},
		Security: logging.FileConfig{
			Enabled:  true,
			Filename: lc.Security.Filename,
			Format:   lc.Security.Format,
			Custom:   lc.Security.Custom,
			Rotate:   lc.Security.Rotate,
			Keep:     lc.Security.Keep,
		},
		Debug: logging.FileConfig{
			Enabled:  lc.Debug.Enabled,
			Filename: lc.Debug.Filename,
			Format:   lc.Debug.Format,
			Custom:   lc.Debug.Custom,
			Rotate:   lc.Debug.Rotate,
			Keep:     lc.Debug.Keep,
		},
	}
}

// toSSLConfig bridges config.SSLConfig (server.yml's `server.ssl` schema) to
// ssl.Config, the SSL/Let's Encrypt subsystem's own shape. certPath is the
// on-disk directory the ssl package uses for both the autocert cache and
// manually-placed certificates (paths.GetSSLDir), never a config value.
// encryptionKey is server.security.encryption_key, used to decrypt
// cfg.LetsEncrypt.DNSCredentials (an AES-256-GCM blob) into the plaintext
// provider credential map the ssl package needs, per AI.md PART 15
// "Provider Credential Storage". If decryption fails (e.g. dns-01 not
// configured yet, or the blob is empty), DNSCredentials is left empty and
// the failure is logged rather than aborting startup.
func toSSLConfig(cfg config.SSLConfig, encryptionKey string, certPath string) ssl.Config {
	var dnsCreds map[string]string
	if cfg.LetsEncrypt.DNSCredentials != "" {
		decrypted, err := ssl.DecryptDNSCredentials(encryptionKey, cfg.LetsEncrypt.DNSCredentials)
		if err != nil {
			log.Printf("Warning: failed to decrypt dns-01 provider credentials: %v", err)
		} else {
			dnsCreds = decrypted
		}
	}

	return ssl.Config{
		Enabled:  cfg.Enabled,
		CertPath: certPath,
		LetsEncrypt: ssl.LetsEncryptConfig{
			Enabled:        cfg.LetsEncrypt.Enabled,
			Email:          cfg.LetsEncrypt.Email,
			Challenge:      ssl.ParseChallenge(cfg.LetsEncrypt.Challenge),
			Staging:        cfg.LetsEncrypt.Staging,
			DNSProvider:    cfg.LetsEncrypt.DNSProvider,
			DNSCredentials: dnsCreds,
		},
	}
}

// buildSchedulerDeps constructs every subsystem registerSchedulerTasks needs.
// blockStore is passed in rather than constructed here so the live server
// path can share the exact instance the HTTP middleware enforces against
// (see server.Server.BlockStore) — the CLI-only scheduler path constructs
// its own standalone store instead, since it has no running server to share
// with. listenPort is used only to build the self health-check URL; it may
// be empty in the CLI-only path, in which case healthcheck_self becomes a
// no-op. torMgr is constructed once by the caller (so the exact same
// *tor.Manager instance can also be shared with server.New(), letting
// /server/healthz report the live Tor state) and simply attached here.
func buildSchedulerDeps(cfg *config.Config, configDir, logsDir string, geoipSvc *geoip.Service, blockStore *server.BlockStore, listenPort string, torMgr *tor.Manager) schedulerDeps {
	deps := schedulerDeps{
		geoipSvc:   geoipSvc,
		blockStore: blockStore,
		fqdn:       cfg.Server.FQDN,
		torMgr:     torMgr,
		configDir:  configDir,
	}

	securityDir := paths.GetSecurityDir(ProjectName)
	if cveSvc, err := cve.NewService(securityDir, cfg.Server.CVE.APIKey); err != nil {
		log.Printf("Warning: CVE service unavailable: %v", err)
	} else {
		deps.cveSvc = cveSvc
	}

	deps.logMgr = logging.NewManager(logsDir, toLoggingConfig(cfg.Server.Logging))

	sslCfg := toSSLConfig(cfg.Server.SSL, cfg.Server.Security.EncryptionKey, paths.GetSSLDir(ProjectName))
	if sslCfg.LetsEncrypt.Challenge == "dns-01" && sslCfg.LetsEncrypt.DNSProvider != "" {
		if err := ssl.ValidateDNSProviderCredentials(sslCfg.LetsEncrypt.DNSProvider, sslCfg.LetsEncrypt.DNSCredentials); err != nil {
			log.Printf("Warning: dns-01 provider %q credentials invalid: %v", sslCfg.LetsEncrypt.DNSProvider, err)
		} else {
			log.Printf("dns-01 provider %q credentials validated", sslCfg.LetsEncrypt.DNSProvider)
		}
	}

	sslMgr := ssl.NewManager(sslCfg)
	if cfg.Server.SSL.Enabled && cfg.Server.SSL.LetsEncrypt.Enabled && cfg.Server.FQDN != "" {
		if _, err := sslMgr.GetTLSConfig([]string{cfg.Server.FQDN}); err != nil {
			log.Printf("Warning: Let's Encrypt initialization failed: %v", err)
		}
	}
	deps.sslMgr = sslMgr

	if listenPort != "" {
		deps.healthURL = fmt.Sprintf("http://127.0.0.1:%s/server/healthz", listenPort)
	}

	return deps
}

// registerSchedulerTasks wires up the 11 built-in scheduled tasks required
// by AI.md PART 18 "Built-in Tasks". token_cleanup is a permanent no-op, not
// deferred work: AI.md PART 10 "Resource Owner Tokens" (line 11812) states
// resource owner tokens exist only when a project's resources are writable
// ("a read-only stats API generates none"), and IDEA.md "Product scope &
// non-goals" explicitly rules out any write/mutation API and any user
// accounts for airports. There is no resource-creation flow that could ever
// issue a tok_-prefixed owner token, so the api_tokens table is never
// populated and this task has nothing to clean up.
func registerSchedulerTasks(sched *scheduler.Scheduler, cfg *config.Config, deps schedulerDeps) {
	tasks := cfg.Server.Scheduler.Tasks

	mustRegister := func(id, name, schedule string, enabledDefault, retryOnFail bool, retryDelayStr string, handler func() error) {
		retryDelay := time.Minute * 5
		if retryDelayStr != "" {
			if d, err := time.ParseDuration(retryDelayStr); err == nil && d > 0 {
				retryDelay = d
			}
		}
		// Wrap every task handler with a generic scheduler_error notification
		// (AI.md PART 17/18). A more specific event (backup_failed,
		// ssl_renewal_failed) suppresses this duplicate by wrapping its own
		// returned error with notify.ErrSuppressScheduler.
		wrapped := func() error {
			err := handler()
			if err != nil && !errors.Is(err, notify.ErrSuppressScheduler) {
				nextRun := ""
				if next := sched.NextRun(id); !next.IsZero() {
					nextRun = next.Format(time.RFC1123)
				}
				if sendErr := notify.Send(cfg, deps.configDir, "scheduler_error", notifyRecipient(cfg), map[string]string{
					"task_id":   id,
					"task_name": name,
					"error":     err.Error(),
					"next_run":  nextRun,
				}); sendErr != nil {
					log.Printf("Warning: failed to send scheduler_error notification email: %v", sendErr)
				}
			}
			return err
		}
		if err := sched.RegisterTask(id, name, schedule, enabledDefault, retryOnFail, retryDelay, wrapped); err != nil {
			log.Printf("scheduler: failed to register task %q: %v", id, err)
		}
	}

	mustRegister("geoip_update", "GeoIP database update", tasks.GeoIPUpdate.Schedule,
		tasks.GeoIPUpdate.Enabled, tasks.GeoIPUpdate.RetryOnFail, tasks.GeoIPUpdate.RetryDelay,
		func() error {
			if deps.geoipSvc == nil {
				return fmt.Errorf("geoip service unavailable")
			}
			return deps.geoipSvc.UpdateDatabases()
		})

	mustRegister("blocklist_update", "IP blocklist update", tasks.BlocklistUpdate.Schedule,
		tasks.BlocklistUpdate.Enabled, tasks.BlocklistUpdate.RetryOnFail, tasks.BlocklistUpdate.RetryDelay,
		func() error {
			if deps.blockStore == nil {
				return fmt.Errorf("blocklist store unavailable")
			}
			return server.UpdateBlocklist(deps.blockStore, ProjectName)
		})

	mustRegister("cve_update", "CVE database update", tasks.CVEUpdate.Schedule,
		tasks.CVEUpdate.Enabled, tasks.CVEUpdate.RetryOnFail, tasks.CVEUpdate.RetryDelay,
		func() error {
			if deps.cveSvc == nil {
				return fmt.Errorf("cve service unavailable")
			}
			return deps.cveSvc.Update()
		})

	mustRegister("update_check", "Update check", tasks.UpdateCheck.Schedule,
		tasks.UpdateCheck.Enabled, false, "",
		func() error {
			return runScheduledUpdateCheck(cfg, deps.configDir)
		})

	mustRegister("token_cleanup", "Expired token cleanup", tasks.TokenCleanup.Schedule,
		tasks.TokenCleanup.Enabled, false, "",
		func() error {
			// Permanent no-op per AI.md PART 10 "Resource Owner Tokens"
			// (a read-only project generates none) and IDEA.md's explicit
			// no-write/no-accounts scope — no api_tokens rows are ever
			// created, so there is nothing to expire. Honest no-op, not
			// a fake stub or deferred work.
			return nil
		})

	mustRegister("log_rotation", "Log rotation", tasks.LogRotation.Schedule,
		tasks.LogRotation.Enabled, false, "",
		func() error {
			if deps.logMgr == nil {
				return fmt.Errorf("log manager unavailable")
			}
			return deps.logMgr.Rotate()
		})

	mustRegister("backup_daily", "Daily backup", cfg.Server.Scheduler.Tasks.Backup.Schedule,
		cfg.Server.Scheduler.Tasks.Backup.Enabled, false, "",
		func() error {
			return createBackup("", false)
		})

	mustRegister("backup_hourly", "Hourly incremental backup", tasks.BackupHourly.Schedule,
		tasks.BackupHourly.Enabled, false, "",
		func() error {
			return createIncrementalBackup("hourly")
		})

	mustRegister("ssl_renewal", "SSL certificate renewal", tasks.SSLRenewal.Schedule,
		tasks.SSLRenewal.Enabled, false, "",
		func() error {
			if deps.sslMgr == nil || deps.fqdn == "" {
				return nil
			}
			renewed, err := deps.sslMgr.RenewIfNeeded([]string{deps.fqdn})
			if err != nil {
				expiryDate := ""
				expiresIn := ""
				if expiry, expErr := deps.sslMgr.CertificateExpiry(deps.fqdn); expErr == nil {
					expiryDate = expiry.Format(time.RFC1123)
					expiresIn = fmt.Sprintf("%d", int(time.Until(expiry).Hours()/24))
				}
				nextRetry := ""
				if next := sched.NextRun("ssl_renewal"); !next.IsZero() {
					nextRetry = next.Format(time.RFC1123)
				}
				if sendErr := notify.Send(cfg, deps.configDir, "ssl_renewal_failed", notifyRecipient(cfg), map[string]string{
					"fqdn":        deps.fqdn,
					"error":       err.Error(),
					"expiry_date": expiryDate,
					"expires_in":  expiresIn,
					"next_retry":  nextRetry,
				}); sendErr != nil {
					log.Printf("Warning: failed to send ssl_renewal_failed notification email: %v", sendErr)
				}
				return fmt.Errorf("%w: %w", err, notify.ErrSuppressScheduler)
			}

			if renewed {
				if setErr := config.SetSSLLastExpiryWarningDays(0); setErr != nil {
					log.Printf("Warning: failed to reset ssl expiry warning state: %v", setErr)
				}
				validUntil := ""
				if expiry, expErr := deps.sslMgr.CertificateExpiry(deps.fqdn); expErr == nil {
					validUntil = expiry.Format(time.RFC1123)
				}
				if sendErr := notify.Send(cfg, deps.configDir, "ssl_renewed", notifyRecipient(cfg), map[string]string{
					"fqdn":        deps.fqdn,
					"valid_until": validUntil,
				}); sendErr != nil {
					log.Printf("Warning: failed to send ssl_renewed notification email: %v", sendErr)
				}
				return nil
			}

			expiry, expErr := deps.sslMgr.CertificateExpiry(deps.fqdn)
			if expErr != nil {
				return nil
			}
			daysLeft := int(time.Until(expiry).Hours() / 24)
			thresholds := []int{30, 14, 7, 3, 1}
			lastWarned := cfg.Server.SSL.LastExpiryWarningDays
			for _, threshold := range thresholds {
				if daysLeft > threshold {
					continue
				}
				if lastWarned != 0 && lastWarned <= threshold {
					break
				}
				log.Printf("SSL certificate for %s expires in %d day(s) (%s)", deps.fqdn, daysLeft, expiry.Format(time.RFC1123))
				if threshold <= 7 {
					if sendErr := notify.Send(cfg, deps.configDir, "ssl_expiring", notifyRecipient(cfg), map[string]string{
						"fqdn":        deps.fqdn,
						"expires_in":  fmt.Sprintf("%d", daysLeft),
						"expiry_date": expiry.Format(time.RFC1123),
					}); sendErr != nil {
						log.Printf("Warning: failed to send ssl_expiring notification email: %v", sendErr)
					}
				}
				if setErr := config.SetSSLLastExpiryWarningDays(threshold); setErr != nil {
					log.Printf("Warning: failed to persist ssl expiry warning state: %v", setErr)
				}
				break
			}
			return nil
		})

	mustRegister("healthcheck_self", "Self health check", tasks.HealthCheck.Schedule,
		tasks.HealthCheck.Enabled, false, "",
		func() error {
			if deps.healthURL == "" {
				return nil
			}
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Get(deps.healthURL)
			if err != nil {
				return fmt.Errorf("self health check failed: %w", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("self health check returned status %d", resp.StatusCode)
			}
			return nil
		})

	mustRegister("tor_health", "Tor hidden service health check", tasks.TorHealth.Schedule,
		tasks.TorHealth.Enabled, tasks.TorHealth.RetryOnFail, tasks.TorHealth.RetryDelay,
		func() error {
			if deps.torMgr == nil || !deps.torMgr.Enabled() {
				return nil
			}
			return deps.torMgr.HealthCheck()
		})
}

func printServiceHelp() {
	fmt.Println("Service Commands:")
	fmt.Println()
	fmt.Println("  install     Install as system service (does not start it)")
	fmt.Println("  uninstall   Remove system service (preserves config and data)")
	fmt.Println("  start       Start the service")
	fmt.Println("  stop        Stop the service")
	fmt.Println("  restart     Restart the service")
	fmt.Println("  reload      Reload configuration")
	fmt.Println("  status      Show service status")
	fmt.Println("  enable      Enable service at boot")
	fmt.Println("  disable     Disable service at boot")
	fmt.Println("  logs        Tail recent service logs")
	fmt.Println()
	fmt.Println("Supported service managers:")
	fmt.Println("  Linux:   systemd, OpenRC, runit")
	fmt.Println("  macOS:   launchd")
	fmt.Println("  Windows: Windows Service Manager")
	fmt.Println("  BSD:     rc.d")
}

// promptBackupPassword interactively reads a password from the terminal via
// golang.org/x/term.ReadPassword, per binary-rules.md "NEVER accept a backup
// password via CLI flag — interactive prompt only". Returns "" (no error) on
// non-interactive stdin (piped/redirected), since encryption is optional
// unless compliance mode requires it — the caller enforces that gate.
func promptBackupPassword(prompt string) (string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return "", nil
	}
	fmt.Fprint(os.Stderr, prompt)
	pw, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}
	return string(pw), nil
}

// resolveBackupDir applies the --backup {backup_dir} > BACKUP_DIR env var >
// OS-standard default priority chain (AI.md PART 8: --backup SETS the
// backup directory; it does not run a backup — see --maintenance backup).
func resolveBackupDir() string {
	return firstNonEmpty(backupDirOverride, os.Getenv("BACKUP_DIR"), paths.GetBackupDir(ProjectName))
}

// createBackup builds a backup archive via the src/backup package (AI.md
// PART 21) and applies retention afterward. An empty fileLocation resolves
// to {backup_dir}/{project}_backup_{date}.tar.gz[.enc], matching the
// scheduled-task naming convention. interactive controls whether a missing
// encryption password is prompted for (never for scheduled/non-TTY calls).
func createBackup(fileLocation string, interactive bool) error {
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	cfg := config.Get()

	password := cfg.Server.Backup.EncryptionPassword
	if password == "" && interactive {
		var err error
		password, err = promptBackupPassword("Backup encryption password (leave blank for unencrypted): ")
		if err != nil {
			return err
		}
	}

	if cfg.Server.Compliance.Enabled && password == "" {
		if !interactive {
			log.Printf("WARN: scheduled backup skipped — server.compliance.enabled is true but no server.backup.encryption_password is configured")
			return nil
		}
		return fmt.Errorf("compliance mode requires an encryption password — set server.backup.encryption_password or enter one when prompted")
	}

	backupDir := resolveBackupDir()
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	isScheduledDefaultLocation := fileLocation == ""
	if fileLocation == "" {
		date := time.Now().UTC().Format("2006-01-02")
		ext := "tar.gz"
		if password != "" {
			ext = "tar.gz.enc"
		}
		fileLocation = filepath.Join(backupDir, fmt.Sprintf("%s_backup_%s.%s", ProjectName, date, ext))
	}

	fmt.Printf("Creating backup: %s\n", fileLocation)

	if err := backup.Create(fileLocation, backup.CreateOptions{
		ConfigDir:          configDir,
		DataDir:            dataDir,
		IncludeSSL:         true,
		IncludeData:        true,
		EncryptionPassword: password,
		ComplianceEnabled:  cfg.Server.Compliance.Enabled,
		AppVersion:         Version,
		CreatedBy:          ProjectName,
	}); err != nil {
		if sendErr := notify.Send(cfg, configDir, "backup_failed", notifyRecipient(cfg), map[string]string{
			"filename": filepath.Base(fileLocation),
			"error":    err.Error(),
		}); sendErr != nil {
			log.Printf("Warning: failed to send backup_failed notification email: %v", sendErr)
		}
		return fmt.Errorf("%w: %w", err, notify.ErrSuppressScheduler)
	}

	if err := backup.Verify(fileLocation, password); err != nil {
		_ = os.Remove(fileLocation)
		if sendErr := notify.Send(cfg, configDir, "backup_failed", notifyRecipient(cfg), map[string]string{
			"filename": filepath.Base(fileLocation),
			"error":    err.Error(),
		}); sendErr != nil {
			log.Printf("Warning: failed to send backup_failed notification email: %v", sendErr)
		}
		return fmt.Errorf("backup verification failed, removed: %w: %w", err, notify.ErrSuppressScheduler)
	}

	if isScheduledDefaultLocation {
		// Scheduled/default-location run (backup_daily task, AI.md PART 21
		// "Backup Creation Flow" steps 5-6): also refresh the daily
		// incremental. A manual "--maintenance backup {custom-file}" run
		// never reaches this branch, since it always passes an explicit
		// fileLocation - it only produces the file the operator asked for.
		if incErr := createIncrementalBackup("daily"); incErr != nil {
			log.Printf("Warning: daily incremental backup failed: %v", incErr)
		}
	}

	maxTotalSize, err := backupMaxTotalSizeBytes(cfg, backupDir)
	if err != nil {
		log.Printf("Warning: invalid server.backup.retention.max_total_size: %v", err)
	}
	deleted, err := backup.ApplyRetention(backupDir, ProjectName, backup.RetentionConfig{
		MaxBackups:   cfg.Server.Backup.Retention.MaxBackups,
		KeepWeekly:   cfg.Server.Backup.Retention.KeepWeekly,
		KeepMonthly:  cfg.Server.Backup.Retention.KeepMonthly,
		KeepYearly:   cfg.Server.Backup.Retention.KeepYearly,
		MaxTotalSize: maxTotalSize,
	})
	if err != nil {
		log.Printf("Warning: backup retention sweep failed: %v", err)
	} else if len(deleted) > 0 {
		log.Printf("Backup retention removed %d old backup(s): %s", len(deleted), strings.Join(deleted, ", "))
	}

	backupSize := ""
	if info, statErr := os.Stat(fileLocation); statErr == nil {
		backupSize = fmt.Sprintf("%d", info.Size())
	}
	if sendErr := notify.Send(cfg, configDir, "backup_complete", notifyRecipient(cfg), map[string]string{
		"filename": filepath.Base(fileLocation),
		"size":     backupSize,
	}); sendErr != nil {
		log.Printf("Warning: failed to send backup_complete notification email: %v", sendErr)
	}

	fmt.Printf("Backup created successfully: %s\n", fileLocation)
	return nil
}

// createIncrementalBackup creates or replaces the fixed-name "{project}-{kind}"
// incremental backup file ({project_name}-{kind}.tar.gz[.enc]) in the
// default backup directory, per AI.md PART 21 "Backup Files Created" —
// kind is "daily" (called from createBackup's scheduled-location path) or
// "hourly" (called directly by the backup_hourly task). Unlike the dated
// full backup, this file is always exactly one, replaced on every
// successful run, and never subject to count-based retention (backup.
// ApplyRetention only matches the "{project}_backup_" prefix). A
// verification failure removes only the failed incremental and leaves every
// existing backup — full or incremental — untouched, matching the full
// backup's own fail-safe behavior.
func createIncrementalBackup(kind string) error {
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	cfg := config.Get()

	password := cfg.Server.Backup.EncryptionPassword
	if cfg.Server.Compliance.Enabled && password == "" {
		// Same compliance gate as the full backup (PART 21 "Compliance Mode
		// Enforcement"); the full-backup task already warns for this run.
		return nil
	}

	backupDir := resolveBackupDir()
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	ext := "tar.gz"
	if password != "" {
		ext = "tar.gz.enc"
	}
	fileLocation := filepath.Join(backupDir, fmt.Sprintf("%s-%s.%s", ProjectName, kind, ext))

	if err := backup.Create(fileLocation, backup.CreateOptions{
		ConfigDir:          configDir,
		DataDir:            dataDir,
		IncludeSSL:         true,
		IncludeData:        true,
		EncryptionPassword: password,
		ComplianceEnabled:  cfg.Server.Compliance.Enabled,
		AppVersion:         Version,
		CreatedBy:          ProjectName,
	}); err != nil {
		if sendErr := notify.Send(cfg, configDir, "backup_failed", notifyRecipient(cfg), map[string]string{
			"filename": filepath.Base(fileLocation),
			"error":    err.Error(),
		}); sendErr != nil {
			log.Printf("Warning: failed to send backup_failed notification email: %v", sendErr)
		}
		return fmt.Errorf("%s incremental backup failed: %w", kind, err)
	}

	if err := backup.Verify(fileLocation, password); err != nil {
		_ = os.Remove(fileLocation)
		if sendErr := notify.Send(cfg, configDir, "backup_failed", notifyRecipient(cfg), map[string]string{
			"filename": filepath.Base(fileLocation),
			"error":    err.Error(),
		}); sendErr != nil {
			log.Printf("Warning: failed to send backup_failed notification email: %v", sendErr)
		}
		return fmt.Errorf("%s incremental backup verification failed, removed: %w", kind, err)
	}

	backupSize := ""
	if info, statErr := os.Stat(fileLocation); statErr == nil {
		backupSize = fmt.Sprintf("%d", info.Size())
	}
	if sendErr := notify.Send(cfg, configDir, "backup_complete", notifyRecipient(cfg), map[string]string{
		"filename": filepath.Base(fileLocation),
		"size":     backupSize,
	}); sendErr != nil {
		log.Printf("Warning: failed to send backup_complete notification email: %v", sendErr)
	}

	fmt.Printf("%s incremental backup created successfully: %s\n", kind, fileLocation)
	return nil
}

// backupMaxTotalSizeBytes resolves server.backup.retention.max_total_size
// (percent or absolute string) to a byte count via the backup package's
// parser. Falls back to 0 (disabled) on parse failure.
func backupMaxTotalSizeBytes(cfg *config.Config, backupDir string) (int64, error) {
	value := cfg.Server.Backup.Retention.MaxTotalSize
	if value == "" {
		return 0, nil
	}
	return backup.ParseMaxTotalSize(value, backupDir)
}

// restoreBackup restores from a backup archive via the src/backup package
// (AI.md PART 21). An empty fileLocation resolves to the most recent backup
// in the default backup directory. interactive controls whether a missing
// decryption password is prompted for.
func restoreBackup(fileLocation string, interactive bool) error {
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)

	if fileLocation == "" {
		backupDir := resolveBackupDir()
		entries, err := os.ReadDir(backupDir)
		if err != nil {
			return fmt.Errorf("no backups found in %s", backupDir)
		}

		var latest string
		for _, entry := range entries {
			name := entry.Name()
			if strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tar.gz.enc") {
				if latest == "" || name > latest {
					latest = name
				}
			}
		}
		if latest == "" {
			return fmt.Errorf("no backup files found")
		}
		fileLocation = filepath.Join(backupDir, latest)
	}

	fmt.Printf("Restoring from: %s\n", fileLocation)

	password := ""
	if strings.HasSuffix(fileLocation, ".enc") && interactive {
		var err error
		password, err = promptBackupPassword("Backup decryption password: ")
		if err != nil {
			return err
		}
	}

	err := backup.Restore(fileLocation, backup.RestoreOptions{
		ConfigDir:  configDir,
		DataDir:    dataDir,
		Password:   password,
		AppVersion: Version,
	})
	var mismatch *backup.VersionMismatchWarning
	if err != nil {
		if errors.As(err, &mismatch) {
			log.Printf("Warning: %v", mismatch)
			fmt.Println("Restore completed successfully")
			return nil
		}
		return err
	}

	fmt.Println("Restore completed successfully")
	return nil
}

// isRunningInContainer reports whether the process is inside a Docker container.
func isRunningInContainer() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

// releaseInfo is the subset of the GitHub Releases API response used by the
// update flow (PART 22).
type releaseInfo struct {
	TagName     string    `json:"tag_name"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// matchesBranch reports whether release r is eligible for update channel
// branch, per AI.md PART 22 "Channel Semantics". Channels are cumulative:
// a stable (non-prerelease) release matches every channel; the beta channel
// additionally matches "-beta"-suffixed tags; the daily channel additionally
// matches 14-digit timestamp tags (YYYYMMDDHHMMSS) on top of beta.
func matchesBranch(r releaseInfo, branch string) bool {
	if !r.Prerelease {
		return true
	}
	isBeta := strings.HasSuffix(r.TagName, "-beta")
	isDaily := len(r.TagName) == 14 && !strings.Contains(r.TagName, ".")
	switch branch {
	case "beta":
		return isBeta
	case "daily":
		return isBeta || isDaily
	default:
		return false
	}
}

// fetchLatestRelease queries the GitHub Releases API for the latest stable
// release of this project (`/releases/latest`).
func fetchLatestRelease(httpClient *http.Client) (*releaseInfo, error) {
	resp, err := httpClient.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", ProjectOrg, ProjectName))
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		// PART 22: HTTP 404 from the GitHub API means no updates are available.
		return nil, nil
	}

	var release releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("failed to parse release info: %w", err)
	}
	return &release, nil
}

// fetchReleaseForBranch resolves the newest release eligible for the given
// update channel that differs from currentVersion, per PART 22 "Channel
// Semantics". stable uses the GitHub "latest" endpoint directly; beta/daily
// fetch the full releases list (newest-first) and return the first entry
// that matchesBranch, since channels are cumulative and the list is already
// sorted newest-first.
func fetchReleaseForBranch(httpClient *http.Client, branch, currentVersion string) (*releaseInfo, error) {
	if branch == "" {
		branch = "stable"
	}
	if branch == "stable" {
		release, err := fetchLatestRelease(httpClient)
		if err != nil {
			return nil, err
		}
		if release == nil {
			return nil, nil
		}
		if strings.TrimPrefix(release.TagName, "v") == currentVersion {
			return nil, nil
		}
		return release, nil
	}

	resp, err := httpClient.Get(fmt.Sprintf("https://api.github.com/repos/%s/%s/releases", ProjectOrg, ProjectName))
	if err != nil {
		return nil, fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	var releases []releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to parse release list: %w", err)
	}

	for i := range releases {
		r := releases[i]
		if matchesBranch(r, branch) && strings.TrimPrefix(r.TagName, "v") != currentVersion {
			return &r, nil
		}
	}
	return nil, nil
}

// loadUpdateBranch loads server.yml and returns the configured update
// channel, defaulting to "stable" if the config cannot be loaded or the
// field is unset (manual `--update check`/`--update yes` must still work
// even before first-run config exists).
func loadUpdateBranch() string {
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.Load(configPath)
	if err != nil || cfg.Server.Update.Branch == "" {
		return "stable"
	}
	return cfg.Server.Update.Branch
}

// checkForUpdate implements `--update check`: reports whether a newer release
// is available without downloading or installing anything. No privileges
// required. Exit code 0 whether or not an update is available; 1 on error.
// Always resolves the true latest release for the configured channel — the
// defer_days window applies only to the scheduled update_check task, never
// to an explicit manual operator action (PART 22 "Defer Semantics").
func checkForUpdate() error {
	fmt.Println("Checking for updates...")

	branch := loadUpdateBranch()
	httpClient := &http.Client{Timeout: 30 * time.Second}

	release, err := fetchReleaseForBranch(httpClient, branch, Version)
	if err != nil {
		return err
	}
	if release == nil {
		fmt.Printf("Already running latest version: %s\n", Version)
		return nil
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	fmt.Printf("New version available: %s (current: %s)\n", latestVersion, Version)
	fmt.Println("Run `--update yes` to install.")
	return nil
}

// runScheduledUpdateCheck implements the built-in `update_check` scheduler
// task (PART 22 "Scheduled Check"). Unlike manual `--update check`/`--update
// yes`, it filters candidate releases by defer_days and only fires the
// update_available notification once per newly-eligible version; if
// auto_install is enabled it also performs the install.
func runScheduledUpdateCheck(cfg *config.Config, configDir string) error {
	return runScheduledUpdateCheckWithClient(cfg, configDir, &http.Client{Timeout: 30 * time.Second})
}

// runScheduledUpdateCheckWithClient is the client-injectable core of
// runScheduledUpdateCheck, split out so tests can point release fetching at
// a local httptest.Server instead of the real GitHub API.
func runScheduledUpdateCheckWithClient(cfg *config.Config, configDir string, httpClient *http.Client) error {
	branch := cfg.Server.Update.Branch
	if branch == "" {
		branch = "stable"
	}

	release, err := fetchReleaseForBranch(httpClient, branch, Version)
	if err != nil {
		return err
	}
	if release == nil {
		return nil
	}

	// PART 22 "Defer Semantics": the scheduled task only adopts releases
	// that have been public for at least defer_days; manual invocation is
	// exempt from this, but the scheduler is not.
	deferDays := cfg.Server.Update.DeferDays
	if deferDays > 0 && !release.PublishedAt.IsZero() {
		if time.Since(release.PublishedAt) < time.Duration(deferDays)*24*time.Hour {
			return nil
		}
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")

	// "Fires once per version" — only log/notify the first time this
	// version becomes eligible, not on every scheduler run.
	if latestVersion != cfg.Server.Update.LastNotifiedVersion {
		log.Printf("WARN: update available: %s (current: %s, channel: %s)", latestVersion, Version, branch)
		if sendErr := notify.Send(cfg, configDir, "update_available", notifyRecipient(cfg), map[string]string{
			"current_version": Version,
			"new_version":     latestVersion,
			"channel":         branch,
		}); sendErr != nil {
			log.Printf("Warning: failed to send update_available notification email: %v", sendErr)
		}
		if err := config.SetLastNotifiedVersion(latestVersion); err != nil {
			log.Printf("Warning: failed to persist last notified update version: %v", err)
		} else {
			cfg.Server.Update.LastNotifiedVersion = latestVersion
		}
	}

	if !cfg.Server.Update.AutoInstall {
		return nil
	}

	previousVersion := Version
	if err := checkAndUpdate(); err != nil {
		return err
	}
	if sendErr := notify.Send(cfg, configDir, "update_installed", notifyRecipient(cfg), map[string]string{
		"old_version": previousVersion,
		"new_version": latestVersion,
	}); sendErr != nil {
		log.Printf("Warning: failed to send update_installed notification email: %v", sendErr)
	}
	return nil
}

// setUpdateBranch implements `--update branch {stable|beta|daily}`: writes
// the update channel to server.yml, following the same load/mutate/save
// pattern as setApplicationMode (PART 22: config is the single source of
// truth, no separate CLI-side state).
func setUpdateBranch(branch string) error {
	switch branch {
	case "stable", "beta", "daily":
		// valid
	default:
		return fmt.Errorf("invalid update branch: %q (expected stable, beta, or daily)", branch)
	}

	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configPath := filepath.Join(configDir, "server.yml")

	if _, err := config.Load(configPath); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := config.SetUpdateBranch(branch); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("Update branch set to: %s\n", branch)
	return nil
}

func checkAndUpdate() error {
	fmt.Println("Checking for updates...")

	// Refuse to self-update inside a Docker container — the image itself must be
	// replaced via `docker pull` / a new image build (PART 22).
	if isRunningInContainer() {
		return fmt.Errorf("self-update is not supported inside a Docker container\n"+
			"Pull the latest image instead:\n\n"+
			"  docker pull ghcr.io/%s/%s:latest\n\n"+
			"Then restart your container", ProjectOrg, ProjectName)
	}

	branch := loadUpdateBranch()
	httpClient := &http.Client{Timeout: 30 * time.Second}

	release, err := fetchReleaseForBranch(httpClient, branch, Version)
	if err != nil {
		return err
	}
	if release == nil {
		fmt.Printf("Already running latest version: %s\n", Version)
		return nil
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")

	fmt.Printf("New version available: %s (current: %s)\n", latestVersion, Version)

	// Find correct asset and SHA256SUMS for this platform
	assetName := fmt.Sprintf("%s-%s-%s", ProjectName, runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		assetName += ".exe"
	}

	var downloadURL, checksumURL string
	for _, asset := range release.Assets {
		switch asset.Name {
		case assetName:
			downloadURL = asset.BrowserDownloadURL
		case "SHA256SUMS":
			checksumURL = asset.BrowserDownloadURL
		}
	}

	if downloadURL == "" {
		return fmt.Errorf("no binary available for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// Fetch SHA256SUMS first so we can verify after download.
	var expectedHash string
	if checksumURL != "" {
		csResp, err := httpClient.Get(checksumURL)
		if err != nil {
			return fmt.Errorf("failed to fetch SHA256SUMS: %w", err)
		}
		defer csResp.Body.Close()
		const maxSumsSize = 1 << 20 // 1 MiB cap
		sumsData, err := io.ReadAll(io.LimitReader(csResp.Body, maxSumsSize))
		if err != nil {
			return fmt.Errorf("failed to read SHA256SUMS: %w", err)
		}
		for _, line := range strings.Split(string(sumsData), "\n") {
			// Format: "<hex>  <filename>" or "<hex> <filename>"
			parts := strings.Fields(line)
			if len(parts) == 2 && parts[1] == assetName {
				expectedHash = parts[0]
				break
			}
		}
		if expectedHash == "" {
			return fmt.Errorf("no SHA-256 entry found for %s in SHA256SUMS", assetName)
		}
	} else {
		return fmt.Errorf("SHA256SUMS not found in release assets — refusing to update without checksum verification")
	}

	fmt.Printf("Downloading %s...\n", assetName)

	// Download to temp file
	tmpFile, err := os.CreateTemp("", ProjectName+"-update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	dlResp, err := httpClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer dlResp.Body.Close()

	// Hash while writing; cap at 256 MiB to guard against unbounded streams.
	const maxBinarySize = 256 << 20
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmpFile, hasher), io.LimitReader(dlResp.Body, maxBinarySize)); err != nil {
		return fmt.Errorf("failed to save update: %w", err)
	}
	tmpFile.Close()

	// Verify SHA-256 checksum before replacing anything.
	actualHash := fmt.Sprintf("%x", hasher.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch for %s:\n  expected: %s\n  actual:   %s\nUpdate aborted", assetName, expectedHash, actualHash)
	}
	fmt.Println("Checksum verified.")

	// Get current binary path
	currentPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current binary path: %w", err)
	}

	// Replace binary atomically
	if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := os.Rename(tmpFile.Name(), currentPath); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	fmt.Printf("Updated to version %s\n", latestVersion)

	// Attempt to restart via service manager (PART 22).
	if err := service.Restart(); err != nil {
		// Not running as a managed service — inform the operator.
		fmt.Println("Please restart the service manually to apply the update.")
	} else {
		fmt.Println("Service restarted successfully.")
	}
	return nil
}

// Helper functions

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// normalizeBaseURL resolves the --baseurl {path} flag per AI.md PART 8
// "Server Binary Commands" ("Set URL path prefix (default: /)"): empty input
// defaults to "/", a leading slash is enforced, and any trailing slash is
// trimmed except when the result is the root path itself.
func normalizeBaseURL(baseURLFlag string) string {
	if baseURLFlag == "" {
		return "/"
	}
	p := baseURLFlag
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if len(p) > 1 {
		p = strings.TrimRight(p, "/")
	}
	if p == "" {
		p = "/"
	}
	return p
}

func findRandomPort() (string, error) {
	// Go 1.20+ auto-seeds the global rand; no explicit Seed needed.
	for i := 0; i < 100; i++ {
		port := 64000 + rand.Intn(1000)
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			ln.Close()
			return fmt.Sprintf("%d", port), nil
		}
	}

	return "", fmt.Errorf("could not find available port in range 64000-64999")
}

func getAccessibleURL(port string) string {
	hostname, err := os.Hostname()
	if err == nil && hostname != "" && hostname != "localhost" {
		if addrs, err := net.LookupHost(hostname); err == nil && len(addrs) > 0 {
			return fmt.Sprintf("http://%s:%s", hostname, port)
		}
	}

	if ip := getOutboundIP(); ip != "" {
		return fmt.Sprintf("http://%s:%s", ip, port)
	}

	if hostname != "" && hostname != "localhost" {
		return fmt.Sprintf("http://%s:%s", hostname, port)
	}

	return fmt.Sprintf("http://<your-host>:%s", port)
}

func getOutboundIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 2*time.Second)
	if err != nil {
		return ""
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}

// applyTZEnv sets process-wide time.Local from the TZ env var per AI.md
// PART 0 "Timezone: System timezone or TZ env var" and PART 8 "Runtime
// Variables". An invalid TZ value is logged and ignored (config-rules.md:
// invalid config -> warn, never crash) rather than falling back to the
// system default, since Go's time.LoadLocation already resolves an unset
// or empty TZ to the system zone on its own. The binary embeds
// "time/tzdata" (see the blank import at the top of this file) so this
// works even in the minimal Alpine runtime image, which ships no
// /usr/share/zoneinfo database.
func applyTZEnv() {
	tz := os.Getenv("TZ")
	if tz == "" {
		return
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Printf("Warning: invalid TZ value %q: %v — using system default timezone", tz, err)
		return
	}
	time.Local = loc
}

// resolveFQDN implements the non-request-time portion of AI.md PART 8/13's
// FQDN resolution order: DOMAIN env (comma-separated, first entry wins,
// highest priority per config precedence "env var > config file") ->
// configured server.yml value -> os.Hostname() -> $HOSTNAME -> outbound
// public IP -> "localhost". Per-request reverse-proxy headers are handled
// separately and take priority over all of this in server.publicBaseURL.
func resolveFQDN(configured string) string {
	if domainEnv := os.Getenv("DOMAIN"); domainEnv != "" {
		first := strings.TrimSpace(strings.SplitN(domainEnv, ",", 2)[0])
		if first != "" {
			return first
		}
	}
	if configured != "" {
		return configured
	}
	if hostname, err := os.Hostname(); err == nil && hostname != "" && hostname != "localhost" {
		return hostname
	}
	if hostnameEnv := os.Getenv("HOSTNAME"); hostnameEnv != "" && hostnameEnv != "localhost" {
		return hostnameEnv
	}
	if ip := getOutboundIP(); ip != "" {
		return ip
	}
	return "localhost"
}

// resolveSchedulerLocation parses server.scheduler.timezone (AI.md PART 18)
// into a *time.Location for cron.WithLocation. An empty or invalid value
// falls back to time.Local (itself already TZ-env-aware via applyTZEnv)
// rather than failing startup, per config-rules.md's validate-and-default
// rule.
func resolveSchedulerLocation(timezone string) *time.Location {
	if timezone == "" {
		return time.Local
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		log.Printf("Warning: invalid server.scheduler.timezone %q: %v — using local timezone", timezone, err)
		return time.Local
	}
	return loc
}

// colorSetting holds the resolved --color CLI flag value ("auto", "yes", or
// "no"), set once from run()'s validated flag value. getEmoji consults it
// first per AI.md PART 8 "NO_COLOR Support" priority order: CLI flag >
// NO_COLOR env var > TERM=dumb > default enabled.
var colorSetting = "auto"

// resolveColorSetting validates the --color flag value (auto/yes/no,
// case-sensitive per AI.md PART 8), defaults an empty value to "auto",
// stores the result in colorSetting for getEmoji to consult, and returns it.
// Split out from run() so the flag-handling logic is unit-testable without
// starting a real server.
func resolveColorSetting(color string) (string, error) {
	if color != "" && color != "auto" && color != "yes" && color != "no" {
		return "", fmt.Errorf("invalid --color value: %s (must be auto, yes, or no)", color)
	}
	if color == "" {
		color = "auto"
	}
	colorSetting = color
	return colorSetting, nil
}

// emojiEnabled resolves whether decorative emoji output should render, per
// AI.md PART 8 "NO_COLOR Support" EmojiEnabled() priority order: CLI --color
// flag overrides everything, then NO_COLOR, then TERM=dumb, defaulting to
// enabled.
func emojiEnabled() bool {
	switch colorSetting {
	case "yes":
		return true
	case "no":
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	return true
}

// getEmoji returns the named emoji, or an empty string when decorative
// output is disabled per emojiEnabled().
func getEmoji(name string) string {
	if !emojiEnabled() {
		return ""
	}
	emojis := map[string]string{
		"plane":   "✈️",
		"link":    "🔗",
		"check":   "✓",
		"cross":   "✗",
		"warning": "⚠️",
		"info":    "ℹ️",
		"success": "✅",
		"error":   "❌",
		"folder":  "📁",
		"file":    "📄",
		"gear":    "⚙️",
		"rocket":  "🚀",
	}
	if e, ok := emojis[name]; ok {
		return e
	}
	return ""
}

// setApplicationMode sets the application mode (production/development)
func setApplicationMode(modeStr string) error {
	// Validate the mode
	parsedMode, err := mode.ParseMode(modeStr)
	if err != nil {
		return err
	}

	// Get config path
	configDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configPath := filepath.Join(configDir, "server.yml")

	// Load existing config
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Update mode
	cfg.Server.Mode = string(parsedMode)

	// Save config
	if err := config.Save(configPath, cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Also set in runtime (SetFromValue applies the debug alias side effect
	// for a raw "debug" value, per AI.md PART 6)
	if err := mode.SetFromValue(modeStr); err != nil {
		return err
	}

	fmt.Printf("Application mode set to: %s\n", parsedMode)
	fmt.Println("Restart the service for the change to take full effect")
	return nil
}

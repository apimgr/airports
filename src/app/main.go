package app

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	ossignal "os/signal"
	"os/user"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	_ "time/tzdata" // embed the IANA timezone database for TZ / scheduler.timezone (alpine ships no zoneinfo)

	"github.com/go-chi/chi/v5"
	"golang.org/x/term"

	"github.com/apimgr/airports/src/airports"
	"github.com/apimgr/airports/src/backup"
	"github.com/apimgr/airports/src/common/display"
	"github.com/apimgr/airports/src/common/i18n"
	urlvars "github.com/apimgr/airports/src/common/urlvars"
	commonversion "github.com/apimgr/airports/src/common/version"
	"github.com/apimgr/airports/src/config"
	airportdata "github.com/apimgr/airports/src/data"
	"github.com/apimgr/airports/src/db"
	"github.com/apimgr/airports/src/geoip"
	"github.com/apimgr/airports/src/i2p"
	"github.com/apimgr/airports/src/logging"
	"github.com/apimgr/airports/src/mode"
	"github.com/apimgr/airports/src/notify"
	"github.com/apimgr/airports/src/paths"
	"github.com/apimgr/airports/src/scheduler"
	compliancerequirements "github.com/apimgr/airports/src/security/compliance"
	"github.com/apimgr/airports/src/security/cve"
	"github.com/apimgr/airports/src/server"
	appbanner "github.com/apimgr/airports/src/server/banner"
	"github.com/apimgr/airports/src/service"
	"github.com/apimgr/airports/src/shellcompl"
	appsignal "github.com/apimgr/airports/src/signal"
	"github.com/apimgr/airports/src/ssl"
	"github.com/apimgr/airports/src/tor"
)

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
	serverShellCommands = []string{"update", "service", "maintenance", "shell", "email", "tor", "scheduler"}
	serverShellFlags    = []string{
		"--help", "-h", "--version", "-v", "--status", "--mode", "--port", "--address",
		"--config", "--data", "--cache", "--log", "--pid", "--baseurl", "--daemon",
		"--debug", "--color", "--lang", "--backup", "--i2p",
		"--service", "--maintenance", "--update", "--shell",
	}

	// backupDirOverride holds the value of --backup {backup_dir} (PART 8/21:
	// --backup SETS the backup directory, it does not run a backup). Consulted
	// by resolveBackupDir() alongside BACKUP_DIR and the OS-standard default.
	backupDirOverride = ""
	backupDataDir     = ""
	resolvedBackupDir = ""
)

var diskLowState = struct {
	sync.Mutex
	volumes map[string]time.Time
}{volumes: make(map[string]time.Time)}

var databaseErrorState = struct {
	sync.Mutex
	failures         int
	since            time.Time
	lastNotification time.Time
}{}

var (
	errRestartForUpdate  = errors.New("staged update applied; restart required")
	updateRestartRequest = make(chan struct{}, 1)
)

// buildEpoch parses the embedded BuildEpoch ldflag; 0 when unset or invalid.
// Passed to the updater's release-freshness check so a rolling channel tag
// can be compared against this binary's own build time.
func buildEpoch() int64 {
	return commonversion.ParseBuildEpoch(BuildEpoch)
}

// init derives BuildDate (RFC 3339 UTC) from the embedded BuildEpoch.
func init() {
	BuildDate = commonversion.BuildDateFromEpoch(BuildEpoch, BuildDate)
}

type runConfig struct {
	Port, Address, ConfigDir, DataDir, CacheDir, LogDir, PIDFile, BaseURL, Mode string
	Debug, Daemon, EnableI2P, I2PFlagSet, Maintenance                           bool
	Color, Lang                                                                 string
	Version, CommitID, BuildEpoch, OfficialSite                                 string
	Listener                                                                    net.Listener
	Ready                                                                       chan<- struct{}
}

type acceptReadyListener struct {
	net.Listener
	ready chan struct{}
	once  sync.Once
}

func (listener *acceptReadyListener) Accept() (net.Conn, error) {
	listener.once.Do(func() { close(listener.ready) })
	return listener.Listener.Accept()
}

func waitForStartupAddress(readAddress func() string) string {
	deadline := time.Now().Add(5 * time.Second)
	for {
		if address := strings.TrimSpace(readAddress()); address != "" {
			return address
		}
		if time.Now().After(deadline) {
			return ""
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Main parses process commands and dispatches startup and maintenance operations.
func Main() {
	paths.SetStartedElevated(service.IsElevated())

	// Command line flags
	portFlag := flag.String("port", "", "HTTP port")
	addressFlag := flag.String("address", "", "Listen address")
	configDirFlag := flag.String("config", "", "Configuration directory")
	dataDirFlag := flag.String("data", "", "Data directory")
	modeFlag := flag.String("mode", "", "Application mode: production, development, debug")
	showVersion := flag.Bool("version", false, "Show version and exit")
	flag.BoolVar(showVersion, "v", false, "Show version and exit")
	showStatus := flag.Bool("status", false, "Show server status and exit")
	showHelp := flag.Bool("help", false, "Show help message")
	flag.BoolVar(showHelp, "h", false, "Show help message")
	debugFlag := flag.Bool("debug", false, "Enable debug mode")
	colorFlag := flag.String("color", "auto", "Color output: auto, yes, no")
	langFlag := flag.String("lang", "", "Language code (e.g. en, es, zh, fr, ar, de, ja)")
	backupDirFlag := flag.String("backup", "", "Backup directory")
	cacheDirFlag := flag.String("cache", "", "Cache directory")
	logDirFlag := flag.String("log", "", "Log directory")
	pidFileFlag := flag.String("pid", "", "PID file path")
	baseURLFlag := flag.String("baseurl", "", "URL path prefix (default: /)")
	daemonFlag := flag.Bool("daemon", false, "Daemonize (detach from terminal)")
	i2pFlag := flag.Bool("i2p", false, "Enable the optional I2P eepsite")

	// --maintenance starts the server in maintenance (read-only) mode per spec.
	maintenanceMode := flag.Bool("maintenance", false, "Start in maintenance mode (read-only; serves maintenance page)")

	// --update [check|yes|branch {stable|beta|devel}] is a value/subcommand flag,
	// not a plain boolean (PART 22). The stdlib flag package has no notion of an
	// optional value, so extract it from argv before flag.Parse() runs.
	commandArgs := normalizeMaintenanceSubcommands(os.Args[1:])
	updateFound, updateAction, updateBranchArg, afterUpdateArgs := extractUpdateFlag(commandArgs)

	// --shell {completions|init|help} [SHELL] is likewise a value/subcommand
	// flag (PART 32 "Shell Completions") extracted before flag.Parse() runs.
	shellFound, shellSubcmd, shellName, afterShellArgs := shellcompl.Extract(afterUpdateArgs)

	// `scheduler list|show <id>|run <id>|enable <id>|disable <id>|history <id>`
	// is a positional subcommand (PART 18 "CLI Commands"), not a --flag, so it
	// is extracted the same way before flag.Parse() runs.
	schedulerFound, schedulerAction, schedulerTaskID, afterSchedulerArgs := extractSchedulerCommand(afterShellArgs)

	// The canonical `--maintenance email test` spelling is normalized to this
	// positional parser before flag.Parse.
	emailFound, emailAction, emailAddress, emailDryRun, afterEmailArgs, emailParseErr := extractEmailFlag(afterSchedulerArgs)

	// The canonical `--maintenance tor ...` spelling is normalized to this
	// parser before flag.Parse (PART 31 "CLI-to-running-server control channel").
	// extracted the same way.
	torFound, torAction, torSubAction, torArg, afterTorArgs := extractTorCommand(afterEmailArgs)
	serviceFound, serviceAction, afterServiceArgs := extractServiceFlag(afterTorArgs)

	// `--maintenance backup [filename]`, `restore <file>`, `update`, or
	// `help` are value/subcommand aliases extracted the same
	// way, before flag.Parse() runs, so bare `--maintenance` keeps working
	// as the existing maintenance-mode boolean flag.
	maintenanceSubcommandFound, maintenanceAction, maintenanceTarget, remainingArgs := extractMaintenanceFlag(afterServiceArgs)

	if err := flag.CommandLine.Parse(remainingArgs); err != nil {
		os.Exit(2)
	}

	// Immediate-exit flags take precedence over every operational command
	// and must not initialize paths, configuration, or services (AI.md PART 8).
	if *showHelp {
		printHelp(filepath.Base(os.Args[0]))
		return
	}

	// Handle version (can run without privileges)
	if *showVersion {
		fmt.Println((commonversion.BuildInfo{Version: Version, CommitID: CommitID, BuildDate: BuildDate}).Format(filepath.Base(os.Args[0])))
		return
	}

	// Handle status (can run without privileges)
	if *showStatus {
		exitCode := checkStatus()
		os.Exit(exitCode)
	}
	if shellFound {
		binaryName := filepath.Base(os.Args[0])
		os.Exit(shellcompl.HandleShellCompletion(os.Stdout, os.Stderr, shellSubcmd, shellName, binaryName, serverShellCommands, serverShellFlags))
	}
	if serviceFound && serviceAction == "help" {
		printServiceHelp()
		return
	}
	if maintenanceSubcommandFound && maintenanceAction == "help" {
		printMaintenanceHelp(filepath.Base(os.Args[0]))
		return
	}
	if updateFound && updateAction == "help" {
		printUpdateHelp(filepath.Base(os.Args[0]))
		return
	}
	for _, override := range []struct {
		name  string
		value *string
	}{
		{name: "--config", value: configDirFlag},
		{name: "--data", value: dataDirFlag},
		{name: "--cache", value: cacheDirFlag},
		{name: "--log", value: logDirFlag},
		{name: "--backup", value: backupDirFlag},
		{name: "--pid", value: pidFileFlag},
	} {
		if *override.value == "" {
			continue
		}
		safe, err := paths.SafeDirPath(*override.value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid %s path: %v\n", override.name, err)
			os.Exit(2)
		}
		*override.value = safe
	}
	for _, variable := range []string{"CONFIG_DIR", "DATA_DIR", "CACHE_DIR", "LOG_DIR", "BACKUP_DIR", "PID_FILE"} {
		value := os.Getenv(variable)
		if value == "" {
			continue
		}
		safe, err := paths.SafeDirPath(value)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid %s path: %v\n", variable, err)
			os.Exit(2)
		}
		if err := os.Setenv(variable, safe); err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid %s path: %v\n", variable, err)
			os.Exit(2)
		}
	}
	// --backup {backup_dir} sets the backup directory for the whole process.
	if *backupDirFlag != "" {
		backupDirOverride = *backupDirFlag
	}
	_, defaultDataDir, _ := paths.GetDefaultDirs(ProjectName)
	backupDataDir = firstNonEmpty(*dataDirFlag, os.Getenv("DATA_DIR"), defaultDataDir)

	// Execute service commands before maintenance and update commands (PART 8).
	if serviceFound {
		if err := handleServiceCommand(serviceAction); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if maintenanceSubcommandFound {
		switch maintenanceAction {
		case "backup":
			var err error
			if maintenanceTarget == "test" || strings.HasPrefix(maintenanceTarget, "test ") {
				file := strings.TrimSpace(strings.TrimPrefix(maintenanceTarget, "test"))
				err = testBackup(file, true)
			} else {
				err = createBackup(maintenanceTarget, true)
			}
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "restore":
			if err := restoreBackup(maintenanceTarget, true); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "update":
			if err := checkAndUpdate(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "setup":
			if err := runMaintenanceSetup(*configDirFlag); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "compliance":
			if maintenanceTarget != "report" {
				fmt.Fprintf(os.Stderr, "Error: --maintenance compliance requires the report action\n")
				os.Exit(1)
			}
			if err := runComplianceReport(*configDirFlag, *logDirFlag); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "mode":
			if maintenanceTarget == "" {
				fmt.Fprintln(os.Stderr, "Error: --maintenance mode requires production, development, or debug")
				os.Exit(2)
			}
			if err := runMaintenanceMode(*configDirFlag, maintenanceTarget); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "security":
			if err := runMaintenanceSecurity(*configDirFlag, maintenanceTarget); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "pgp":
			if err := runMaintenancePGP(*configDirFlag, *logDirFlag, maintenanceTarget); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "secret":
			if err := runMaintenanceSecret(*configDirFlag, *logDirFlag, maintenanceTarget); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "token":
			if err := runMaintenanceToken(*configDirFlag, *logDirFlag, maintenanceTarget); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		case "data":
			if err := runMaintenanceData(*configDirFlag, *logDirFlag, maintenanceTarget); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
		return
	}

	// Handle --update [check|yes|branch {stable|beta|devel}] (PART 22).
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
			fmt.Fprintf(os.Stderr, "Error: invalid --update value %q (expected check, yes, or branch {stable|beta|devel})\n", updateAction)
			os.Exit(1)
		}
		return
	}

	// Handle other non-startup commands.
	if schedulerFound {
		os.Exit(handleSchedulerCLI(schedulerAction, schedulerTaskID))
	}
	if emailFound {
		os.Exit(handleEmailCLI(emailAction, emailAddress, emailDryRun, emailParseErr))
	}
	if torFound {
		os.Exit(handleTorCLI(torAction, torSubAction, torArg))
	}

	// Start server (possibly in maintenance mode)
	i2pFlagSet := false
	flag.Visit(func(current *flag.Flag) {
		if current.Name == "i2p" {
			i2pFlagSet = true
		}
	})
	if err := run(context.Background(), runConfig{
		Port: *portFlag, Address: *addressFlag, ConfigDir: *configDirFlag, DataDir: *dataDirFlag,
		CacheDir: *cacheDirFlag, LogDir: *logDirFlag, PIDFile: *pidFileFlag, BaseURL: *baseURLFlag,
		Mode: *modeFlag, Debug: *debugFlag, Color: *colorFlag, Lang: *langFlag,
		Maintenance: *maintenanceMode, Daemon: *daemonFlag, EnableI2P: *i2pFlag, I2PFlagSet: i2pFlagSet,
		Version: Version, CommitID: CommitID, BuildEpoch: BuildEpoch, OfficialSite: OfficialSite,
	}); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		if errors.Is(err, errRestartForUpdate) {
			os.Exit(75)
		}
		os.Exit(1)
	}
}

// extractUpdateFlag scans argv for --update/-update before flag.Parse() runs,
// since the stdlib flag package has no concept of an optional value (PART 22:
// --update [check|yes|branch {stable|beta|devel}]). It returns whether the
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
			case "check", "yes", "branch", "--help", "-h":
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
		case "--help", "-h":
			action = "help"
		default:
			action = value
		}

		if consumedNext {
			i++
		}
	}

	return found, action, branch, remaining
}

func extractServiceFlag(args []string) (found bool, action string, remaining []string) {
	remaining = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		name, value := arg, ""
		if strings.HasPrefix(arg, "-") {
			trimmed := strings.TrimLeft(arg, "-")
			if idx := strings.IndexByte(trimmed, '='); idx >= 0 {
				name, value = trimmed[:idx], trimmed[idx+1:]
			} else {
				name = trimmed
			}
		}
		if name != "service" {
			remaining = append(remaining, arg)
			continue
		}
		found = true
		if value == "" && i+1 < len(args) {
			next := args[i+1]
			switch next {
			case "start", "stop", "restart", "reload", "--install", "--uninstall", "--disable", "--help", "help":
				value = next
				i++
			}
		}
		if value == "" {
			value = "help"
		}
		if value == "--help" {
			value = "help"
		}
		action = value
	}
	return found, action, remaining
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
func extractEmailFlag(args []string) (found bool, action, address string, dryRun bool, remaining []string, parseErr error) {
	if len(args) == 0 || args[0] != "email" {
		return false, "", "", false, args, nil
	}
	found = true
	if len(args) > 1 {
		action = args[1]
	}
	for index := 2; index < len(args); index++ {
		argument := args[index]
		switch {
		case argument == "--dry-run":
			dryRun = true
		case argument == "--to":
			if index+1 >= len(args) || strings.HasPrefix(args[index+1], "-") {
				parseErr = fmt.Errorf("--to requires an address")
				return found, action, address, dryRun, args[len(args):], parseErr
			}
			index++
			address = args[index]
		case strings.HasPrefix(argument, "--to="):
			address = strings.TrimPrefix(argument, "--to=")
			if address == "" {
				parseErr = fmt.Errorf("--to requires an address")
				return found, action, address, dryRun, args[len(args):], parseErr
			}
		case strings.HasPrefix(argument, "-"):
			parseErr = fmt.Errorf("unknown email option %q", argument)
			return found, action, address, dryRun, args[len(args):], parseErr
		case address == "":
			address = argument
		default:
			parseErr = fmt.Errorf("unexpected email argument %q", argument)
			return found, action, address, dryRun, args[len(args):], parseErr
		}
	}
	return found, action, address, dryRun, args[len(args):], nil
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
// dispatch backup and restore operations. The standalone `--backup` flag only
// selects a backup directory. Any other value falls
// through to the bare-flag (maintenance-mode) behavior so `flag.Bool` still
// parses it normally downstream.
func extractMaintenanceFlag(args []string) (subcommandFound bool, action string, target string, remaining []string) {
	remaining = make([]string, 0, len(args))

	for i := 0; i < len(args); i++ {
		arg := args[i]

		name := arg
		inlineValue := ""
		if strings.HasPrefix(arg, "--") || strings.HasPrefix(arg, "-") {
			trimmed := strings.TrimLeft(arg, "-")
			if idx := strings.Index(trimmed, "="); idx == -1 {
				name = trimmed
			} else {
				name = trimmed[:idx]
				inlineValue = trimmed[idx+1:]
			}
		}

		if name != "maintenance" {
			remaining = append(remaining, arg)
			continue
		}

		next := inlineValue
		if next == "" && i+1 < len(args) {
			next = args[i+1]
		}
		if isMaintenanceAction(next) {
			subcommandFound = true
			if next == "--help" {
				next = "help"
			}
			action = next
			if inlineValue == "" {
				i++
			}
			if action == "backup" || action == "restore" || action == "mode" || action == "security" || action == "pgp" || action == "secret" || action == "token" || action == "data" || action == "compliance" {
				var values []string
				for i+1 < len(args) {
					nextArg := args[i+1]
					if strings.HasPrefix(nextArg, "-") && (action != "security" || !isSecurityMaintenanceOption(nextArg)) && (action != "data" || nextArg != "--owner" && nextArg != "--output" && nextArg != "--yes" && nextArg != "-") {
						break
					}
					i++
					values = append(values, args[i])
					if action == "backup" && values[0] == "test" && len(values) == 1 {
						continue
					}
					if action == "backup" || action == "restore" || action == "mode" || action == "compliance" {
						break
					}
					if action == "token" && (len(values) == 1 && values[0] == "list" || len(values) == 2) {
						break
					}
					if action == "secret" && (len(values) == 2 || len(values) == 1 && values[0] != "rotate") {
						break
					}
					if action == "pgp" && (values[0] == "generate" || values[0] == "rotate" || values[0] == "publish" || values[0] == "delete" || len(values) >= 3) {
						break
					}
					if action == "security" && len(values) == 1 && (values[0] == "list" || values[0] == "show") {
						continue
					}
				}
				separator := " "
				if action == "security" {
					separator = "\x00"
				}
				target = strings.Join(values, separator)
			}
			continue
		}

		remaining = append(remaining, arg)
	}

	return subcommandFound, action, target, remaining
}

func isMaintenanceAction(action string) bool {
	switch action {
	case "backup", "restore", "update", "mode", "security", "setup", "pgp", "secret", "token", "data", "compliance", "help", "--help":
		return true
	default:
		return false
	}
}

// normalizeMaintenanceSubcommands routes nested email and Tor operations
// through their existing argument parsers while retaining the canonical
// --maintenance command spelling.
func normalizeMaintenanceSubcommands(args []string) []string {
	result := append([]string(nil), args...)
	for index := 0; index < len(result); index++ {
		arg := result[index]
		if (arg == "--maintenance" || arg == "-maintenance") && index+1 < len(result) && (result[index+1] == "email" || result[index+1] == "tor") {
			return append(result[:index], result[index+1:]...)
		}
		for _, prefix := range []string{"--maintenance=", "-maintenance="} {
			if strings.HasPrefix(arg, prefix) {
				value := strings.TrimPrefix(arg, prefix)
				if value == "email" || value == "tor" {
					result[index] = value
					return result
				}
			}
		}
	}
	return result
}

func isSecurityMaintenanceOption(arg string) bool {
	name := strings.TrimLeft(arg, "-")
	if index := strings.IndexByte(name, '='); index >= 0 {
		name = name[:index]
	}
	switch name {
	case "status", "comment", "disclosure-date", "severity":
		return true
	default:
		return false
	}
}

// handleSchedulerCLI implements `{project_name} scheduler list|show|run|enable|disable|history`
// per AI.md PART 18 "CLI Commands". It opens server.db directly, registers
// the same 11 built-in tasks (without starting the ticker), and operates on
// persisted state — safe to run alongside an already-running server since
// all reads/writes go through server.db.
func handleSchedulerCLI(action, taskID string) int {
	applyTZEnv()

	defaultConfigDir, defaultDataDir, defaultLogsDir := paths.GetDefaultDirs(ProjectName)
	configDir := firstNonEmpty(os.Getenv("CONFIG_DIR"), defaultConfigDir)
	dataDir := firstNonEmpty(os.Getenv("DATA_DIR"), defaultDataDir)
	logsDir := firstNonEmpty(os.Getenv("LOG_DIR"), defaultLogsDir)
	configPath := filepath.Join(configDir, "server.yml")

	cfg, err := config.LoadApplicationConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load configuration: %v\n", err)
		return 1
	}
	db.SetQueryLoggingEnabled(cfg.Server.Debug.LogQueries)
	cfg.Server.FQDN = resolveFQDN()

	var dbHandle db.Handle
	dbHandle, err = db.OpenStore(cfg.Server.Database, ProjectName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to open database: %v\n", err)
		return 1
	}
	scheduler.SetQueryObserver(db.ObserveQuery)
	defer func() { _ = dbHandle.Close() }()

	if err := scheduler.EnsureSchema(dbHandle); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to initialize scheduler schema: %v\n", err)
		return 1
	}

	catchUpWindow, parseErr := time.ParseDuration(cfg.Server.Scheduler.CatchUpWindow)
	if parseErr != nil || catchUpWindow <= 0 {
		catchUpWindow = time.Hour
	}
	sched := scheduler.NewSchedulerWithLocation(dbHandle, catchUpWindow, resolveSchedulerLocation(cfg.Server.Scheduler.Timezone))
	blockStore := server.NewBlockStore(cfg.Server.Security.BlockedIPs)
	portInt, _ := strconv.Atoi(cfg.Server.Port)
	torMgr := tor.NewTorManager(ProjectName, portInt, cfg.Server.Tor)
	deps := buildSchedulerDeps(cfg, configDir, dataDir, logsDir, false, dbHandle, nil, blockStore, cfg.Server.Port, torMgr, nil)
	defer func() { _ = deps.logMgr.Close() }()
	sched.SetLogger(func(level, format string, args ...any) {
		if err := deps.logMgr.Server().Write(fmt.Sprintf("%s %s", level, fmt.Sprintf(format, args...))); err != nil {
			_ = deps.logMgr.Error().Write(fmt.Sprintf("ERROR could not write scheduler log event: %v", err))
		}
	})
	registerSchedulerTasks(sched, cfg, deps)

	switch action {
	case "", "list":
		printSchedulerList(sched)
	case "show":
		if taskID == "" {
			fmt.Fprintln(os.Stderr, "Error: scheduler show requires a task id")
			return 1
		}
		st, ok := sched.GetScheduledTaskState(taskID)
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
func notifyRecipient(cfg *config.ApplicationConfig) string {
	return firstNonEmpty(
		cfg.Server.Notifications.Email.ReplyTo,
		cfg.Server.Notifications.Email.From.Email,
		cfg.Web.Security.Admin)
}

// handleEmailCLI implements PART 17 email test and dry-run commands.
func handleEmailCLI(action, address string, dryRun bool, parseErr error) int {
	if parseErr != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", parseErr)
		return 1
	}
	if action != "test" {
		fmt.Fprintf(os.Stderr, "Error: unknown email command %q (expected test)\n", action)
		return 1
	}

	defaultConfigDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configDir := firstNonEmpty(os.Getenv("CONFIG_DIR"), defaultConfigDir)
	configPath := filepath.Join(configDir, "server.yml")

	cfg, err := config.LoadApplicationConfig(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load configuration: %v\n", err)
		return 1
	}

	if dryRun {
		previews, err := notify.PreviewEmailTemplates(configDir, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to preview email templates: %v\n", err)
			return 1
		}
		for index, preview := range previews {
			if index > 0 {
				fmt.Println()
			}
			fmt.Printf("Template: %s\nSubject: %s\n\n%s", preview.Name, preview.Subject, preview.Body)
			for _, warning := range preview.Warnings {
				fmt.Fprintf(os.Stderr, "Warning: template %s: %s\n", preview.Name, warning)
			}
		}
		return 0
	}

	recipient := firstNonEmpty(address, cfg.Server.Notifications.Email.ReplyTo)
	if recipient == "" {
		fmt.Fprintln(os.Stderr, "Error: no recipient address given and server.notifications.email.reply_to is empty")
		return 1
	}

	if !notify.CanSend(cfg) {
		fmt.Fprintln(os.Stderr, "Error: email notifications are disabled — no SMTP host configured or notifications not enabled")
		return 1
	}

	subjectTemplate, _, err := notify.LoadEmailTemplate(configDir, "test")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to load test email template: %v\n", err)
		return 1
	}
	variables := notify.GlobalVars(cfg)
	subjectTemplate, _ = notify.TranslateEmailTemplate(configDir, "test", subjectTemplate, "", cfg.Server.I18n.DefaultLanguage, variables)
	subject, _ := notify.Render(subjectTemplate, "", variables)
	subject = notify.TestEmailSubject(subject)
	if err := notify.Send(cfg, configDir, "test", recipient, nil); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to send test email: %v\n", err)
		return 1
	}
	_, _, logsDir := paths.GetDefaultDirs(ProjectName)
	if err := writeDataAudit(logsDir, toLoggingConfig(cfg.Server.Logging), "notification.email_test_sent", "operator", "template=test"); err != nil {
		log.Printf("Warning: could not audit test email delivery: %v", err)
	}
	fmt.Printf("Test email sent to %s\nTemplate: test\nSubject: %s\n", recipient, subject)
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
	cfg, err := config.LoadApplicationConfig(filepath.Join(configDir, "server.yml"))
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

// printTorStatusData prints a decoded tor.TorRuntimeStatus payload in the same
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

func run(ctx context.Context, options runConfig) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if options.Listener != nil {
		defer func() {
			if err := options.Listener.Close(); err != nil {
				log.Printf("Warning: could not close retained listener: %v", err)
			}
		}()
	}
	portFlag, addressFlag, configDirFlag, dataDirFlag := options.Port, options.Address, options.ConfigDir, options.DataDir
	cacheDirFlag, logDirFlag, pidFileFlag, baseURLFlag, modeFlag := options.CacheDir, options.LogDir, options.PIDFile, options.BaseURL, options.Mode
	debug, color, lang, maintenance := options.Debug, options.Color, options.Lang, options.Maintenance
	daemon, enableI2P, i2PFlagSet := options.Daemon, options.EnableI2P, options.I2PFlagSet
	version := firstNonEmpty(options.Version, Version)
	commitID := firstNonEmpty(options.CommitID, CommitID)
	buildEpoch := firstNonEmpty(options.BuildEpoch, BuildEpoch)
	buildDate := commonversion.BuildDateFromEpoch(buildEpoch, BuildDate)
	officialSite := firstNonEmpty(options.OfficialSite, OfficialSite)
	recentLogBuffer := logging.NewRecentLogBuffer(10_000)
	previousSlogLogger := slog.Default()
	defer slog.SetDefault(previousSlogLogger)
	previousLogOutput := log.Writer()
	log.SetOutput(logging.NewSynchronizedWriter(io.MultiWriter(previousLogOutput, recentLogBuffer)))
	defer log.SetOutput(previousLogOutput)

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
	server.Version = version
	server.CommitID = commitID
	server.BuildDate = buildDate

	// Handle color flag (auto, yes, no); resolveColorSetting also stores the
	// validated value into colorSetting for getEmoji to consult.
	if _, err := resolveColorSetting(color); err != nil {
		return err
	}

	if maintenance {
		log.Printf("Starting %s API server v%s in MAINTENANCE mode (read-only)", ProjectName, version)
	} else {
		log.Printf("Starting %s API server v%s", ProjectName, version)
	}
	log.Printf("Commit: %s, Built: %s", commitID, buildDate)
	if officialSite != "" {
		log.Printf("Official site: %s", officialSite)
	}

	// Get OS-specific default directories
	defaultConfigDir, defaultDataDir, defaultLogsDir := paths.GetDefaultDirs(ProjectName)

	// Allow overrides via flags or environment variables
	configDir := firstNonEmpty(configDirFlag, os.Getenv("CONFIG_DIR"), defaultConfigDir)
	dataDir := firstNonEmpty(dataDirFlag, os.Getenv("DATA_DIR"), defaultDataDir)
	backupDataDir = dataDir
	resolvedBackupDir = ""
	resolvedBackupDir = paths.GetBackupDir(backupDirOverride, dataDir)
	logsDir := firstNonEmpty(logDirFlag, os.Getenv("LOG_DIR"), defaultLogsDir)
	cacheDir := firstNonEmpty(cacheDirFlag, os.Getenv("CACHE_DIR"), paths.GetCacheDir(ProjectName))
	dbDir := firstNonEmpty(os.Getenv("DATABASE_DIR"), paths.GetSQLiteDBPath(ProjectName))
	pidPath := firstNonEmpty(pidFileFlag, os.Getenv("PID_FILE"), paths.GetPIDFile(ProjectName))
	for _, override := range []struct {
		name  string
		value *string
	}{
		{name: "config", value: &configDir},
		{name: "data", value: &dataDir},
		{name: "backup", value: &resolvedBackupDir},
		{name: "log", value: &logsDir},
		{name: "cache", value: &cacheDir},
		{name: "pid", value: &pidPath},
	} {
		safe, err := paths.SafeDirPath(*override.value)
		if err != nil {
			return fmt.Errorf("invalid %s path: %w", override.name, err)
		}
		*override.value = safe
	}
	baseURL := ""
	inContainer := paths.IsRunningInContainer()
	paths.SetBinaryBaseName(ProjectName)
	startedElevated := service.IsElevated()
	stagedUpdateFailure := applyStagedUpdate(dataDir, startedElevated)

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

	// Reject configured symlink roots before generic directory setup can follow
	// them, then create the top-level runtime directories.
	if err := paths.ValidatePrivateRoots(configDir, dataDir); err != nil {
		return err
	}
	// Ensure directories exist
	if err := paths.EnsureDirs(configDir, dataDir, logsDir); err != nil {
		return fmt.Errorf("failed to create directories: %w", err)
	}
	// Repair every private path before reading server.yml or starting any child.
	if err := paths.EnsurePrivatePaths(configDir, dataDir); err != nil {
		return err
	}
	if err := paths.EnsureDir(cacheDir); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}
	if err := paths.EnsureDir(dbDir); err != nil {
		return fmt.Errorf("failed to create database directory: %w", err)
	}
	if err := paths.EnsureDir(resolvedBackupDir); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}
	if err := paths.EnsureDir(filepath.Dir(pidPath)); err != nil {
		return fmt.Errorf("failed to create PID directory: %w", err)
	}

	log.Printf("Config directory: %s", configDir)
	log.Printf("Data directory: %s", dataDir)
	log.Printf("Logs directory: %s", logsDir)
	log.Printf("Cache directory: %s", cacheDir)

	// Reserve the process ID before configuration loading, database startup,
	// scheduler startup, or overlay-network startup (AI.md PART 8 steps 11-12).
	pidCreated, err := writeServerPID(pidPath, inContainer, startedElevated)
	if err != nil {
		return err
	}
	if pidCreated {
		defer func() {
			if rmErr := os.Remove(pidPath); rmErr != nil {
				log.Printf("Warning: could not remove PID file %s: %v", pidPath, rmErr)
			}
		}()
	}

	// Load configuration from YAML file (.yml per BASE.md)
	configPath := filepath.Join(configDir, "server.yml")
	log.Printf("Loading configuration from: %s", configPath)

	cfg, err := config.LoadApplicationConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	db.SetQueryLoggingEnabled(cfg.Server.Debug.LogQueries)
	if i2PFlagSet {
		cfg.Server.I2P.Enabled = enableI2P
	}
	outputEmojiSetting = cfg.Output.Emoji
	lang = i18n.ResolveOutputLanguage(lang, cfg.Server.Lang)
	log.Println("Configuration loaded successfully")
	if baseURLFlag != "" {
		var valid bool
		baseURL, valid = config.NormalizeBaseURL(baseURLFlag)
		if !valid {
			return fmt.Errorf("invalid --baseurl path %q", baseURLFlag)
		}
	} else {
		baseURL = cfg.Server.BaseURL
	}
	cfg.Server.BaseURL = baseURL

	// Resolve the static FQDN using AI.md PART 8's fallback order. Per-request
	// reverse-proxy headers are handled separately by server.publicBaseURL.
	cfg.Server.FQDN = resolveFQDNWithMode(isDevelopmentMode(modeFlag))
	log.Printf("FQDN: %s", cfg.Server.FQDN)

	refreshSMTPStatus(cfg, true)
	log.Printf("email.configured=%t", notify.CanSend(cfg))

	reportStagedUpdate(dataDir, logsDir, configDir, cfg, stagedUpdateFailure)

	// Resolve application mode per AI.md PART 6 priority: --mode flag, MODE
	// environment variable, then production. Mode is runtime-only and does not
	// persist into server.yml.
	if err := mode.Initialize(modeFlag); err != nil {
		log.Printf("Warning: invalid mode %q: %v", firstNonEmpty(modeFlag, os.Getenv("MODE")), err)
	}
	cfg.Server.Mode = mode.GetAppMode().String()

	// Resolve debug flag per AI.md PART 6 priority: --debug flag > DEBUG env
	// var > --mode debug/MODE=debug alias (already applied by Initialize
	// above) > default false.
	mode.InitializeDebug(debug)
	if mode.IsDebugEnabled() {
		log.Println("Debug mode enabled")
	}
	startupLogManager := logging.NewLogManager(logsDir, toLoggingConfig(cfg.Server.Logging))
	logLevel := cfg.Server.Logging.Level
	if strings.TrimSpace(logLevel) == "" {
		logLevel = mode.GetLogLevel()
	}
	slog.SetDefault(slog.New(logging.NewSlogHandler(startupLogManager, logging.ParseSlogLevel(logLevel))))
	defer func() {
		if closeErr := startupLogManager.Close(); closeErr != nil {
			log.Printf("Warning: error closing log files: %v", closeErr)
		}
	}()

	// Determine port: Flag > ENV > Config > (container default 80) > Random.
	// Per AI.md PART 4 "Container Port Behavior": the container's internal
	// default is always 0.0.0.0:80 (Docker maps a random 64xxx host port to
	// it) — containers never fall through to the host/local random-64xxx
	// port-selection behavior.
	port := firstNonEmpty(portFlag, os.Getenv("PORT"), cfg.Server.Port)
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
		if err := config.SetConfiguredServerPort(port); err != nil {
			log.Printf("Warning: Failed to save port to config: %v", err)
		}
	}

	// Determine listen address
	address := firstNonEmpty(addressFlag, os.Getenv("LISTEN"), cfg.Server.Address, "0.0.0.0")
	portBindings, err := parsePortSpec(port, cfg.Server.SSL.Enabled)
	if err != nil {
		return fmt.Errorf("invalid port configuration: %w", err)
	}
	port = portBindings[0].Port

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
	listeners := make([]net.Listener, len(portBindings))
	defer func() {
		for _, listener := range listeners {
			if listener != nil {
				_ = listener.Close()
			}
		}
	}()
	if options.Listener != nil {
		if len(portBindings) != 1 {
			return errors.New("an injected listener requires exactly one configured server port")
		}
		address, port, err = net.SplitHostPort(options.Listener.Addr().String())
		if err != nil {
			return fmt.Errorf("invalid injected listener address: %w", err)
		}
		portBindings[0].Port = port
		listeners[0] = options.Listener
	}
	if runtime.GOOS != "windows" && service.IsElevated() {
		for i, binding := range portBindings {
			if listeners[i] != nil {
				continue
			}
			listeners[i], err = net.Listen("tcp", joinListenAddress(address, binding.Port))
			if err != nil {
				for _, listener := range listeners {
					if listener != nil {
						_ = listener.Close()
					}
				}
				return fmt.Errorf("failed to bind %s:%s: %w", address, binding.Port, err)
			}
		}

		uid, gid, idErr := service.ServiceUserIDs()
		if idErr != nil {
			return fmt.Errorf("failed to resolve service user for privilege drop: %w", idErr)
		}
		for _, dir := range []string{configDir, dataDir, logsDir, cacheDir, dbDir, resolvedBackupDir, filepath.Dir(pidPath)} {
			if chownErr := chownPathTree(dir, uid, gid); chownErr != nil {
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
	airportSvc, err := airports.NewAirportDataService(airportdata.AirportsJSON)
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
	geoipSvc, err := geoip.NewGeoIPService(filepath.Join(dataDir, "security"))
	if err != nil {
		log.Printf("Warning: GeoIP initialization failed: %v", err)
		log.Println("GeoIP features will be unavailable")
		geoipSvc = nil
	} else {
		defer func() {
			if closeErr := geoipSvc.Close(); closeErr != nil {
				log.Printf("Warning: failed to close GeoIP databases: %v", closeErr)
			}
		}()
		log.Println("GeoIP databases loaded successfully")
	}

	// Open server.db and initialize the scheduler's persistent state per
	// AI.md PART 18 "Persistent State" — task status must survive restarts.
	var startupHTTPServers []*http.Server
	var startupSwitchers []*startupRouteSwitcher
	var dbHandle db.Handle
	store, openErr := db.OpenStore(cfg.Server.Database, ProjectName)
	if openErr != nil {
		err = openErr
	} else {
		dbHandle = store
		err = initializeStartupDatabase(dbHandle, cfg, configDir)
	}
	if err != nil {
		if dbHandle != nil {
			_ = dbHandle.Close()
		}
		dbHandle, startupHTTPServers, startupSwitchers, err = waitForStartupDatabaseRecovery(ctx, cfg, configDir, dataDir, address, portBindings, listeners, airportSvc, geoipSvc, startupLogManager, err)
		if err != nil {
			return fmt.Errorf("failed to recover database startup: %w", err)
		}
	}
	scheduler.SetQueryObserver(db.ObserveQuery)
	defer func() {
		if closeErr := dbHandle.Close(); closeErr != nil {
			log.Printf("Warning: error closing database: %v", closeErr)
		}
	}()

	catchUpWindow, parseErr := time.ParseDuration(cfg.Server.Scheduler.CatchUpWindow)
	if parseErr != nil || catchUpWindow <= 0 {
		catchUpWindow = time.Hour
	}
	sched := scheduler.NewSchedulerWithLocation(dbHandle, catchUpWindow, resolveSchedulerLocation(cfg.Server.Scheduler.Timezone))
	sched.SetLogger(func(level, format string, args ...any) {
		line := fmt.Sprintf("%s %s", level, fmt.Sprintf(format, args...))
		if err := startupLogManager.Server().Write(line); err != nil {
			_ = startupLogManager.Error().Write(fmt.Sprintf("ERROR could not write scheduler log event: %v", err))
		}
	})

	// Lifecycle states are reported separately from the configured application
	// mode. The mode remains production/development/debug during maintenance.
	server.SetHealthRuntimeState(server.HealthRuntimeState{})
	if maintenance {
		server.SetHealthMaintenance(&server.MaintenanceHealthInfo{
			Reason: "operator_requested",
			SelfHealing: server.SelfHealingHealthStatus{
				Enabled: false,
			},
		})
	}
	// Construct the Tor manager once, before the server, so the exact same
	// *tor.TorManager instance can be shared between server.NewServer() (which
	// reports live Tor state via /server/healthz, AI.md PART 13) and the
	// scheduler deps (which start/stop the process, AI.md PART 31).
	portInt, _ := strconv.Atoi(port)
	torMgr := tor.NewTorManager(ProjectName, portInt, cfg.Server.Tor)
	i2pMgr := i2p.NewI2PManager(cfg.Server.I2P, i2p.I2PDirs{ConfigDir: configDir, DataDir: dataDir, LogDir: logsDir})
	if err := server.InitializeOwnerTokenSchema(dbHandle); err != nil {
		return fmt.Errorf("initialize owner identity storage: %w", err)
	}

	// Create HTTP server
	srv := server.NewServer(airportSvc, geoipSvc, cfg, sched, dbHandle, dataDir, configDir, torMgr, recentLogBuffer)
	srv.SetI2PManager(i2pMgr)
	defer func() {
		if closeErr := i2pMgr.StopI2PService(); closeErr != nil {
			log.Printf("Warning: error stopping I2P eepsite: %v", closeErr)
		}
	}()
	defer func() {
		if closeErr := srv.CloseCache(); closeErr != nil {
			log.Printf("Warning: error closing cache backend: %v", closeErr)
		}
	}()

	// registerSchedulerTasks needs srv.BlockStore() so the scheduled
	// blocklist_update task shares the exact in-memory store the live HTTP
	// middleware enforces against — no restart required for updates to
	// take effect.
	schedDeps := buildSchedulerDeps(cfg, configDir, dataDir, logsDir, startedElevated, dbHandle, geoipSvc, srv.BlockStore(), port, torMgr, startupLogManager)
	srv.SetAccessLog(schedDeps.logMgr.Access())
	srv.SetErrorLog(schedDeps.logMgr.Error())
	srv.SetAuditLogger(schedDeps.logMgr)
	for _, key := range cfg.GeneratedSecretPaths() {
		if err := schedDeps.logMgr.WriteAudit(logging.AuditCategoryConfiguration, "event=config.secret_generated key="+strconv.Quote(key)); err != nil {
			log.Printf("Warning: could not write generated-secret audit event: %v", err)
		}
	}
	schedDeps.i2pMgr = i2pMgr
	srv.SetSecurityLog(schedDeps.logMgr.Security())
	if triggered, cleanupErr := runDiskPressureCleanup(cfg.Server.Maintenance.Cleanup, dataDir, resolvedBackupDir, schedDeps.logMgr, time.Now(), backup.DiskUsage); cleanupErr != nil {
		log.Printf("Warning: maintenance disk-pressure cleanup failed: %v", cleanupErr)
	} else if triggered {
		log.Printf("Maintenance disk-pressure cleanup completed")
	}
	registerSchedulerTasks(sched, cfg, schedDeps)
	maintenanceRecovery := startMaintenanceRecovery(cfg, dbHandle, dataDir, configDir, logsDir, resolvedBackupDir, schedDeps.logMgr, maintenance)
	defer maintenanceRecovery.stop()

	var tlsConfig *tls.Config
	for _, binding := range portBindings {
		if !binding.TLS {
			continue
		}
		if schedDeps.sslMgr == nil {
			return fmt.Errorf("HTTPS port %s requires SSL configuration", binding.Port)
		}
		tlsConfig, err = schedDeps.sslMgr.GetTLSConfig([]string{cfg.Server.FQDN})
		if err != nil {
			return fmt.Errorf("failed to configure HTTPS on port %s: %w", binding.Port, err)
		}
		if tlsConfig == nil {
			return fmt.Errorf("HTTPS port %s requires SSL to be enabled", binding.Port)
		}
	}

	// Start scheduler
	sched.StartScheduler()
	defer sched.StopScheduler()
	if err := startupLogManager.Server().Write(fmt.Sprintf("INFO scheduler started; registered tasks: %s", strings.Join(sched.RegisteredTaskIDs(), ", "))); err != nil {
		_ = startupLogManager.Error().Write(fmt.Sprintf("ERROR could not write scheduler startup event: %v", err))
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
	var torHTTPServer *http.Server
	var torStartDone chan struct{}
	var torStartCancel context.CancelFunc
	if cfg.Server.I2P.Enabled {
		if startErr := i2pMgr.StartI2PService(context.Background(), rootHandler); startErr != nil {
			log.Printf("Warning: I2P eepsite failed to start: %v", startErr)
		} else {
			srv.SetI2PManager(i2pMgr)
		}
	}

	httpServers := make([]*http.Server, len(portBindings))
	for i, binding := range portBindings {
		handler := rootHandler
		if !binding.TLS && httpsPortFromBindings(portBindings) != "" {
			if httpsPort := httpsPortFromBindings(portBindings); httpsPort != "" {
				handler = redirectHTTPToHTTPS(handler, cfg.Server.FQDN, httpsPort, srv.IsTrustedProxyRequest)
			}
		}
		if !binding.TLS && schedDeps.sslMgr != nil {
			handler = schedDeps.sslMgr.GetHTTPHandler(handler)
		}
		serverConfig := &http.Server{
			Addr:         joinListenAddress(address, binding.Port),
			Handler:      handler,
			ReadTimeout:  cfg.Server.Limits.ReadTimeoutDuration(),
			WriteTimeout: cfg.Server.Limits.WriteTimeoutDuration(),
			IdleTimeout:  cfg.Server.Limits.IdleTimeoutDuration(),
		}
		if binding.TLS && tlsConfig != nil {
			serverConfig.TLSConfig = tlsConfig.Clone()
		}
		if i < len(startupSwitchers) {
			startupSwitchers[i].Set(handler)
			httpServers[i] = startupHTTPServers[i]
		} else {
			httpServers[i] = serverConfig
		}
	}
	if len(startupSwitchers) > 0 {
		server.SetHealthMaintenance(nil)
	}
	for i, binding := range portBindings {
		if listeners[i] != nil {
			continue
		}
		listeners[i], err = net.Listen("tcp", joinListenAddress(address, binding.Port))
		if err != nil {
			for _, listener := range listeners {
				if listener != nil {
					_ = listener.Close()
				}
			}
			return fmt.Errorf("failed to bind %s:%s: %w", address, binding.Port, err)
		}
	}

	// Start one listener for each configured HTTP/HTTPS port.
	urls := make([]string, 0, len(portBindings))
	for _, binding := range portBindings {
		urls = append(urls, getServerAccessibleURL(binding.Port, cfg.Server.FQDN, binding.TLS))
	}
	for i, listener := range listeners {
		if i < len(startupSwitchers) {
			continue
		}
		serverConfig := httpServers[i]
		binding := portBindings[i]
		ready := make(chan struct{})
		readyListener := &acceptReadyListener{Listener: listener, ready: ready}
		go func(srv *http.Server, ln net.Listener, useTLS bool) {
			if serveErr := serveHTTPServer(context.Background(), srv, ln, useTLS, nil); serveErr != nil {
				fmt.Fprintf(os.Stderr, "Server error: %v\n", serveErr)
				os.Exit(1)
			}
		}(serverConfig, readyListener, binding.TLS)
		select {
		case <-ready:
		case <-time.After(5 * time.Second):
			return fmt.Errorf("HTTP listener %s did not begin accepting connections", serverConfig.Addr)
		}
	}
	forceColor := colorSetting == "yes"
	var forceColorSetting *bool
	if colorSetting != "auto" {
		forceColorSetting = &forceColor
	} else {
		forceColorSetting = cfg.Output.Color
	}
	var setupToken string
	for _, generated := range cfg.GeneratedSecretPaths() {
		if generated == "server.token" {
			setupToken = cfg.Server.Token
			break
		}
	}
	var onionAddress, i2pAddress, smtpAddress string
	if schedDeps.torMgr != nil && schedDeps.torMgr.IsTorEnabled() {
		onionAddress = waitForStartupAddress(schedDeps.torMgr.OnionAddress)
	}
	if cfg.Server.I2P.Enabled && cfg.Server.I2P.Expose && i2pMgr != nil && i2pMgr.IsI2PRunning() {
		i2pAddress = waitForStartupAddress(i2pMgr.EepsiteAddress)
	}
	if cfg.Server.Notifications.Email.Enabled && cfg.Server.Notifications.Email.SMTP.Host != "" {
		smtpAddress = net.JoinHostPort(cfg.Server.Notifications.Email.SMTP.Host, strconv.Itoa(cfg.Server.Notifications.Email.SMTP.Port))
	}
	appbanner.PrintServerStartupBanner(appbanner.ServerBannerInfo{
		AppName: "airports", Version: Version, AppMode: mode.GetAppMode().String(),
		StartupDatetime: time.Now().Format("January 02, 2006 at 15:04:05 MST"), URLs: urls,
		OnionAddress: onionAddress, I2PAddress: i2pAddress, SMTPAddress: smtpAddress,
		SetupToken: setupToken, ForceColor: forceColorSetting, Lang: lang,
	})
	for _, listenURL := range urls {
		log.Printf("Server listening on %s", listenURL)
	}
	log.Printf("Server ready on %s", strings.Join(urls, ", "))
	if options.Ready != nil {
		close(options.Ready)
	}
	if err := notify.Send(cfg, configDir, "startup", notifyRecipient(cfg), nil); err != nil {
		log.Printf("Warning: failed to send startup notification email: %v", err)
	}
	// Tor bootstrap is independent of clearnet startup. A missing Tor network
	// must never hold the public HTTP listener behind its bootstrap timeout.
	if schedDeps.torMgr != nil && schedDeps.torMgr.IsTorEnabled() {
		torStartDone = make(chan struct{})
		torStartContext, cancelTorStart := context.WithCancel(context.Background())
		torStartCancel = cancelTorStart
		go func() {
			defer close(torStartDone)
			bootstrapTimeout := time.Duration(cfg.Server.Tor.BootstrapTimeout) * time.Second
			if bootstrapTimeout <= 0 {
				bootstrapTimeout = 180 * time.Second
			}
			torContext, cancel := context.WithTimeout(torStartContext, bootstrapTimeout)
			defer cancel()
			if startErr := schedDeps.torMgr.StartTorService(torContext); startErr != nil {
				log.Printf("Warning: Tor hidden service failed to start: %v", startErr)
				return
			}
			log.Printf("Tor hidden service started: %s", schedDeps.torMgr.OnionAddress())
			torListener := schedDeps.torMgr.BackendListener()
			if torListener == nil {
				return
			}
			torHTTPServer = &http.Server{
				Handler:      rootHandler,
				ConnContext:  srv.TorConnContext,
				ReadTimeout:  cfg.Server.Limits.ReadTimeoutDuration(),
				WriteTimeout: cfg.Server.Limits.WriteTimeoutDuration(),
				IdleTimeout:  cfg.Server.Limits.IdleTimeoutDuration(),
			}
			go func(backendServer *http.Server, listener net.Listener) {
				if serveErr := backendServer.Serve(listener); serveErr != nil && serveErr != http.ErrServerClosed {
					log.Printf("Tor backend server error: %v", serveErr)
				}
			}(torHTTPServer, torListener)
			log.Printf("Tor backend listening on 127.0.0.1:%d", torListener.Addr().(*net.TCPAddr).Port)
		}()
	}

	// Start automatic config file watcher — polls server.yml for external
	// edits and hot-reloads without requiring SIGHUP, per AI.md PART 5
	// "Live Reload". Tor settings restart only the Tor child process.
	configWatchDone := make(chan struct{})
	defer close(configWatchDone)
	go watchConfigFile(configPath, cfg, srv, schedDeps.torMgr, configWatchDone, maintenanceRecovery.updateConfig)

	// Wait for shutdown, log-reopen, or status-dump signals. Configuration
	// reloads are owned by the file watcher; SIGHUP is ignored per AI.md PART 8.
	quit := make(chan os.Signal, 1)
	reopen := make(chan os.Signal, 1)
	statusDump := make(chan os.Signal, 1)
	ossignal.Notify(quit, appsignal.ShutdownSignals()...)
	appsignal.IgnoreConfigReloadSignal()
	if sigs := appsignal.LogReopenSignals(); len(sigs) > 0 {
		ossignal.Notify(reopen, sigs...)
	}
	if sigs := appsignal.StatusDumpSignals(); len(sigs) > 0 {
		ossignal.Notify(statusDump, sigs...)
	}

	updateRestart := false
	for {
		select {
		case <-reopen:
			// SIGUSR1: external log-rotation tools have moved active files;
			// close and reopen every managed writer immediately (AI.md PART 8).
			if err := schedDeps.logMgr.Reopen(); err != nil {
				log.Printf("SIGUSR1 received — one or more log files could not be reopened: %v", err)
			} else {
				log.Println("SIGUSR1 received — log files reopened")
			}
		case <-statusDump:
			// SIGUSR2: dump a concise runtime status snapshot to the log
			// (AI.md PART 8).
			var mem runtime.MemStats
			runtime.ReadMemStats(&mem)
			log.Printf("SIGUSR2 status dump — version=%s commit=%s goroutines=%d heap_alloc=%dKB",
				Version, CommitID, runtime.NumGoroutine(), mem.HeapAlloc/1024)
		case <-quit:
			goto shutdown
		case <-ctx.Done():
			goto shutdown
		case <-updateRestartRequest:
			updateRestart = true
			goto shutdown
		}
	}
shutdown:

	log.Println("Shutting down server...")
	server.SetHealthShuttingDown(true)
	if torStartCancel != nil {
		torStartCancel()
	}
	if torStartDone != nil {
		<-torStartDone
	}

	reason := "signal"
	if updateRestart {
		reason = "update restart"
	}
	if err := notify.Send(cfg, configDir, "shutdown", notifyRecipient(cfg), map[string]string{"reason": reason}); err != nil {
		log.Printf("Warning: failed to send shutdown notification email: %v", err)
	}

	// Graceful shutdown with 30 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for _, httpServer := range httpServers {
		if err := httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("server shutdown error: %w", err)
		}
	}
	if torHTTPServer != nil {
		if err := torHTTPServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("tor backend shutdown error: %w", err)
		}
	}
	if schedDeps.torMgr != nil && schedDeps.torMgr.IsTorEnabled() {
		if stopErr := schedDeps.torMgr.StopTorService(); stopErr != nil {
			log.Printf("Warning: error stopping Tor hidden service: %v", stopErr)
		}
		if torListener := schedDeps.torMgr.BackendListener(); torListener != nil {
			if closeErr := torListener.Close(); closeErr != nil && closeErr != net.ErrClosed {
				log.Printf("Warning: error closing Tor backend listener: %v", closeErr)
			}
		}
	}

	log.Println("Server stopped")
	if updateRestart {
		return errRestartForUpdate
	}
	return nil
}

func chownPathTree(root string, uid, gid int) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if err := os.Lchown(path, uid, gid); err != nil {
			return err
		}
		return nil
	})
}

// watchConfigFile polls the config file, drop-ins, secrets and assets every 5
// seconds and hot-reloads the running server, per AI.md PART 5 "Live
// ReloadApplicationConfig": config files are the source of truth and changes must apply
// without a restart. Listener, TLS-listener, database and daemon settings are
// reported for the next process start; Tor settings restart only the Tor child.
func watchConfigFile(configPath string, initial *config.ApplicationConfig, srv *server.Server, torMgr *tor.TorManager, done <-chan struct{}, onReload func(*config.ApplicationConfig)) {
	lastStamp := configWatchStamp(configPath)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	current := initial
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			stamp := configWatchStamp(configPath)
			if stamp == lastStamp {
				continue
			}
			lastStamp = stamp

			newCfg, loadErr := config.LoadApplicationConfig(configPath)
			if loadErr != nil {
				log.Printf("Config file watcher: reload failed: %v — keeping current config", loadErr)
				continue
			}

			warnRestartRequiredChanges(current, newCfg)
			if torMgr != nil && current.Server.Tor != newCfg.Server.Tor {
				go func(torConfig tor.TorConfig) {
					timeout := time.Duration(torConfig.BootstrapTimeout) * time.Second
					if timeout <= 0 {
						timeout = 180 * time.Second
					}
					ctx, cancel := context.WithTimeout(context.Background(), timeout)
					defer cancel()
					if err := torMgr.ReloadConfig(ctx, torConfig); err != nil {
						log.Printf("Warning: Tor configuration reload failed: %v", err)
					} else {
						log.Printf("Tor configuration reloaded")
					}
				}(newCfg.Server.Tor)
			}
			refreshSMTPStatus(newCfg, false)
			db.SetQueryLoggingEnabled(newCfg.Server.Debug.LogQueries)
			srv.ReloadConfig(newCfg)
			if onReload != nil {
				onReload(newCfg)
			}
			server.SetPendingRestartSettings(restartRequiredSettings(initial, newCfg))
			current = newCfg
			log.Println("Configuration hot-reloaded from file change")
		}
	}
}

func refreshSMTPStatus(cfg *config.ApplicationConfig, persistAutodetection bool) {
	smtp := &cfg.Server.Notifications.Email.SMTP
	if smtp.Host == "" {
		if host, port, ok := notify.Autodetect(cfg.Server.FQDN); ok {
			smtp.Host = host
			smtp.Port = port
			log.Printf("SMTP auto-detected: %s:%d", host, port)
			if persistAutodetection {
				if err := config.SaveCurrentApplicationConfig(); err != nil {
					log.Printf("Warning: failed to persist auto-detected SMTP settings: %v", err)
				}
			}
		} else {
			cfg.Server.Notifications.Email.Enabled = false
			log.Println("SMTP auto-detection found no server — email notifications disabled")
			return
		}
	}
	if notify.IsPrivateSMTPHost(smtp.Host) {
		log.Printf("SMTP host %s is a private address: TLS certificate verification skipped", smtp.Host)
	}
	if err := notify.TestConnection(smtp.Host, smtp.Port, smtp.Username, smtp.Password, smtp.TLS); err != nil {
		cfg.Server.Notifications.Email.Enabled = false
		log.Printf("Warning: configured SMTP connection test failed (%s:%d): %v — email disabled until the next configuration check", smtp.Host, smtp.Port, err)
		return
	}
	cfg.Server.Notifications.Email.Enabled = true
}

func configWatchStamp(configPath string) string {
	paths := []string{
		configPath,
		filepath.Join(filepath.Dir(configPath), "yaml.d"),
		filepath.Join(filepath.Dir(configPath), "secrets.yml"),
		filepath.Join(filepath.Dir(configPath), "assets"),
	}
	var stamp strings.Builder
	for _, path := range paths {
		_ = filepath.Walk(path, func(entryPath string, info os.FileInfo, err error) error {
			if err != nil {
				stamp.WriteString(entryPath)
				stamp.WriteString(":missing;")
				return nil
			}
			fmt.Fprintf(&stamp, "%s:%d:%d:%d;", entryPath, info.ModTime().UnixNano(), info.Size(), info.Mode())
			return nil
		})
	}
	return stamp.String()
}

// warnRestartRequiredChanges logs settings that AI.md PART 5 classifies as
// process-restart-only when the config file changes.
func warnRestartRequiredChanges(oldCfg, newCfg *config.ApplicationConfig) {
	changed := restartRequiredSettings(oldCfg, newCfg)
	if len(changed) > 0 {
		log.Printf("Warning: config changes require a restart to fully take effect: %s", strings.Join(changed, ", "))
	}
}

func restartRequiredSettings(oldCfg, newCfg *config.ApplicationConfig) []string {
	var changed []string
	if oldCfg.Server.Port != newCfg.Server.Port {
		changed = append(changed, "server.port")
	}
	if oldCfg.Server.Address != newCfg.Server.Address {
		changed = append(changed, "server.address")
	}
	if oldCfg.Server.Daemonize != newCfg.Server.Daemonize {
		changed = append(changed, "server.daemonize")
	}
	oldSSL, newSSL := oldCfg.Server.SSL, newCfg.Server.SSL
	if oldSSL.Enabled != newSSL.Enabled {
		changed = append(changed, "ssl.enabled")
	}
	if oldSSL.Cert != newSSL.Cert {
		changed = append(changed, "ssl.cert")
	}
	if oldSSL.Key != newSSL.Key {
		changed = append(changed, "ssl.key")
	}
	if oldSSL.MinVersion != newSSL.MinVersion {
		changed = append(changed, "ssl.min_version")
	}
	oldDB, newDB := oldCfg.Server.Database, newCfg.Server.Database
	if oldDB.Driver != newDB.Driver {
		changed = append(changed, "database.driver")
	}
	if oldDB.URL != newDB.URL {
		changed = append(changed, "database.url")
	}
	if oldDB.Pool.MaxOpen != newDB.Pool.MaxOpen {
		changed = append(changed, "database.pool.max_open")
	}
	if oldDB.Pool.MaxIdle != newDB.Pool.MaxIdle {
		changed = append(changed, "database.pool.max_idle")
	}
	if oldDB.Pool.MaxLifetime != newDB.Pool.MaxLifetime {
		changed = append(changed, "database.pool.max_lifetime")
	}
	if oldDB.Pool.MaxIdleTime != newDB.Pool.MaxIdleTime {
		changed = append(changed, "database.pool.max_idle_time")
	}
	return changed
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
	fmt.Println("  --mode MODE           Run in production, development, or debug mode")
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
	fmt.Println("  --update branch NAME  Set update branch (stable, beta, devel)")
	fmt.Println()
	fmt.Println("Service Commands:")
	fmt.Println("  --service start       Start the service")
	fmt.Println("  --service stop        Stop the service")
	fmt.Println("  --service restart     Restart the service")
	fmt.Println("  --service reload      Reload configuration")
	fmt.Println("  --service --disable   Stop and disable the service")
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
	fmt.Println("  --maintenance email test [--to ADDRESS]  Send a test email")
	fmt.Println("  --maintenance email test --dry-run       Preview and validate all email templates")
	fmt.Println()
	fmt.Println("Tor Commands:")
	fmt.Println("  --maintenance tor status                 Show Tor hidden service status")
	fmt.Println("  --maintenance tor validate               Validate the Tor configuration")
	fmt.Println("  --maintenance tor restart                Restart the Tor process")
	fmt.Println("  --maintenance tor regenerate             Generate a new onion address")
	fmt.Println("  --maintenance tor vanity start <prefix>  Search for a vanity onion address")
	fmt.Println("  --maintenance tor vanity apply           Apply the completed vanity search result")
	fmt.Println("  --maintenance tor import-keys <path>     Import an existing onion service key file")
	fmt.Println()
	fmt.Println("Maintenance / Admin:")
	fmt.Println("  --maintenance                 Start in maintenance mode (read-only)")
	fmt.Println("  --maintenance backup [path]   Create a backup now")
	fmt.Println("  --maintenance backup test [path]  Verify a backup and its incremental base")
	fmt.Println("  --maintenance restore <path>  Restore from backup archive")
	fmt.Println("  --maintenance update          Check for and install a server update")
	fmt.Println("  --maintenance help            Show maintenance command help")
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
	fmt.Printf("  Root: /etc/%s/%s/server.yml\n", paths.OrgName, ProjectName)
	fmt.Printf("  User: ~/.config/%s/%s/server.yml\n", paths.OrgName, ProjectName)
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Printf("  %s                          # Start with defaults\n", binaryName)
	fmt.Printf("  %s --port 8080              # Start on port 8080\n", binaryName)
	fmt.Printf("  %s --maintenance            # Start in maintenance mode\n", binaryName)
	fmt.Printf("  %s --maintenance backup           # Create backup now\n", binaryName)
	fmt.Printf("  %s --maintenance backup test      # Verify the newest backup\n", binaryName)
	fmt.Printf("  %s --service --install      # Install as service\n", binaryName)
}

func printMaintenanceHelp(binaryName string) {
	fmt.Printf("%s maintenance operations\n", binaryName)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s --maintenance {backup,restore,update,mode,setup,pgp,secret,token,data,compliance,--help} [arguments]\n", binaryName)
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  backup [path]                  Create a backup")
	fmt.Println("  restore <path>                 Restore a backup archive")
	fmt.Println("  update                         Check for and install a server update")
	fmt.Println("  mode {production|development|debug}  Rejected pending the PART 6/PART 8 mode conflict")
	fmt.Println("  security {list|show|update}         Manage coordinated-disclosure reports")
	fmt.Println("  setup                          Reset server configuration (first run or root)")
	fmt.Println("  pgp {generate|rotate|publish|export|import|delete} Manage the security keypair")
	fmt.Println("  secret rotate {name}           Rotate a supported server secret")
	fmt.Println("  token {list|revoke <prefix>}   List or revoke API tokens")
	fmt.Println("  data {export|delete}           Export or delete stored user data")
	fmt.Println("  compliance report              Print the compliance summary")
	fmt.Println("  help                           Show this help")
	fmt.Println()
	fmt.Println("Sensitive commands require administrator authorization. Destructive operations may require typed confirmation.")
}

func modeChangeAuthorization(elevated, serviceUser bool) (requiresToken bool, err error) {
	if elevated {
		return false, nil
	}
	if serviceUser {
		return true, nil
	}
	return false, errors.New("mode changes require an administrator or the configured service account")
}

func maintenanceModeValue(value string) (string, error) {
	canonical := strings.ToLower(strings.TrimSpace(value))
	switch canonical {
	case "production", "development", "debug":
		return canonical, nil
	default:
		return "", fmt.Errorf("invalid mode %q (expected production, development, or debug)", value)
	}
}

func maintenanceOperatorTokenMatches(provided, configured string) bool {
	return provided != "" && configured != "" && len(provided) == len(configured) && subtle.ConstantTimeCompare([]byte(provided), []byte(configured)) == 1
}

func writeServerPID(pidPath string, inContainer, startedElevated bool) (bool, error) {
	if inContainer {
		return false, nil
	}
	if running, existingPID, err := paths.CheckPIDFile(pidPath); err != nil {
		log.Printf("Warning: could not check PID file %s: %v", pidPath, err)
	} else if running {
		return false, fmt.Errorf("already running (pid %d)", existingPID)
	}

	pidPerm := os.FileMode(0o600)
	if startedElevated {
		pidPerm = 0o644
	}
	if err := os.WriteFile(pidPath, fmt.Appendf(nil, "%d\n", os.Getpid()), pidPerm); err != nil {
		log.Printf("Warning: could not write PID file %s: %v", pidPath, err)
		return false, nil
	}
	return true, nil
}

func serverDatabaseIsEmpty(dbFile string) (bool, error) {
	if _, err := os.Stat(dbFile); errors.Is(err, os.ErrNotExist) {
		return true, nil
	} else if err != nil {
		return false, fmt.Errorf("inspect server database: %w", err)
	}
	databaseURL := url.URL{Scheme: "file", Path: filepath.ToSlash(dbFile)}
	query := databaseURL.Query()
	query.Set("mode", "ro")
	databaseURL.RawQuery = query.Encode()
	database, err := sql.Open("sqlite", databaseURL.String())
	if err != nil {
		return false, fmt.Errorf("open server database read-only: %w", err)
	}
	defer func() { _ = database.Close() }()
	if err := database.Ping(); err != nil {
		return false, fmt.Errorf("read server database: %w", err)
	}
	tableQuery := `SELECT name FROM sqlite_master WHERE type = 'table' AND name NOT LIKE 'sqlite_%'`
	started := time.Now()
	rows, err := database.Query(tableQuery)
	if err != nil {
		db.LogQuery(tableQuery, 0, started, 0, err)
		return false, fmt.Errorf("inspect server database tables: %w", err)
	}
	defer func() { _ = rows.Close() }()
	tableRows := int64(0)
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			db.LogQuery(tableQuery, 0, started, tableRows, err)
			return false, fmt.Errorf("read server database table: %w", err)
		}
		tableRows++
		quotedTable := `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
		var hasRows bool
		query := `SELECT EXISTS(SELECT 1 FROM ` + quotedTable + ` LIMIT 1)`
		if table == "app_secrets" {
			query = `SELECT EXISTS(SELECT 1 FROM "app_secrets" WHERE name NOT IN ('installation_secret', 'cookie_signing_key', 'csrf_token_secret') LIMIT 1)`
		}
		started := time.Now()
		if err := database.QueryRow(query).Scan(&hasRows); err != nil {
			db.LogQuery(query, 0, started, 0, err)
			return false, fmt.Errorf("inspect server database table %q: %w", table, err)
		}
		db.LogQuery(query, 0, started, 1, nil)
		if hasRows {
			return false, nil
		}
	}
	if err := rows.Err(); err != nil {
		db.LogQuery(tableQuery, 0, started, tableRows, err)
		return false, fmt.Errorf("read server database tables: %w", err)
	}
	db.LogQuery(tableQuery, 0, started, tableRows, nil)
	return true, nil
}

func confirmConfigurationReset() error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errors.New("resetting an initialized server requires an interactive confirmation")
	}
	fmt.Fprint(os.Stderr, "This resets server.yml to defaults. Type RESET to continue: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && len(line) == 0 {
		return fmt.Errorf("read reset confirmation: %w", err)
	}
	if strings.TrimSpace(line) != "RESET" {
		return errors.New("setup cancelled")
	}
	return nil
}

func runMaintenanceSetup(configDir string) error {
	if configDir == "" {
		configDir = firstNonEmpty(os.Getenv("CONFIG_DIR"))
	}
	if configDir == "" {
		configDir, _, _ = paths.GetDefaultDirs(ProjectName)
	}
	dbDir := firstNonEmpty(os.Getenv("DATABASE_DIR"), paths.GetSQLiteDBPath(ProjectName))
	dbEmpty, err := serverDatabaseIsEmpty(filepath.Join(dbDir, "server.db"))
	if err != nil {
		return err
	}
	if !dbEmpty && !service.IsElevated() {
		return errors.New("setup already completed; edit server.yml and restart, or run maintenance setup as root")
	}
	if !dbEmpty {
		if err := confirmConfigurationReset(); err != nil {
			return err
		}
	}

	configPath := filepath.Join(configDir, "server.yml")
	if err := config.SaveApplicationConfig(configPath, config.DefaultApplicationConfig()); err != nil {
		return fmt.Errorf("reset server configuration: %w", err)
	}
	if _, err := config.LoadApplicationConfig(configPath); err != nil {
		return fmt.Errorf("initialize default server secrets: %w", err)
	}
	fmt.Printf("Server configuration reset to defaults: %s\n", configPath)
	return nil
}

func runComplianceReport(configDir, logsDir string) error {
	defaultConfigDir, defaultDataDir, defaultLogsDir := paths.GetDefaultDirs(ProjectName)
	configDir = firstNonEmpty(configDir, os.Getenv("CONFIG_DIR"), defaultConfigDir)
	logsDir = firstNonEmpty(logsDir, os.Getenv("LOG_DIR"), defaultLogsDir)
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.LoadApplicationConfig(configPath)
	if err != nil {
		return fmt.Errorf("load server configuration: %w", err)
	}
	if !service.IsElevated() {
		provided, err := promptBackupPassword("Operator token (server.token): ")
		if err != nil {
			return err
		}
		if cfg.Server.Token == "" || len(provided) != len(cfg.Server.Token) || subtle.ConstantTimeCompare([]byte(provided), []byte(cfg.Server.Token)) != 1 {
			return errors.New("compliance maintenance denied: invalid operator token")
		}
	}
	database, err := db.OpenStore(cfg.Server.Database, ProjectName)
	if err != nil {
		return fmt.Errorf("open server database: %w", err)
	}
	defer func() { _ = database.Close() }()
	backupDir := paths.GetBackupDir("", firstNonEmpty(os.Getenv("DATA_DIR"), defaultDataDir))
	owners, resources, activeTokens, openReports, recentEvents, lastBackup, err := complianceCounts(database, backupDir, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("collect compliance summary: %w", err)
	}

	generatedAt := time.Now().UTC()
	complianceMode := "disabled"
	if cfg.Server.Compliance.IsEnabled() {
		complianceMode = "enabled"
	}
	standards := make([]string, 0, 11)
	for _, standard := range []struct {
		name    string
		enabled bool
	}{
		{"gdpr", cfg.Server.Compliance.GDPR},
		{"ccpa", cfg.Server.Compliance.CCPA},
		{"hipaa", cfg.Server.Compliance.HIPAA},
		{"soc2", cfg.Server.Compliance.SOC2},
		{"pci_dss", cfg.Server.Compliance.PCIDSS},
		{"iso27001", cfg.Server.Compliance.ISO27001},
		{"fedramp", cfg.Server.Compliance.FedRAMP},
		{"lgpd", cfg.Server.Compliance.LGPD},
		{"pipeda", cfg.Server.Compliance.PIPEDA},
		{"appi", cfg.Server.Compliance.APPI},
		{"pdpa", cfg.Server.Compliance.PDPA},
	} {
		if standard.enabled {
			standards = append(standards, standard.name)
		}
	}
	backupEncryption := "not configured"
	if cfg.Server.Backup.EncryptionPassword != "" {
		backupEncryption = "configured"
	}
	transportEncryption := "not configured"
	if cfg.Server.SSL.Enabled {
		transportEncryption = "configured"
	}
	auditCfg := toLoggingConfig(cfg.Server.Logging)
	auditCfg.Audit.Events = logging.AuditEventsConfig{
		Configuration: cfg.Server.Logging.Audit.Events.Configuration,
		Security:      cfg.Server.Logging.Audit.Events.Security,
		Backup:        cfg.Server.Logging.Audit.Events.Backup,
		Server:        cfg.Server.Logging.Audit.Events.Server,
	}
	auditManager := logging.NewLogManager(logsDir, auditCfg)
	defer func() {
		if closeErr := auditManager.Close(); closeErr != nil {
			log.Printf("Warning: could not close compliance report audit log: %v", closeErr)
		}
	}()
	auditIntegrity := auditManager.AuditIntegrity()
	fmt.Println("Airports compliance summary")
	fmt.Printf("Generated at: %s\n", generatedAt.Format(time.RFC3339))
	fmt.Printf("Compliance mode: %s\n", complianceMode)
	if len(standards) == 0 {
		fmt.Println("Enabled standards: none")
	} else {
		fmt.Printf("Enabled standards: %s\n", strings.Join(standards, ", "))
	}
	fmt.Println("Consent state: client-side only")
	effectiveRequirements := compliancerequirements.Resolve(cfg.Server.Compliance)
	if effectiveRequirements.AuditRetentionYears > 0 {
		fmt.Printf("Minimum audit retention: %d years\n", effectiveRequirements.AuditRetentionYears)
	}
	if effectiveRequirements.BreachNotification {
		if effectiveRequirements.BreachNotificationHours > 0 {
			fmt.Printf("Breach notification: required; shortest specified deadline=%d hours\n", effectiveRequirements.BreachNotificationHours)
		} else {
			fmt.Println("Breach notification: required; enabled standards define no common numeric deadline")
		}
	}
	fmt.Printf("Matrix requirements: encrypt_at_rest=%t encrypt_in_transit=%t erasure=%t portability=%t consent=%t access_logging=%t data_residency=%t strong_auth=%t token_policy=%t session_timeout=%t vulnerability_scanning=%t\n",
		effectiveRequirements.EncryptAtRest,
		effectiveRequirements.EncryptInTransit,
		effectiveRequirements.RightToErasure,
		effectiveRequirements.DataPortability,
		effectiveRequirements.ConsentTracking,
		effectiveRequirements.AccessLogging,
		effectiveRequirements.DataResidency,
		effectiveRequirements.StrongAuthentication,
		effectiveRequirements.TokenPolicy,
		effectiveRequirements.SessionTimeoutRequired,
		effectiveRequirements.VulnerabilityScanning,
	)
	fmt.Printf("Data retention: %s\n", firstNonEmpty(cfg.Server.Privacy.Retention.Period, "not configured"))
	if cfg.Server.Compliance.GDPR {
		regions := "not configured"
		if len(cfg.Server.Compliance.GDPRAllowedRegions) > 0 {
			regions = strings.Join(cfg.Server.Compliance.GDPRAllowedRegions, ", ")
		}
		fmt.Printf("GDPR allowed data regions: %s\n", regions)
	}
	if cfg.Server.Compliance.FedRAMP {
		fmt.Printf("FedRAMP impact level: %s\n", firstNonEmpty(cfg.Server.Compliance.FedRAMPImpactLevel, "not configured"))
		fmt.Println("FedRAMP data residency: US-only required; deployment location is not verified by this report")
	}
	dpoContact := strings.TrimSpace(cfg.Server.Contact.DPO.Email)
	if dpoContact == "" {
		dpoContact = strings.TrimSpace(cfg.Server.Contact.General.Email)
	}
	dpoStatus := "not configured"
	if dpoContact != "" {
		dpoStatus = "configured"
	}
	fmt.Printf("DPO contact: %s\n", dpoStatus)
	fmt.Printf("CCPA opt-out applicable: %t\n", cfg.Server.Privacy.Data.Sold)
	fmt.Printf("Backup retention: daily=%d weekly=%d monthly=%d yearly=%d\n", cfg.Server.Backup.Retention.MaxBackups, cfg.Server.Backup.Retention.KeepWeekly, cfg.Server.Backup.Retention.KeepMonthly, cfg.Server.Backup.Retention.KeepYearly)
	fmt.Printf("Audit logging: enabled=%t retention=%s categories=configuration:%t,security:%t,backup:%t,server:%t\n",
		cfg.Server.Logging.Audit.Enabled,
		firstNonEmpty(cfg.Server.Logging.Audit.Keep, "none"),
		cfg.Server.Logging.Audit.Events.Configuration,
		cfg.Server.Logging.Audit.Events.Security,
		cfg.Server.Logging.Audit.Events.Backup,
		cfg.Server.Logging.Audit.Events.Server,
	)
	if auditIntegrity.Intact {
		fmt.Printf("Audit log integrity: intact (%d entries)\n", auditIntegrity.Entries)
	} else {
		fmt.Printf("Audit log integrity: broken at %s:%d\n", auditIntegrity.File, auditIntegrity.Line)
	}
	fmt.Printf("Security report retention: %d days\n", cfg.Server.Security.Reports.RetentionDays)
	fmt.Printf("Counts: owners=%d resources=%d active_tokens=%d open_security_reports=%d\n", owners, resources, activeTokens, openReports)
	fmt.Printf("Compliance events in last 30 days: %d\n", recentEvents)
	fmt.Printf("Last backup: %s\n", lastBackup)
	fmt.Printf("Backup encryption: %s\n", backupEncryption)
	fmt.Printf("TLS configuration: %s\n", transportEncryption)

	generatedBy := "unknown"
	if currentUser, userErr := user.Current(); userErr == nil && currentUser.Username != "" {
		generatedBy = currentUser.Username
	}
	auditLine := fmt.Sprintf("event=compliance.report_generated report_type=summary generated_by=%s", strconv.Quote(generatedBy))
	if err := recordComplianceEvent(database, "compliance.report_generated", "", "report_type=summary"); err != nil {
		return fmt.Errorf("record compliance report event: %w", err)
	}
	if err := auditManager.WriteAudit(logging.AuditCategoryServer, auditLine); err != nil {
		return fmt.Errorf("write compliance report audit event: %w", err)
	}
	return nil
}

func checkStatus() int {
	// Try to connect to health endpoint
	defaultConfigDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configDir := firstNonEmpty(os.Getenv("CONFIG_DIR"), defaultConfigDir)
	configPath := filepath.Join(configDir, "server.yml")

	cfg, err := config.LoadApplicationConfig(configPath)
	if err != nil {
		fmt.Println("Status: Not configured")
		return 1
	}
	port := firstNonEmpty(cfg.Server.Port, os.Getenv("PORT"))
	if port == "" && paths.IsRunningInContainer() {
		port = "80"
	}
	if port == "" {
		fmt.Println("Status: Not configured")
		return 1
	}

	statusClient := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://127.0.0.1:%s/server/healthz", port), nil)
	if err != nil {
		fmt.Printf("Status: Not running (port %s)\n", port)
		return 1
	}
	req.Header.Set("User-Agent", "airports-cli/"+Version)
	resp, err := statusClient.Do(req)
	if err != nil {
		fmt.Printf("Status: Not running (port %s)\n", port)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()

	var health struct {
		Status   string `json:"status"`
		Features struct {
			I2P struct {
				Enabled  bool   `json:"enabled"`
				Running  bool   `json:"running"`
				Hostname string `json:"hostname"`
				Provider string `json:"provider"`
			} `json:"i2p"`
		} `json:"features"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		fmt.Printf("Status: Unhealthy (invalid health response)\n")
		return 1
	}
	if resp.StatusCode == http.StatusOK {
		fmt.Printf("Status: Running on port %s\n", port)
		if health.Status != "" {
			fmt.Printf("Health: %s\n", health.Status)
		}
		if health.Features.I2P.Enabled {
			fmt.Printf("I2P: enabled=%t running=%t provider=%s hostname=%s\n", health.Features.I2P.Enabled, health.Features.I2P.Running, health.Features.I2P.Provider, health.Features.I2P.Hostname)
		}
		return 0
	}

	fmt.Printf("Status: Unhealthy (HTTP %d)\n", resp.StatusCode)
	return 1
}

// serviceEscalationActions maps each service subcommand needing root/admin
// to a human-readable action description used in escalation prompts, per
// AI.md PART 5 "Commands Requiring Escalation" table. The help subcommand
// is handled before this map because it is read-only.
var serviceEscalationActions = map[string]string{
	"start":     "Starting the service",
	"stop":      "Stopping the service",
	"restart":   "Restarting the service",
	"reload":    "Reloading the service",
	"disable":   "Disabling the service",
	"install":   "Installing the service",
	"uninstall": "Uninstalling the service",
}

func handleServiceCommand(cmd string) error {
	rawCommand := strings.TrimSpace(cmd)
	switch rawCommand {
	case "--install":
		cmd = "install"
	case "--uninstall":
		cmd = "uninstall"
	case "--disable":
		cmd = "disable"
	case "--help", "help":
		printServiceHelp()
		return nil
	case "install", "uninstall", "disable", "status", "enable", "logs":
		return fmt.Errorf("unknown service command: %s", rawCommand)
	default:
		cmd = rawCommand
	}
	if !service.IsElevated() && service.UserServiceInstalled() {
		switch cmd {
		case "install":
			return service.InstallUserService()
		case "start", "stop", "restart", "reload", "disable":
			return service.UserServiceAction(cmd)
		case "uninstall":
			return service.UninstallUserService()
		}
	}
	if action, needsEscalation := serviceEscalationActions[cmd]; needsEscalation {
		if err := service.HandleEscalation(action); err != nil {
			if cmd == "install" && !service.IsElevated() {
				fmt.Println("Installing user service instead...")
				return service.InstallUserService()
			}
			return err
		}
	}

	switch cmd {
	case "start":
		return service.StartSystemService()
	case "stop":
		return service.StopSystemService()
	case "restart":
		return service.RestartSystemService()
	case "reload":
		return service.ReloadSystemService()
	case "disable":
		return service.DisableSystemService()
	case "install":
		// Per AI.md PART 23 the install flow must detect the init system,
		// write the service file, enable it, AND start it. Install() writes
		// and enables; start it immediately afterwards.
		if err := service.InstallSystemService(); err != nil {
			return err
		}
		return service.StartSystemService()
	case "uninstall":
		return service.UninstallSystemService()
	default:
		return fmt.Errorf("unknown service command: %s", cmd)
	}
}

// schedulerDeps bundles every subsystem a scheduled task handler may need,
// per AI.md PART 18 "Built-in Tasks". Constructed once at startup (see
// buildSchedulerDeps) and shared by both call sites: the live server's run()
// and the standalone `scheduler run <id>` CLI path in handleSchedulerCLI.
type schedulerDeps struct {
	db              db.Handle
	geoipSvc        *geoip.GeoIPService
	cveSvc          *cve.CVEService
	blockStore      *server.BlockStore
	logMgr          *logging.LogManager
	torMgr          *tor.TorManager
	i2pMgr          *i2p.I2PManager
	sslMgr          *ssl.SSLManager
	fqdn            string
	healthURL       string
	configDir       string
	dataDir         string
	logsDir         string
	backupDir       string
	startedElevated bool
}

// toLoggingConfig bridges config.LoggingConfig (server.yml's `server.logs`
// schema, PART 11 "Logging" § Configuration) to logging.LogManagerConfig, the log
// rotation subsystem's own shape. Only Audit and Debug expose an `enabled`
// toggle in server.yml (matching PART 11 — every other log type is always
// on), so Access/Server/Error/Security are always mapped Enabled: true.
func toLoggingConfig(lc config.LoggingConfig) logging.LogManagerConfig {
	return logging.LogManagerConfig{
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
// ssl.SSLManagerConfig, the SSL/Let's Encrypt subsystem's own shape. certPath is the
// on-disk directory the ssl package uses for both the autocert cache and
// manually-placed certificates (paths.GetSSLDir), never a config value.
// encryptionKey is server.security.encryption_key, used to decrypt
// cfg.LetsEncrypt.DNSCredentials (an AES-256-GCM blob) into the plaintext
// provider credential map the ssl package needs, per AI.md PART 15
// "Provider Credential Storage". If decryption fails (e.g. dns-01 not
// configured yet, or the blob is empty), DNSCredentials is left empty and
// the failure is logged rather than aborting startup.
func toSSLConfig(cfg config.SSLConfig, encryptionKey string, certPath string) ssl.SSLManagerConfig {
	var dnsCreds map[string]string
	if cfg.LetsEncrypt.DNSCredentials != "" {
		decrypted, err := ssl.DecryptDNSCredentials(encryptionKey, cfg.LetsEncrypt.DNSCredentials)
		if err != nil {
			log.Printf("Warning: failed to decrypt dns-01 provider credentials: %v", err)
		} else {
			dnsCreds = decrypted
		}
	}

	return ssl.SSLManagerConfig{
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
// *tor.TorManager instance can also be shared with server.NewServer(), letting
// /server/healthz report the live Tor state) and simply attached here.
func buildSchedulerDeps(cfg *config.ApplicationConfig, configDir, dataDir, logsDir string, startedElevated bool, database db.Handle, geoipSvc *geoip.GeoIPService, blockStore *server.BlockStore, listenPort string, torMgr *tor.TorManager, logManager *logging.LogManager) schedulerDeps {
	deps := schedulerDeps{
		db:              database,
		geoipSvc:        geoipSvc,
		blockStore:      blockStore,
		fqdn:            cfg.Server.FQDN,
		torMgr:          torMgr,
		configDir:       configDir,
		dataDir:         dataDir,
		logsDir:         logsDir,
		backupDir:       paths.GetBackupDir(backupDirOverride, dataDir),
		startedElevated: startedElevated,
	}

	securityDir := paths.GetSecurityDir(ProjectName)
	if cveSvc, err := cve.NewCVEService(securityDir, cfg.Server.CVE.APIKey); err != nil {
		log.Printf("Warning: CVE service unavailable: %v", err)
	} else {
		deps.cveSvc = cveSvc
	}

	deps.logMgr = logManager
	if deps.logMgr == nil {
		deps.logMgr = logging.NewLogManager(logsDir, toLoggingConfig(cfg.Server.Logging))
	}

	sslCfg := toSSLConfig(cfg.Server.SSL, cfg.Server.Security.EncryptionKey, paths.GetSSLDir(ProjectName))
	if sslCfg.LetsEncrypt.Challenge == "dns-01" && sslCfg.LetsEncrypt.DNSProvider != "" {
		if err := ssl.ValidateDNSProviderCredentials(sslCfg.LetsEncrypt.DNSProvider, sslCfg.LetsEncrypt.DNSCredentials); err != nil {
			log.Printf("Warning: dns-01 provider %q credentials invalid: %v", sslCfg.LetsEncrypt.DNSProvider, err)
		} else {
			log.Printf("dns-01 provider %q credentials validated", sslCfg.LetsEncrypt.DNSProvider)
		}
	}

	sslMgr := ssl.NewSSLManager(sslCfg)
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

// registerSchedulerTasks wires the configured built-in tasks from AI.md PART 18.
func registerSchedulerTasks(sched *scheduler.Scheduler, cfg *config.ApplicationConfig, deps schedulerDeps) {
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
			updateErr := deps.geoipSvc.UpdateDatabases()
			server.RefreshHealthChecks()
			return updateErr
		})

	mustRegister("blocklist_update", "IP blocklist update", tasks.BlocklistUpdate.Schedule,
		tasks.BlocklistUpdate.Enabled, tasks.BlocklistUpdate.RetryOnFail, tasks.BlocklistUpdate.RetryDelay,
		func() error {
			if deps.blockStore == nil {
				return fmt.Errorf("blocklist store unavailable")
			}
			updateErr := server.UpdateBlocklistInDir(deps.blockStore, filepath.Join(deps.dataDir, "security"), ProjectName)
			server.RefreshHealthChecks()
			return updateErr
		})

	mustRegister("cve_update", "CVE database update", tasks.CVEUpdate.Schedule,
		tasks.CVEUpdate.Enabled, tasks.CVEUpdate.RetryOnFail, tasks.CVEUpdate.RetryDelay,
		func() error {
			if deps.cveSvc == nil {
				return fmt.Errorf("cve service unavailable")
			}
			updateErr := deps.cveSvc.UpdateCVEFeed()
			server.RefreshHealthChecks()
			return updateErr
		})

	mustRegister("health_dataset_refresh", "Refresh data set health checks", "@every 1m", true, true, "1m", func() error {
		server.RefreshHealthChecks()
		return nil
	})

	mustRegister("update_check", "Update check", tasks.UpdateCheck.Schedule,
		tasks.UpdateCheck.Enabled, false, "",
		func() error {
			return runScheduledUpdateCheckWithPaths(cfg, deps.configDir, deps.dataDir, deps.logsDir, deps.startedElevated, &http.Client{Timeout: 30 * time.Second})
		})

	mustRegister("disk_check", "Disk space low check", tasks.DiskCheck.Schedule,
		tasks.DiskCheck.Enabled, false, "",
		func() error {
			return runDiskLowCheck(cfg, deps.configDir, []string{deps.dataDir, deps.logsDir, deps.backupDir}, time.Now())
		})

	mustRegister("database_check", "Database connection probe", tasks.DatabaseCheck.Schedule,
		tasks.DatabaseCheck.Enabled, false, "",
		func() error {
			return runDatabaseConnectionCheck(deps.db, cfg, deps.configDir, time.Now())
		})

	mustRegister("token_cleanup", "Expired token cleanup", tasks.TokenCleanup.Schedule,
		tasks.TokenCleanup.Enabled, false, "",
		func() error {
			if err := server.CleanupExpiredContactChallenges(deps.db, time.Now()); err != nil {
				return err
			}
			return server.CleanupOwnerTokenData(deps.db, cfg.Server.Tokens.IdlePurgeDays, time.Now())
		})

	mustRegister("security_cleanup", "Security report and retired PGP key cleanup", "30 3 * * *", true, true, "5m",
		func() error {
			return CleanupSecurityReports(deps.db, deps.configDir, time.Now().UTC(), cfg.Server.Security.Reports.RetentionDays)
		})

	mustRegister("keyserver_retry", "PGP keyserver retry", "@every 15m", true, true, "15m",
		func() error {
			return retryPendingPGPKeyservers(deps.db, deps.configDir, cfg.Web.Security.Keyservers, deps.logMgr, time.Now().UTC())
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
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("self health check returned status %d", resp.StatusCode)
			}
			return nil
		})

	mustRegister("tor_health", "Tor hidden service health check", tasks.TorHealth.Schedule,
		tasks.TorHealth.Enabled, tasks.TorHealth.RetryOnFail, tasks.TorHealth.RetryDelay,
		func() error {
			if deps.torMgr == nil || !deps.torMgr.IsTorEnabled() {
				return nil
			}
			return deps.torMgr.HealthCheck()
		})
	if cfg.Server.I2P.Enabled {
		mustRegister("i2p_health", "I2P eepsite health check", "@every 10m", true, true, "1m", func() error {
			if deps.i2pMgr == nil {
				return fmt.Errorf("I2P manager is unavailable")
			}
			return deps.i2pMgr.HealthCheck()
		})
	}
}

func runDiskLowCheck(cfg *config.ApplicationConfig, configDir string, directories []string, now time.Time) error {
	settings := cfg.Server.Notifications.DiskLow
	if !settings.Enabled {
		return nil
	}
	type volumeUsage struct {
		path        string
		usedPercent int
		free        uint64
		total       uint64
	}
	volumes := make(map[string]volumeUsage)
	for _, directory := range directories {
		if directory == "" {
			continue
		}
		total, free, err := backup.DiskUsage(directory)
		if err != nil {
			return fmt.Errorf("measure filesystem for %q: %w", directory, err)
		}
		if total == 0 {
			continue
		}
		key, err := diskVolumeKey(directory, total, free)
		if err != nil {
			return fmt.Errorf("identify filesystem for %q: %w", directory, err)
		}
		if _, exists := volumes[key]; exists {
			continue
		}
		used := total - free
		volumes[key] = volumeUsage{path: directory, usedPercent: int((used*100 + total/2) / total), free: free, total: total}
	}
	repeat, err := time.ParseDuration(settings.Repeat)
	if err != nil || repeat <= 0 {
		repeat = 24 * time.Hour
	}
	for key, usage := range volumes {
		diskLowState.Lock()
		lastNotice, notified := diskLowState.volumes[key]
		if usage.usedPercent <= settings.ThresholdPercent-5 {
			if notified {
				delete(diskLowState.volumes, key)
				diskLowState.Unlock()
				slog.Info("Disk space recovered", "path", usage.path, "used_percent", usage.usedPercent, "threshold_percent", settings.ThresholdPercent)
				continue
			}
			diskLowState.Unlock()
			continue
		}
		if usage.usedPercent < settings.ThresholdPercent || notified && now.Sub(lastNotice) < repeat {
			diskLowState.Unlock()
			continue
		}
		diskLowState.volumes[key] = now
		diskLowState.Unlock()
		slog.Warn("Disk space low", "path", usage.path, "used_percent", usage.usedPercent, "free_bytes", usage.free, "threshold_percent", settings.ThresholdPercent)
		vars := notify.GlobalVars(cfg)
		vars["path"] = usage.path
		vars["used_percent"] = strconv.Itoa(usage.usedPercent)
		vars["free"] = humanBytes(usage.free)
		vars["threshold_percent"] = strconv.Itoa(settings.ThresholdPercent)
		if sendErr := notify.Send(cfg, configDir, "disk_low", notifyRecipient(cfg), vars); sendErr != nil {
			log.Printf("Warning: failed to send disk_low notification email: %v", sendErr)
		}
	}
	return nil
}

func runDatabaseConnectionCheck(database db.Handle, cfg *config.ApplicationConfig, configDir string, now time.Time) error {
	settings := cfg.Server.Notifications.DatabaseError
	if !settings.Enabled {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	probeErr := db.HealthProbe(ctx, database)
	cancel()

	databaseErrorState.Lock()
	if probeErr == nil {
		recovered := databaseErrorState.failures > 0
		databaseErrorState.failures = 0
		databaseErrorState.since = time.Time{}
		databaseErrorState.lastNotification = time.Time{}
		databaseErrorState.Unlock()
		if recovered {
			slog.Info("Database connection recovered")
		}
		return nil
	}
	if databaseErrorState.failures == 0 {
		databaseErrorState.since = now
	}
	databaseErrorState.failures++
	failures := databaseErrorState.failures
	since := databaseErrorState.since
	repeat, err := time.ParseDuration(settings.Repeat)
	if err != nil || repeat <= 0 {
		repeat = 24 * time.Hour
	}
	shouldNotify := failures >= settings.Failures && (databaseErrorState.lastNotification.IsZero() || now.Sub(databaseErrorState.lastNotification) >= repeat)
	if shouldNotify {
		databaseErrorState.lastNotification = now
	}
	databaseErrorState.Unlock()
	if !shouldNotify {
		return nil
	}
	message := sanitizeDatabaseProbeError(probeErr, cfg.Server.Database.URL, cfg.Server.Database.Token)
	slog.Warn("Database connection problem", "failures", failures, "since", since.UTC().Format(time.RFC3339), "error", message)
	vars := notify.GlobalVars(cfg)
	vars["error"] = message
	vars["since"] = since.UTC().Format(time.RFC3339)
	if sendErr := notify.Send(cfg, configDir, "database_error", notifyRecipient(cfg), vars); sendErr != nil {
		log.Printf("Warning: failed to send database_error notification email: %v", sendErr)
	}
	return nil
}

func sanitizeDatabaseProbeError(err error, databaseURL, token string) string {
	message := err.Error()
	if databaseURL != "" {
		message = strings.ReplaceAll(message, databaseURL, "[database endpoint]")
	}
	if token != "" {
		message = strings.ReplaceAll(message, token, "xxxxx")
	}
	return message
}

func diskVolumeKey(path string, total, free uint64) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	system := reflect.ValueOf(info.Sys())
	if system.Kind() == reflect.Pointer && !system.IsNil() {
		system = system.Elem()
	}
	if system.IsValid() && system.Kind() == reflect.Struct {
		device := system.FieldByName("Dev")
		if device.IsValid() && device.CanUint() {
			return fmt.Sprintf("device:%d", device.Uint()), nil
		}
	}
	if volume := filepath.VolumeName(path); volume != "" {
		return "volume:" + strings.ToLower(volume), nil
	}
	return fmt.Sprintf("capacity:%d:%d", total, free), nil
}

func humanBytes(value uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	size := float64(value)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d %s", value, units[unit])
	}
	return fmt.Sprintf("%.1f %s", size, units[unit])
}

func printServiceHelp() {
	fmt.Println("Service Commands:")
	fmt.Println()
	fmt.Println("  --install   Install, enable, and start the system service")
	fmt.Println("  --uninstall Stop and remove the service and all server data after confirmation")
	fmt.Println("  start       Start the service")
	fmt.Println("  stop        Stop the service")
	fmt.Println("  restart     Restart the service")
	fmt.Println("  reload      Reload configuration")
	fmt.Println("  --disable   Stop and disable the service")
	fmt.Println("  --help      Show this help")
	fmt.Println()
	fmt.Println("Supported service managers:")
	fmt.Println("  Linux:   systemd, OpenRC, runit")
	fmt.Println("  macOS:   launchd")
	fmt.Println("  Windows: Windows Service Manager")
	fmt.Println("  BSD:     rc.d")
}

func printUpdateHelp(binaryName string) {
	fmt.Printf("%s update commands\n", binaryName)
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Printf("  %s --update [check|yes|branch {stable|beta|devel}|--help]\n", binaryName)
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  check                 Check for an available update")
	fmt.Println("  yes                   Download and install an available update")
	fmt.Println("  branch {stable|beta|devel}  Select the update channel (daily/rolling/dev/development alias devel)")
	fmt.Println("  --help                Show this help")
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
	if resolvedBackupDir != "" {
		return resolvedBackupDir
	}
	dataDir := backupDataDir
	if dataDir == "" {
		_, dataDir, _ = paths.GetDefaultDirs(ProjectName)
	}
	return paths.GetBackupDir(backupDirOverride, dataDir)
}

func runDiskPressureCleanup(cleanup config.CleanupConfig, dataDir, backupDir string, logs *logging.LogManager, now time.Time, diskUsage func(string) (uint64, uint64, error)) (bool, error) {
	if dataDir == "" || diskUsage == nil {
		return false, fmt.Errorf("maintenance cleanup requires a data directory and disk-usage provider")
	}
	total, free, err := diskUsage(dataDir)
	if err != nil {
		return false, fmt.Errorf("check data filesystem usage: %w", err)
	}
	if total == 0 || free > total {
		return false, nil
	}
	usedPercent := float64(total-free) / float64(total) * 100
	if usedPercent <= float64(cleanup.DiskThreshold) {
		return false, nil
	}
	cutoff := now.Add(-time.Duration(cleanup.LogRetentionDays) * 24 * time.Hour)
	deletedLogs, logErr := logs.PruneRotatedBefore(cutoff)
	deletedTempFiles, tempErr := pruneProjectTempFiles(now.Add(-24*time.Hour), os.TempDir())
	var backupErr error
	if backupDir != "" {
		if info, statErr := os.Stat(backupDir); statErr == nil && info.IsDir() {
			_, backupErr = backup.ApplyRetention(backupDir, ProjectName, backup.RetentionConfig{MaxBackups: cleanup.BackupKeepCount})
		} else if statErr != nil && !os.IsNotExist(statErr) {
			backupErr = fmt.Errorf("inspect cached backup directory: %w", statErr)
		}
	}
	var freed uint64
	if afterTotal, afterFree, statErr := diskUsage(dataDir); statErr == nil && afterTotal == total && afterFree > free {
		freed = afterFree - free
	}
	log.Printf("Maintenance disk-pressure cleanup: usage=%.1f%% removed_rotated_logs=%d removed_temp_files=%d estimated_freed_bytes=%d", usedPercent, deletedLogs, deletedTempFiles, freed)
	return true, errors.Join(logErr, tempErr, backupErr)
}

// pruneProjectTempFiles removes only expired regular files inside this
// project's namespaced OS temp directories. Symlinks and directories are
// preserved so cleanup cannot escape the project temp namespace.
func pruneProjectTempFiles(cutoff time.Time, tempRoot string) (int, error) {
	orgRoot := filepath.Join(tempRoot, paths.OrgName)
	orgInfo, err := os.Lstat(orgRoot)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("inspect project temp namespace: %w", err)
	}
	if !orgInfo.IsDir() || orgInfo.Mode()&os.ModeSymlink != 0 {
		return 0, fmt.Errorf("project temp namespace is not a real directory")
	}

	entries, err := os.ReadDir(orgRoot)
	if err != nil {
		return 0, fmt.Errorf("read project temp namespace: %w", err)
	}
	removed := 0
	var cleanupErr error
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ProjectName+"-") || !entry.IsDir() {
			continue
		}
		dir := filepath.Join(orgRoot, entry.Name())
		info, statErr := os.Lstat(dir)
		if statErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("inspect project temp directory: %w", statErr))
			continue
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			continue
		}
		walkErr := filepath.WalkDir(dir, func(file string, d os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			fileInfo, infoErr := d.Info()
			if infoErr != nil {
				return infoErr
			}
			if !fileInfo.Mode().IsRegular() || !fileInfo.ModTime().Before(cutoff) {
				return nil
			}
			if removeErr := os.Remove(file); removeErr != nil {
				return removeErr
			}
			removed++
			return nil
		})
		if walkErr != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("prune project temp directory %q: %w", entry.Name(), walkErr))
		}
	}
	return removed, cleanupErr
}

func snapshotDatabaseForBackup(ctx context.Context, cfg config.DatabaseConfig, destination string) error {
	store, err := db.OpenStore(cfg, ProjectName)
	if err != nil {
		return err
	}
	snapshotErr := store.Snapshot(ctx, destination)
	closeErr := store.Close()
	return errors.Join(snapshotErr, closeErr)
}

// createBackup builds a backup archive via the src/backup package (AI.md
// PART 21) and applies retention afterward. An empty fileLocation resolves
// to {backup_dir}/{project}_backup_{date}.tar.gz[.enc], matching the
// scheduled-task naming convention. interactive controls whether a missing
// encryption password is prompted for (never for scheduled/non-TTY calls).
func createBackup(fileLocation string, interactive bool) error {
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	cfg := config.GetApplicationConfig()
	snapshotAt := time.Now().UTC()

	password := cfg.Server.Backup.EncryptionPassword
	if password == "" && interactive {
		var err error
		password, err = promptBackupPassword("Backup encryption password (leave blank for unencrypted): ")
		if err != nil {
			return err
		}
	}

	if cfg.Server.Compliance.IsEnabled() && password == "" {
		if !interactive {
			log.Printf("WARN: scheduled backup skipped — a server.compliance standard is enabled but no server.backup.encryption_password is configured")
			return nil
		}
		return fmt.Errorf("compliance mode requires an encryption password — set server.backup.encryption_password or enter one when prompted")
	}

	backupDir := resolveBackupDir()
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	isScheduledDefaultLocation := fileLocation == "" && !interactive
	if fileLocation == "" {
		date := snapshotAt.Format("2006-01-02")
		ext := "tar.gz"
		if password != "" {
			ext = "tar.gz.enc"
		}
		if isScheduledDefaultLocation {
			fileLocation = filepath.Join(backupDir, fmt.Sprintf("%s_backup_%s.%s", ProjectName, date, ext))
		} else {
			fileLocation = filepath.Join(backupDir, fmt.Sprintf("%s_backup_%s_%s.%s", ProjectName, date, snapshotAt.Format("150405"), ext))
		}
		if _, err := os.Stat(fileLocation); err == nil {
			stem := fmt.Sprintf("%s_backup_%s_%s", ProjectName, date, snapshotAt.Format("150405"))
			fileLocation = filepath.Join(backupDir, stem+"."+ext)
			for suffix := 1; ; suffix++ {
				if _, statErr := os.Stat(fileLocation); os.IsNotExist(statErr) {
					break
				} else if statErr != nil {
					return fmt.Errorf("inspect backup destination %s: %w", fileLocation, statErr)
				}
				fileLocation = filepath.Join(backupDir, fmt.Sprintf("%s_%d.%s", stem, suffix, ext))
			}
		}
	}

	fmt.Printf("Creating backup: %s\n", fileLocation)
	backupOpts := backup.BackupArchiveOptions{
		ConfigDir: configDir,
		DataDir:   dataDir,
		DatabaseSnapshot: func(ctx context.Context, destination string) error {
			return snapshotDatabaseForBackup(ctx, cfg.Server.Database, destination)
		},
		IncludeSSL:         true,
		IncludeData:        true,
		EncryptionPassword: password,
		ComplianceEnabled:  cfg.Server.Compliance.IsEnabled(),
		AppVersion:         Version,
		CreatedBy:          ProjectName,
		SnapshotAt:         snapshotAt,
	}
	pairCreated := false
	if isScheduledDefaultLocation {
		var err error
		pairCreated, err = createScheduledBackupPair(fileLocation, backupDir, password, backupOpts, cfg)
		if err != nil {
			if sendErr := notify.Send(cfg, configDir, "backup_failed", notifyRecipient(cfg), map[string]string{"filename": filepath.Base(fileLocation), "error": err.Error()}); sendErr != nil {
				log.Printf("Warning: failed to send backup_failed notification email: %v", sendErr)
			}
			return fmt.Errorf("scheduled backup pair failed: %w: %w", err, notify.ErrSuppressScheduler)
		}
	}
	if !pairCreated {
		if err := backup.CreateBackupArchive(fileLocation, backupOpts); err != nil {
			if sendErr := notify.Send(cfg, configDir, "backup_failed", notifyRecipient(cfg), map[string]string{
				"filename": filepath.Base(fileLocation),
				"error":    err.Error(),
			}); sendErr != nil {
				log.Printf("Warning: failed to send backup_failed notification email: %v", sendErr)
			}
			return fmt.Errorf("%w: %w", err, notify.ErrSuppressScheduler)
		}

		if err := backup.VerifyWithLimits(fileLocation, password, configuredRestoreLimits(cfg)); err != nil {
			_ = os.Remove(fileLocation)
			if sendErr := notify.Send(cfg, configDir, "backup_failed", notifyRecipient(cfg), map[string]string{
				"filename": filepath.Base(fileLocation),
				"error":    err.Error(),
			}); sendErr != nil {
				log.Printf("Warning: failed to send backup_failed notification email: %v", sendErr)
			}
			return fmt.Errorf("backup verification failed, removed: %w: %w", err, notify.ErrSuppressScheduler)
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

func createScheduledBackupPair(fullPath, backupDir, password string, opts backup.BackupArchiveOptions, cfg *config.ApplicationConfig) (bool, error) {
	basePath, baseManifest, err := latestFullBackup(backupDir, password)
	if err != nil {
		return false, err
	}
	if basePath == "" {
		log.Printf("INFO: skipping daily incremental backup: no previous full backup")
		return false, nil
	}
	matched, corrupt, err := backupPasswordMatchesBase(basePath, password)
	if err != nil {
		return false, err
	}
	if corrupt {
		log.Printf("WARN: skipping daily incremental backup: previous full backup header is corrupt: %s", filepath.Base(basePath))
		return false, nil
	}
	if !matched {
		log.Printf("INFO: skipping daily incremental backup because backup encryption password changed")
		return false, nil
	}
	if baseManifest == nil {
		log.Printf("WARN: skipping daily incremental backup because previous full backup is corrupt: %s", filepath.Base(basePath))
		return false, nil
	}
	ext := "tar.gz"
	if password != "" {
		ext = "tar.gz.enc"
	}
	incPath := filepath.Join(backupDir, fmt.Sprintf("%s-daily.%s", ProjectName, ext))
	incTemp, err := createTemporaryBackupPath(backupDir, ProjectName+"-daily", ext)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := os.Remove(incTemp); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("WARN: failed to remove temporary incremental backup %s: %v", incTemp, err)
		}
	}()
	fullTemp, err := createTemporaryBackupPath(backupDir, ProjectName+"-full", ext)
	if err != nil {
		return false, err
	}
	defer func() {
		if err := os.Remove(fullTemp); err != nil && !errors.Is(err, os.ErrNotExist) {
			log.Printf("WARN: failed to remove temporary full backup %s: %v", fullTemp, err)
		}
	}()
	incOpts := opts
	incOpts.BackupType = "incremental"
	incOpts.BaseBackup = filepath.Base(basePath)
	incOpts.BaseArchivePath = basePath
	incOpts.BaseSHA256 = baseManifest.Checksum
	if err := backup.CreateBackupPair(incTemp, fullTemp, incOpts); err != nil {
		return false, err
	}
	limits := configuredRestoreLimits(cfg)
	if err := backup.VerifyWithLimits(incTemp, password, limits); err != nil {
		return false, fmt.Errorf("daily incremental verification: %w", err)
	}
	if err := backup.VerifyWithLimits(fullTemp, password, limits); err != nil {
		return false, fmt.Errorf("full backup verification: %w", err)
	}
	if err := finalizeBackupPair(fullTemp, fullPath, incTemp, incPath, backupDir); err != nil {
		return false, err
	}
	previousExt := "tar.gz"
	if ext == "tar.gz" {
		previousExt = "tar.gz.enc"
	}
	previousPath := filepath.Join(backupDir, fmt.Sprintf("%s-daily.%s", ProjectName, previousExt))
	if previousPath != incPath {
		if err := os.Remove(previousPath); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: could not remove replaced daily incremental %s: %v", previousPath, err)
		}
	}
	return true, nil
}

func finalizeBackupPair(fullTemp, fullPath, incTemp, incPath, backupDir string) error {
	return finalizeBackupPairWithRename(fullTemp, fullPath, incTemp, incPath, backupDir, os.Rename)
}

func finalizeBackupPairWithRename(fullTemp, fullPath, incTemp, incPath, backupDir string, rename func(string, string) error) error {
	fullBackup, hadFull, err := stageBackupForReplacement(fullPath, "full", backupDir, rename)
	if err != nil {
		return fmt.Errorf("stage previous full backup: %w", err)
	}
	incBackup, hadIncremental, err := stageBackupForReplacement(incPath, "incremental", backupDir, rename)
	if err != nil {
		rollbackErr := restoreBackupPair(fullPath, fullBackup, hadFull, incPath, incBackup, hadIncremental, false, rename)
		return errors.Join(fmt.Errorf("stage previous daily incremental: %w", err), rollbackErr)
	}
	if err := rename(fullTemp, fullPath); err != nil {
		rollbackErr := restoreBackupPair(fullPath, fullBackup, hadFull, incPath, incBackup, hadIncremental, false, rename)
		return errors.Join(fmt.Errorf("finalize full backup: %w", err), rollbackErr)
	}
	if err := rename(incTemp, incPath); err != nil {
		rollbackErr := restoreBackupPair(fullPath, fullBackup, hadFull, incPath, incBackup, hadIncremental, true, rename)
		return errors.Join(fmt.Errorf("finalize daily incremental: %w", err), rollbackErr)
	}
	removeBackupRollbackFile(fullBackup)
	removeBackupRollbackFile(incBackup)
	return nil
}

func stageBackupForReplacement(destination, label, backupDir string, rename func(string, string) error) (string, bool, error) {
	if _, err := os.Lstat(destination); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	backupPath, err := createTemporaryBackupPath(backupDir, ProjectName+"-rollback-"+label, backupArchiveExtension(destination))
	if err != nil {
		return "", false, err
	}
	if err := rename(destination, backupPath); err != nil {
		cleanupErr := os.Remove(backupPath)
		if cleanupErr != nil && !errors.Is(cleanupErr, os.ErrNotExist) {
			return "", false, errors.Join(err, fmt.Errorf("remove unused rollback path %s: %w", backupPath, cleanupErr))
		}
		return "", false, err
	}
	return backupPath, true, nil
}

func backupArchiveExtension(path string) string {
	if strings.HasSuffix(path, ".tar.gz.enc") {
		return "tar.gz.enc"
	}
	return "tar.gz"
}

func restoreBackupPair(fullPath, fullBackup string, hadFull bool, incPath, incBackup string, hadIncremental, fullInstalled bool, rename func(string, string) error) error {
	var rollbackErrors []error
	canRestoreFull := true
	if fullInstalled {
		if err := os.Remove(fullPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			canRestoreFull = false
			rollbackErrors = append(rollbackErrors, fmt.Errorf("remove newly finalized full backup: %w", err))
		}
	}
	if hadFull && canRestoreFull {
		if err := rename(fullBackup, fullPath); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restore previous full backup from %s: %w", fullBackup, err))
		}
	}
	if hadIncremental {
		if err := rename(incBackup, incPath); err != nil {
			rollbackErrors = append(rollbackErrors, fmt.Errorf("restore previous daily incremental from %s: %w", incBackup, err))
		}
	}
	return errors.Join(rollbackErrors...)
}

func removeBackupRollbackFile(path string) {
	if path == "" {
		return
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("WARN: failed to remove backup rollback file %s: %v", path, err)
	}
}

func createTemporaryBackupPath(directory, prefix, extension string) (string, error) {
	file, err := os.CreateTemp(directory, "."+prefix+"-pending-*"+"."+extension)
	if err != nil {
		return "", err
	}
	path := file.Name()
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	if err := os.Remove(path); err != nil {
		return "", err
	}
	return path, nil
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
	return createIncrementalBackupAt(kind, time.Now().UTC())
}

func createIncrementalBackupAt(kind string, snapshotAt time.Time) error {
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)
	cfg := config.GetApplicationConfig()

	password := cfg.Server.Backup.EncryptionPassword
	if cfg.Server.Compliance.IsEnabled() && password == "" {
		// Same compliance gate as the full backup (PART 21 "Compliance Mode
		// Enforcement"); the full-backup task already warns for this run.
		return nil
	}

	backupDir := resolveBackupDir()
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}
	basePath, baseManifest, err := latestFullBackup(backupDir, password)
	if err != nil {
		return err
	}
	if basePath == "" {
		if kind == "daily" {
			log.Printf("INFO: skipping daily incremental backup: no previous full backup")
		}
		return nil
	}
	if matched, corrupt, err := backupPasswordMatchesBase(basePath, password); err != nil {
		return err
	} else if corrupt {
		log.Printf("WARN: skipping %s incremental backup: previous full backup header is corrupt: %s", kind, filepath.Base(basePath))
		return nil
	} else if !matched {
		log.Printf("INFO: skipping %s incremental backup because backup encryption password changed", kind)
		return nil
	}
	if baseManifest == nil {
		log.Printf("WARN: skipping %s incremental backup because previous full backup is corrupt: %s", kind, filepath.Base(basePath))
		return nil
	}

	ext := "tar.gz"
	if password != "" {
		ext = "tar.gz.enc"
	}
	fileLocation := filepath.Join(backupDir, fmt.Sprintf("%s-%s.%s", ProjectName, kind, ext))
	tempFile, err := os.CreateTemp(backupDir, "."+ProjectName+"-"+kind+"-pending-*"+"."+ext)
	if err != nil {
		return fmt.Errorf("create incremental temporary file: %w", err)
	}
	tempLocation := tempFile.Name()
	if err := tempFile.Close(); err != nil {
		_ = os.Remove(tempLocation)
		return err
	}
	if err := os.Remove(tempLocation); err != nil {
		return err
	}

	if err := backup.CreateBackupArchive(tempLocation, backup.BackupArchiveOptions{
		ConfigDir: configDir,
		DataDir:   dataDir,
		DatabaseSnapshot: func(ctx context.Context, destination string) error {
			return snapshotDatabaseForBackup(ctx, cfg.Server.Database, destination)
		},
		IncludeSSL:         true,
		IncludeData:        true,
		EncryptionPassword: password,
		ComplianceEnabled:  cfg.Server.Compliance.IsEnabled(),
		AppVersion:         Version,
		CreatedBy:          ProjectName,
		BackupType:         "incremental",
		SnapshotAt:         snapshotAt,
		BaseBackup:         filepath.Base(basePath),
		BaseArchivePath:    basePath,
		BaseSHA256:         baseManifest.Checksum,
	}); err != nil {
		_ = os.Remove(tempLocation)
		if sendErr := notify.Send(cfg, configDir, "backup_failed", notifyRecipient(cfg), map[string]string{
			"filename": filepath.Base(fileLocation),
			"error":    err.Error(),
		}); sendErr != nil {
			log.Printf("Warning: failed to send backup_failed notification email: %v", sendErr)
		}
		return fmt.Errorf("%s incremental backup failed: %w", kind, err)
	}

	if err := backup.VerifyWithLimits(tempLocation, password, configuredRestoreLimits(cfg)); err != nil {
		_ = os.Remove(tempLocation)
		if sendErr := notify.Send(cfg, configDir, "backup_failed", notifyRecipient(cfg), map[string]string{
			"filename": filepath.Base(fileLocation),
			"error":    err.Error(),
		}); sendErr != nil {
			log.Printf("Warning: failed to send backup_failed notification email: %v", sendErr)
		}
		return fmt.Errorf("%s incremental backup verification failed, removed: %w", kind, err)
	}
	if err := os.Rename(tempLocation, fileLocation); err != nil {
		_ = os.Remove(tempLocation)
		return fmt.Errorf("%s incremental backup finalize failed: %w", kind, err)
	}
	previousExt := "tar.gz"
	if ext == "tar.gz" {
		previousExt = "tar.gz.enc"
	}
	previousLocation := filepath.Join(backupDir, fmt.Sprintf("%s-%s.%s", ProjectName, kind, previousExt))
	if previousLocation != fileLocation {
		if err := os.Remove(previousLocation); err != nil && !os.IsNotExist(err) {
			log.Printf("Warning: could not remove replaced incremental %s: %v", previousLocation, err)
		}
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

func latestFullBackup(backupDir, password string) (string, *backup.Manifest, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return "", nil, err
	}
	prefix := ProjectName + "_backup_"
	var names []string
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !isBackupArchiveFile(name) {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return "", nil, nil
	}
	sort.Strings(names)
	for i := len(names) - 1; i >= 0; i-- {
		path := filepath.Join(backupDir, names[i])
		manifest, err := backup.ReadManifest(path, password)
		if err == nil {
			if manifest.Type == "full" {
				return path, manifest, nil
			}
			return path, nil, nil
		}
		if strings.HasSuffix(names[i], ".enc") || err != nil {
			return path, nil, nil
		}
	}
	return "", nil, nil
}

func newestBackupBySnapshot(backupDir string) (string, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return "", fmt.Errorf("no backups found in %s: %w", backupDir, err)
	}
	var newestByTime os.FileInfo
	var newestPath string
	var newestManifestPath string
	var newestSnapshot time.Time
	hasEncrypted := false
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !isBackupArchiveFile(name) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		path := filepath.Join(backupDir, name)
		if newestByTime == nil || info.ModTime().After(newestByTime.ModTime()) {
			newestByTime, newestPath = info, path
		}
		if strings.HasSuffix(strings.ToLower(name), ".enc") {
			hasEncrypted = true
			continue
		}
		manifest, err := backup.ReadManifest(path, "")
		if err != nil {
			continue
		}
		snapshot, err := time.Parse(time.RFC3339, manifest.SnapshotAt)
		if err == nil && (newestManifestPath == "" || snapshot.After(newestSnapshot)) {
			newestSnapshot, newestManifestPath = snapshot, path
		}
	}
	if newestPath == "" {
		return "", fmt.Errorf("no backup files found")
	}
	// Encrypted manifests require the operator-supplied password. Select by
	// filesystem modification time when any encrypted archive is present.
	if !hasEncrypted && newestManifestPath != "" {
		return newestManifestPath, nil
	}
	return newestPath, nil
}

func isBackupArchiveFile(name string) bool {
	return strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".tar.gz.enc")
}

func backupPasswordMatchesBase(path, password string) (matched, corrupt bool, err error) {
	if strings.HasSuffix(strings.ToLower(path), ".enc") {
		blob, readErr := os.ReadFile(path)
		if readErr != nil {
			return false, false, readErr
		}
		header, parseErr := backup.ParseEncryptionHeader(blob)
		if parseErr != nil {
			return false, true, nil
		}
		if password == "" {
			return false, false, nil
		}
		matched, matchErr := backup.PasswordMatchesHeader(password, header)
		return matched, false, matchErr
	}
	return password == "", false, nil
}

// backupMaxTotalSizeBytes resolves server.backup.retention.max_total_size
// (percent or absolute string) to a byte count via the backup package's
// parser. Falls back to 0 (disabled) on parse failure.
func backupMaxTotalSizeBytes(cfg *config.ApplicationConfig, backupDir string) (int64, error) {
	value := cfg.Server.Backup.Retention.MaxTotalSize
	if value == "" {
		return 0, nil
	}
	return backup.ParseMaxTotalSize(value, backupDir)
}

func configuredRestoreLimits(cfg *config.ApplicationConfig) backup.RestoreLimits {
	if cfg == nil {
		cfg = config.DefaultApplicationConfig()
	}
	settings := cfg.Server.Restore
	return backup.RestoreLimits{
		MaxArchiveSize:  settings.MaxArchiveBytes(),
		MaxExpandedSize: settings.MaxExpandedBytes(),
		MaxFiles:        settings.MaxFiles,
		MaxFileSize:     settings.MaxFileBytes(),
		MaxRatio:        settings.MaxRatio,
	}
}

func authorizeRestoreWithConfirmation(configDir string, interactive, terminal bool, readInput func(string, int) (string, error)) error {
	dbIsEmpty := true
	dbFile := filepath.Join(paths.GetSQLiteDBPath(ProjectName), "server.db")
	if info, err := os.Stat(dbFile); err == nil && info.Size() > 0 {
		dbIsEmpty = false
	}

	runningAsRoot := os.Geteuid() == 0

	configuredToken := ""
	if !dbIsEmpty && !runningAsRoot {
		cfg, err := config.LoadApplicationConfig(filepath.Join(configDir, "server.yml"))
		if err != nil {
			return fmt.Errorf("restore: cannot read server.yml to authorize: %w", err)
		}
		configuredToken = cfg.Server.Token
	}

	providedToken := ""
	if !dbIsEmpty && !runningAsRoot && configuredToken != "" && interactive {
		var err error
		providedToken, err = promptBackupPassword("Operator token (server.token): ")
		if err != nil {
			return err
		}
	}

	authorization, err := backup.AuthorizeRestore(dbIsEmpty, runningAsRoot, providedToken, configuredToken)
	if err != nil {
		return err
	}
	if authorization == backup.RestoreAllowedRoot && !dbIsEmpty {
		if err := confirmRootRestore(interactive, terminal, readInput); err != nil {
			return err
		}
	}
	return nil
}

func confirmRootRestore(interactive, terminal bool, readInput func(string, int) (string, error)) error {
	if !interactive || !terminal {
		return errors.New("restore as root requires an interactive confirmation")
	}
	answer, err := readInput("This will overwrite all server data. Type RESTORE to confirm: ", 80)
	if err != nil {
		return fmt.Errorf("restore confirmation: %w", err)
	}
	if answer != "RESTORE" {
		return errors.New("restore cancelled")
	}
	return nil
}

// restoreBackup restores from a backup archive via the src/backup package
// (AI.md PART 21). An empty fileLocation resolves to the most recent backup
// in the default backup directory. interactive controls whether a missing
// decryption password is prompted for.
func restoreBackup(fileLocation string, interactive bool) error {
	return restoreBackupWithAuthorizer(fileLocation, interactive, func(configDir string) error {
		return authorizeRestoreWithConfirmation(configDir, interactive, term.IsTerminal(int(os.Stdin.Fd())), readRequiredPGPInput)
	})
}

func restoreBackupWithAuthorizer(fileLocation string, interactive bool, authorize func(string) error) error {
	configDir, dataDir, _ := paths.GetDefaultDirs(ProjectName)

	if fileLocation == "" {
		backupDir := resolveBackupDir()
		var err error
		fileLocation, err = newestBackupBySnapshot(backupDir)
		if err != nil {
			return err
		}
	}

	if authorize == nil {
		return errors.New("restore: authorization is required")
	}
	if err := authorize(configDir); err != nil {
		return err
	}

	fmt.Printf("Restoring from: %s\n", fileLocation)

	password := ""
	if strings.HasSuffix(fileLocation, ".enc") && interactive {
		var err error
		password, err = promptBackupPassword("Backup decryption password: ")
		if err != nil {
			return err
		}
		if password == "" {
			return fmt.Errorf("an encrypted backup requires a password; run restore from a terminal to enter it")
		}
	}

	err := backup.RestoreBackupArchive(fileLocation, backup.BackupRestoreOptions{
		ConfigDir:  configDir,
		DataDir:    dataDir,
		Password:   password,
		AppVersion: Version,
		Limits:     configuredRestoreLimits(config.GetApplicationConfig()),
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

func testBackup(fileLocation string, interactive bool) error {
	if fileLocation == "" {
		backupDir := resolveBackupDir()
		var err error
		fileLocation, err = newestBackupBySnapshot(backupDir)
		if err != nil {
			return err
		}
	}
	password := ""
	if strings.HasSuffix(strings.ToLower(fileLocation), ".enc") {
		if !interactive || !term.IsTerminal(int(os.Stdin.Fd())) {
			return fmt.Errorf("encrypted backup test requires an interactive terminal to enter the password")
		}
		var err error
		password, err = promptBackupPassword("Backup decryption password: ")
		if err != nil {
			return err
		}
	}
	if err := backup.VerifyWithLimits(fileLocation, password, configuredRestoreLimits(config.GetApplicationConfig())); err != nil {
		return fmt.Errorf("backup test failed: %w", err)
	}
	fmt.Printf("Backup verified successfully: %s\n", fileLocation)
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
// stable releases match every channel, beta adds -beta tags, and devel adds
// the rolling devel tag.
func matchesBranch(r releaseInfo, branch string) bool {
	normalized, ok := config.NormalizeUpdateBranch(branch)
	if !ok {
		return false
	}
	if !r.Prerelease {
		return true
	}
	isBeta := strings.HasSuffix(r.TagName, "-beta")
	switch normalized {
	case "beta":
		return isBeta
	case "devel":
		return isBeta || r.TagName == "devel"
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
	defer func() { _ = resp.Body.Close() }()

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
// Semantics". stable uses the GitHub "latest" endpoint directly; beta/devel
// fetch the full releases list (newest-first). Devel compares release publish
// times with this binary's embedded BUILD_EPOCH, as required for rolling tags.
func fetchReleaseForBranch(httpClient *http.Client, branch, currentVersion string) (*releaseInfo, error) {
	if branch == "" {
		branch = "stable"
	}
	var valid bool
	normalized, valid := config.NormalizeUpdateBranch(branch)
	if !valid {
		return nil, fmt.Errorf("invalid update branch %q", branch)
	}
	branch = normalized
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	var releases []releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to parse release list: %w", err)
	}

	for i := range releases {
		r := releases[i]
		if !matchesBranch(r, branch) {
			continue
		}
		if branch == "devel" {
			if r.PublishedAt.After(time.Unix(buildEpoch(), 0).UTC()) {
				return &r, nil
			}
			continue
		}
		if strings.TrimPrefix(r.TagName, "v") != currentVersion {
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
	defaultConfigDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configDir := firstNonEmpty(os.Getenv("CONFIG_DIR"), defaultConfigDir)
	configPath := filepath.Join(configDir, "server.yml")
	cfg, err := config.LoadApplicationConfig(configPath)
	if err != nil || cfg.Server.Update.Branch == "" {
		return "stable"
	}
	branch, ok := config.NormalizeUpdateBranch(cfg.Server.Update.Branch)
	if !ok {
		return "stable"
	}
	return branch
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

// runScheduledUpdateCheckWithClient is the client-injectable core of
// runScheduledUpdateCheck, split out so tests can point release fetching at
// a local httptest.Server instead of the real GitHub API.
func runScheduledUpdateCheckWithClient(cfg *config.ApplicationConfig, configDir string, httpClient *http.Client) error {
	_, dataDir, logsDir := paths.GetDefaultDirs(ProjectName)
	return runScheduledUpdateCheckWithPaths(cfg, configDir, dataDir, logsDir, false, httpClient)
}

func runScheduledUpdateCheckWithPaths(cfg *config.ApplicationConfig, configDir, dataDir, logsDir string, startedElevated bool, httpClient *http.Client) error {
	branch, ok := config.NormalizeUpdateBranch(cfg.Server.Update.Branch)
	if !ok {
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
	if branch != "devel" && deferDays > 0 && !release.PublishedAt.IsZero() {
		if time.Since(release.PublishedAt) < time.Duration(deferDays)*24*time.Hour {
			return nil
		}
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")

	// "Fires once per version" — only log/notify the first time this
	// version becomes eligible, not on every scheduler run.
	if latestVersion != cfg.Server.Update.LastNotifiedVersion {
		log.Printf("WARN: update available: %s (current: %s, channel: %s)", latestVersion, Version, branch)
		manualCommand := ProjectName + " --update yes"
		if runtime.GOOS == "windows" {
			manualCommand += " (run from an elevated prompt)"
		} else if !startedElevated {
			manualCommand = "sudo " + manualCommand
		}
		if sendErr := notify.Send(cfg, configDir, "update_available", notifyRecipient(cfg), map[string]string{
			"current_version": Version,
			"new_version":     latestVersion,
			"channel":         branch,
			"manual_command":  manualCommand,
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
	if !startedElevated || runtime.GOOS == "windows" || isRunningInContainer() {
		reason := "not_elevated"
		if runtime.GOOS == "windows" {
			reason = "windows_service"
		} else if isRunningInContainer() {
			reason = "container"
		}
		if err := writeDataAudit(logsDir, toLoggingConfig(cfg.Server.Logging), "update.install_skipped", "operator", "reason="+strconv.Quote(reason)+" version="+strconv.Quote(latestVersion)); err != nil {
			log.Printf("Warning: could not audit skipped update installation: %v", err)
		}
		return nil
	}
	if err := stageReleaseBinary(httpClient, release, dataDir); err != nil {
		failStagedUpdate(cfg, configDir, logsDir, latestVersion, err)
		return err
	}
	if err := writeDataAudit(logsDir, toLoggingConfig(cfg.Server.Logging), "update.staged", "operator", "version="+strconv.Quote(latestVersion)); err != nil {
		log.Printf("Warning: could not audit staged update: %v", err)
	}
	manager := service.DetectServiceManager()
	manualCommand := stagedUpdateRestartCommand(manager)
	if sendErr := notify.Send(cfg, configDir, "update_staged", notifyRecipient(cfg), map[string]string{
		"new_version":    latestVersion,
		"manual_command": manualCommand,
	}); sendErr != nil {
		log.Printf("Warning: failed to send update_staged notification email: %v", sendErr)
	}
	if manager == service.ServiceSystemd || manager == service.ServiceLaunchd || manager == service.ServiceRunit || manager == service.ServiceS6 {
		select {
		case updateRestartRequest <- struct{}{}:
		default:
		}
	}
	return nil
}

func stagedUpdateRestartCommand(manager service.ServiceType) string {
	switch manager {
	case service.ServiceOpenRC:
		return "rc-service " + ProjectName + " restart"
	case service.ServiceSysVinit, service.ServiceBSDRC:
		return "service " + ProjectName + " restart"
	default:
		return ProjectName + " --service restart"
	}
}

// setUpdateBranch implements `--update branch {stable|beta|devel}`: writes
// the update channel to server.yml, following the load/mutate/save pattern
// required for this persisted update setting.
func setUpdateBranch(branch string) error {
	normalized, valid := config.NormalizeUpdateBranch(branch)
	if !valid {
		return fmt.Errorf("invalid update branch: %q (expected stable, beta, or devel)", branch)
	}
	branch = normalized

	defaultConfigDir, _, _ := paths.GetDefaultDirs(ProjectName)
	configDir := firstNonEmpty(os.Getenv("CONFIG_DIR"), defaultConfigDir)
	configPath := filepath.Join(configDir, "server.yml")

	if _, err := config.LoadApplicationConfig(configPath); err != nil {
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
		defer func() { _ = csResp.Body.Close() }()
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

	// Download into the project-scoped OS temp namespace so maintenance can
	// reclaim interrupted update artifacts after they age beyond 24 hours.
	tempOrgDir := filepath.Join(os.TempDir(), paths.OrgName)
	if err := os.MkdirAll(tempOrgDir, 0o700); err != nil {
		return fmt.Errorf("failed to create project temp namespace: %w", err)
	}
	tempDir, err := os.MkdirTemp(tempOrgDir, ProjectName+"-")
	if err != nil {
		return fmt.Errorf("failed to create update temp directory: %w", err)
	}
	defer func() { _ = os.Remove(tempDir) }()
	tmpFile, err := os.CreateTemp(tempDir, "update-*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	dlResp, err := httpClient.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("failed to download update: %w", err)
	}
	defer func() { _ = dlResp.Body.Close() }()

	// Hash while writing; cap at 256 MiB to guard against unbounded streams.
	const maxBinarySize = 256 << 20
	hasher := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmpFile, hasher), io.LimitReader(dlResp.Body, maxBinarySize)); err != nil {
		return fmt.Errorf("failed to save update: %w", err)
	}
	_ = tmpFile.Close()

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
	if err := os.Chmod(tmpFile.Name(), 0o755); err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	if err := replaceBinary(currentPath, tmpFile.Name()); err != nil {
		return fmt.Errorf("failed to replace binary: %w", err)
	}

	fmt.Printf("Updated to version %s\n", latestVersion)

	// Attempt to restart via service manager (PART 22).
	if err := service.RestartSystemService(); err != nil {
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

func findRandomPort() (string, error) {
	// Go 1.20+ auto-seeds the global rand; no explicit Seed needed.
	for i := 0; i < 100; i++ {
		port := 64000 + rand.Intn(1000)
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err == nil {
			_ = ln.Close()
			return fmt.Sprintf("%d", port), nil
		}
	}

	return "", fmt.Errorf("could not find available port in range 64000-64999")
}

type portBinding struct {
	Port string
	TLS  bool
}

func joinListenAddress(address, port string) string {
	if strings.HasPrefix(address, "[") && strings.HasSuffix(address, "]") {
		address = strings.TrimSuffix(strings.TrimPrefix(address, "["), "]")
	}
	return net.JoinHostPort(address, port)
}

// parsePortSpec parses the documented single- and dual-port forms. A single
// port is HTTP unless it is 443 or SSL is enabled; a pair is HTTP,HTTPS.
func parsePortSpec(value string, sslEnabled bool) ([]portBinding, error) {
	parts := strings.Split(strings.TrimSpace(value), ",")
	if len(parts) < 1 || len(parts) > 2 {
		return nil, fmt.Errorf("expected PORT or HTTP_PORT,HTTPS_PORT")
	}
	ports := make([]string, len(parts))
	for i, part := range parts {
		part = strings.TrimSpace(part)
		port, err := strconv.Atoi(part)
		if err != nil || port < 0 || port > 65535 {
			return nil, fmt.Errorf("port %q must be an integer from 0 to 65535", part)
		}
		ports[i] = strconv.Itoa(port)
	}
	if len(ports) == 2 {
		if ports[0] == "0" || ports[1] == "0" {
			return nil, fmt.Errorf("dual-port mode requires explicit nonzero ports")
		}
		if ports[0] == ports[1] {
			return nil, fmt.Errorf("HTTP and HTTPS ports must be different")
		}
		if !sslEnabled {
			return nil, fmt.Errorf("dual-port mode requires server.ssl.enabled")
		}
		return []portBinding{{Port: ports[0]}, {Port: ports[1], TLS: true}}, nil
	}
	return []portBinding{{Port: ports[0], TLS: ports[0] == "443" || sslEnabled}}, nil
}

type retainedListener struct{ net.Listener }

func (retainedListener) Close() error { return nil }

type startupRouteSwitcher struct {
	mu      sync.RWMutex
	handler http.Handler
}

func serveHTTPServer(ctx context.Context, srv *http.Server, listener net.Listener, useTLS bool, ready chan<- struct{}) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if ready != nil {
		close(ready)
	}
	serveResult := make(chan error, 1)
	go func() {
		if useTLS {
			serveResult <- srv.ServeTLS(listener, "", "")
			return
		}
		serveResult <- srv.Serve(listener)
	}()
	select {
	case err := <-serveResult:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		shutdownErr := srv.Shutdown(shutdownCtx)
		serveErr := <-serveResult
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return serveErr
		}
		return shutdownErr
	}
}

func (s *startupRouteSwitcher) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()
	if handler == nil {
		http.Error(w, "service starting", http.StatusServiceUnavailable)
		return
	}
	handler.ServeHTTP(w, r)
}

func (s *startupRouteSwitcher) Set(handler http.Handler) {
	s.mu.Lock()
	s.handler = handler
	s.mu.Unlock()
}

func initializeStartupDatabase(database db.Handle, cfg *config.ApplicationConfig, configDir string) error {
	if err := db.EnsureAppSecrets(database); err != nil {
		return fmt.Errorf("database write: initialize application secrets: %w", err)
	}
	if err := applyPGPRotationJournal(database, configDir); err != nil {
		return fmt.Errorf("database write: resume PGP key rotation: %w", err)
	}
	if err := resumeSecretRotations(database, cfg, configDir); err != nil {
		return fmt.Errorf("database write: resume secret rotation: %w", err)
	}
	if err := scheduler.EnsureSchema(database); err != nil {
		return fmt.Errorf("database write: initialize scheduler schema: %w", err)
	}
	return nil
}

// waitForStartupDatabaseRecovery keeps only the canonical health page/assets
// reachable while database open and initialization steps are retried.
func waitForStartupDatabaseRecovery(ctx context.Context, cfg *config.ApplicationConfig, configDir, dataDir, address string, bindings []portBinding, listeners []net.Listener, airportSvc *airports.AirportDataService, geoipSvc *geoip.GeoIPService, logManager *logging.LogManager, initialErr error) (db.Handle, []*http.Server, []*startupRouteSwitcher, error) {
	interval, err := time.ParseDuration(cfg.Server.Maintenance.SelfHealing.RetryInterval)
	if err != nil || interval <= 0 {
		interval = 30 * time.Second
	}
	maxAttempts := cfg.Server.Maintenance.SelfHealing.MaxAttempts
	if maxAttempts < 0 {
		maxAttempts = 0
	}
	since := time.Now().UTC()
	reason := "database_connection"
	if initialErr != nil && strings.Contains(strings.ToLower(initialErr.Error()), "database write") {
		reason = "database_write"
	}
	databaseError := "unknown database startup error"
	if initialErr != nil {
		databaseError = initialErr.Error()
	}
	if cfg.Server.Database.Token != "" {
		databaseError = strings.ReplaceAll(databaseError, cfg.Server.Database.Token, "xxxxx")
	}
	if cfg.Server.Database.URL != "" {
		databaseError = strings.ReplaceAll(databaseError, cfg.Server.Database.URL, "database URL")
	}
	log.Printf("Database startup failed (%s): %s; entering read-only maintenance mode and retrying every %s", reason, databaseError, interval)
	if notify.CanSend(cfg) {
		vars := notify.GlobalVars(cfg)
		vars["error"] = databaseError
		vars["since"] = since.Format(time.RFC1123)
		if sendErr := notify.Send(cfg, configDir, "database_error", notifyRecipient(cfg), vars); sendErr != nil {
			log.Printf("Warning: failed to send database error notification: %v", sendErr)
		}
	}

	server.SetHealthMaintenance(&server.MaintenanceHealthInfo{Reason: reason, Since: since, SelfHealing: server.SelfHealingHealthStatus{Enabled: true}})
	maintenanceServer := server.NewServer(airportSvc, geoipSvc, cfg, nil, nil, dataDir, configDir, nil)
	defer func() {
		if closeErr := maintenanceServer.CloseCache(); closeErr != nil {
			log.Printf("Warning: failed to close maintenance cache: %v", closeErr)
		}
	}()
	serverConfig := make([]*http.Server, len(bindings))
	switchers := make([]*startupRouteSwitcher, len(bindings))
	for i, binding := range bindings {
		if listeners[i] == nil {
			listeners[i], err = net.Listen("tcp", joinListenAddress(address, binding.Port))
			if err != nil {
				return nil, nil, nil, fmt.Errorf("maintenance listener bind failed: %w", err)
			}
		}
		switchers[i] = &startupRouteSwitcher{handler: server.StartupMaintenanceHandler(maintenanceServer)}
		serverConfig[i] = &http.Server{Handler: switchers[i], ReadTimeout: cfg.Server.Limits.ReadTimeoutDuration(), WriteTimeout: cfg.Server.Limits.WriteTimeoutDuration(), IdleTimeout: cfg.Server.Limits.IdleTimeoutDuration()}
		if binding.TLS {
			manager := ssl.NewSSLManager(toSSLConfig(cfg.Server.SSL, cfg.Server.Security.EncryptionKey, paths.GetSSLDir(ProjectName)))
			serverConfig[i].TLSConfig, err = manager.GetTLSConfig([]string{cfg.Server.FQDN})
			if err != nil {
				log.Printf("Warning: maintenance HTTPS configuration unavailable: %v", err)
			}
		}
		listener := retainedListener{Listener: listeners[i]}
		go func(srv *http.Server, ln net.Listener, useTLS bool) {
			var serveErr error
			if useTLS {
				serveErr = srv.ServeTLS(ln, "", "")
			} else {
				serveErr = srv.Serve(ln)
			}
			if serveErr != nil && serveErr != http.ErrServerClosed {
				log.Printf("Maintenance listener stopped unexpectedly: %v", serveErr)
			}
		}(serverConfig[i], listener, binding.TLS)
	}

	quit := make(chan os.Signal, 1)
	ossignal.Notify(quit, appsignal.ShutdownSignals()...)
	defer ossignal.Stop(quit)
	attempts := 0
	exhaustionNotified := false
	for {
		if maxAttempts == 0 || attempts < maxAttempts {
			attempts++
			now := time.Now().UTC()
			next := now.Add(interval)
			server.SetHealthMaintenance(&server.MaintenanceHealthInfo{Reason: reason, Since: since, SelfHealing: server.SelfHealingHealthStatus{Enabled: true, Attempts: attempts, LastAttempt: &now, NextAttempt: &next}})
			candidate, openErr := db.OpenStore(cfg.Server.Database, ProjectName)
			if openErr == nil {
				openErr = initializeStartupDatabase(candidate, cfg, configDir)
			}
			if openErr == nil {
				log.Printf("Database startup recovery succeeded after %d attempt(s); resuming normal initialization", attempts)
				if notify.CanSend(cfg) {
					if sendErr := notify.Send(cfg, configDir, "maintenance_recovered", notifyRecipient(cfg), map[string]string{"reason": reason, "duration": time.Since(since).String()}); sendErr != nil {
						log.Printf("Warning: failed to send maintenance recovery notification: %v", sendErr)
					}
				}
				return candidate, serverConfig, switchers, nil
			}
			if candidate != nil {
				_ = candidate.Close()
			}
			log.Printf("Database startup recovery attempt %d failed; retrying in %s", attempts, interval)
			if maxAttempts > 0 && attempts >= maxAttempts {
				lastAttempt := now
				server.SetHealthMaintenance(&server.MaintenanceHealthInfo{Reason: reason, Since: since, SelfHealing: server.SelfHealingHealthStatus{Enabled: false, Exhausted: true, Attempts: attempts, LastAttempt: &lastAttempt, NextAttempt: nil}})
				log.Printf("Database startup recovery reached configured max_attempts=%d; maintenance mode remains active", maxAttempts)
				if logManager != nil {
					if auditErr := logManager.WriteAudit(logging.AuditCategoryServer, fmt.Sprintf("event=server.self_healing_exhausted reason=%q attempts=%d", reason, attempts)); auditErr != nil {
						log.Printf("Warning: failed to audit exhausted startup self-healing: %v", auditErr)
					}
				}
				if !exhaustionNotified && notify.CanSend(cfg) {
					exhaustionNotified = true
					if sendErr := notify.Send(cfg, configDir, "maintenance_exhausted", notifyRecipient(cfg), map[string]string{"reason": reason, "attempts": strconv.Itoa(attempts)}); sendErr != nil {
						log.Printf("Warning: failed to send maintenance exhaustion notification: %v", sendErr)
					}
				}
			}
		}
		timer := time.NewTimer(interval)
		select {
		case <-quit:
			timer.Stop()
			for _, srv := range serverConfig {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = srv.Shutdown(ctx)
				cancel()
			}
			return nil, nil, nil, initialErr
		case <-ctx.Done():
			timer.Stop()
			for _, srv := range serverConfig {
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				_ = srv.Shutdown(shutdownCtx)
				cancel()
			}
			return nil, nil, nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func httpsPortFromBindings(bindings []portBinding) string {
	for _, binding := range bindings {
		if binding.TLS {
			return binding.Port
		}
	}
	return ""
}

// redirectHTTPToHTTPS redirects clearnet requests to the configured HTTPS
// listener. The SSL manager wraps this handler so ACME HTTP-01 challenges
// are served before the redirect.
func redirectHTTPToHTTPS(next http.Handler, fqdn, httpsPort string, isTrustedProxy func(*http.Request) bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		trustedHTTPS := isTrustedProxy != nil && isTrustedProxy(r) && urlvars.ResolveProto(r, true) == "https"
		if r.TLS != nil || trustedHTTPS || isOverlayOrHealthRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		host := fqdn
		if host == "" {
			host = r.Host
		}
		if parsedHost, _, err := net.SplitHostPort(host); err == nil {
			host = parsedHost
		}
		host = strings.Trim(host, "[]")
		authority := host
		if httpsPort != "443" {
			authority = net.JoinHostPort(host, httpsPort)
		} else if ip := net.ParseIP(host); ip != nil && ip.To4() == nil {
			authority = "[" + ip.String() + "]"
		}
		target := "https://" + authority + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}

func isOverlayOrHealthRequest(r *http.Request) bool {
	requestPath := strings.TrimSuffix(strings.ToLower(r.URL.Path), ".txt")
	if requestPath == "/healthz" || requestPath == "/api/healthz" || strings.HasSuffix(requestPath, "/server/healthz") {
		return true
	}
	host := strings.ToLower(r.Host)
	if parsedHost, _, err := net.SplitHostPort(host); err == nil {
		host = parsedHost
	}
	host = strings.Trim(host, "[]")
	return strings.HasSuffix(host, ".onion") || strings.HasSuffix(host, ".b32.i2p")
}

func getAccessibleURL(port string) string {
	return getServerAccessibleURL(port, "", false)
}

func getServerAccessibleURL(port, configuredHost string, secure bool) string {
	host := configuredHost
	if host == "" {
		host = resolveFQDN()
	}
	if host == "localhost" {
		if ip := getGlobalInterfaceIP(true); ip != "" {
			host = ip
		} else if ip := getGlobalInterfaceIP(false); ip != "" {
			host = ip
		}
	}
	proto := "http"
	if secure || port == "443" {
		proto = "https"
	}
	if port == "80" || port == "443" {
		port = ""
	}
	urlHost := host
	if ip := net.ParseIP(strings.Trim(host, "[]")); ip != nil && ip.To4() == nil {
		urlHost = "[" + ip.String() + "]"
	}
	if port == "" {
		return fmt.Sprintf("%s://%s", proto, urlHost)
	}
	return fmt.Sprintf("%s://%s:%s", proto, urlHost, port)
}

func getOutboundIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", 2*time.Second)
	if err != nil {
		return ""
	}
	defer func() { _ = conn.Close() }()

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
		tz = "America/New_York"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Printf("Warning: invalid TZ value %q: %v — using system default timezone", tz, err)
		return
	}
	time.Local = loc
}

// resolveFQDN follows AI.md PART 8's static fallback order. Request-aware
// proxy headers are resolved separately by server.publicBaseURL.
func resolveFQDN() string {
	return resolveFQDNWithMode(isDevelopmentMode(""))
}

func isDevelopmentMode(modeValue string) bool {
	if modeValue == "" {
		modeValue = os.Getenv("MODE")
	}
	parsed, err := mode.ParseAppMode(modeValue)
	return err == nil && parsed == mode.Development
}

func resolveFQDNWithMode(devMode bool) string {
	hostname, _ := os.Hostname()
	return resolveFQDNFromSources(
		os.Getenv("DOMAIN"),
		hostname,
		os.Getenv("HOSTNAME"),
		getGlobalInterfaceIP(true),
		getGlobalInterfaceIP(false),
		devMode,
	)
}

// resolveFQDNFromSources keeps the precedence rules independently testable.
func resolveFQDNFromSources(domainEnv, systemHostname, hostEnv, publicIPv6, publicIPv4 string, devMode bool) string {
	for _, candidate := range strings.Split(domainEnv, ",") {
		candidate = strings.TrimSpace(candidate)
		if !strings.HasSuffix(strings.ToLower(candidate), ".onion") && !strings.HasSuffix(strings.ToLower(candidate), ".i2p") && !strings.HasSuffix(strings.ToLower(candidate), ".exit") && urlvars.IsValidHost(candidate, devMode, ProjectName) {
			return candidate
		}
	}
	for _, candidate := range []string{systemHostname, hostEnv, publicIPv6, publicIPv4} {
		candidate = strings.TrimSpace(candidate)
		ip := net.ParseIP(strings.Trim(candidate, "[]"))
		isConfiguredPublicIP := (candidate == strings.TrimSpace(publicIPv6) || candidate == strings.TrimSpace(publicIPv4)) && urlvars.IsPublicIP(ip)
		if candidate != "" && !isLoopbackHostname(candidate) && (urlvars.IsValidHost(candidate, devMode, ProjectName) || isConfiguredPublicIP) {
			return candidate
		}
	}
	return "localhost"
}

func isLoopbackHostname(host string) bool {
	host = strings.Trim(strings.ToLower(host), "[]")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// getGlobalInterfaceIP returns a globally routable interface address,
// preferring IPv6 before IPv4 at the caller.
func getGlobalInterfaceIP(wantIPv6 bool) string {
	interfaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			var ip net.IP
			switch value := address.(type) {
			case *net.IPNet:
				ip = value.IP
			case *net.IPAddr:
				ip = value.IP
			}
			if ip == nil || ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() || !ip.IsGlobalUnicast() {
				continue
			}
			isIPv6 := ip.To4() == nil
			if isIPv6 != wantIPv6 {
				continue
			}
			if !isIPv6 && ip[0] == 100 && ip[1]&0xc0 == 64 {
				continue
			}
			return ip.String()
		}
	}
	return ""
}

// resolveSchedulerLocation parses server.scheduler.timezone (AI.md PART 18)
// and falls back to the specified default when it is empty or invalid.
func resolveSchedulerLocation(timezone string) *time.Location {
	const defaultTimezone = "America/New_York"
	if timezone != "" {
		if loc, err := time.LoadLocation(timezone); err == nil {
			return loc
		} else {
			log.Printf("Warning: invalid server.scheduler.timezone %q: %v — using default %q", timezone, err, defaultTimezone)
		}
	} else {
		return loadDefaultSchedulerLocation(defaultTimezone)
	}
	return loadDefaultSchedulerLocation(defaultTimezone)
}

func loadDefaultSchedulerLocation(defaultTimezone string) *time.Location {
	loc, err := time.LoadLocation(defaultTimezone)
	if err == nil {
		return loc
	}
	log.Printf("Warning: could not load scheduler default timezone %q: %v — using UTC", defaultTimezone, err)
	return time.UTC
}

// colorSetting holds the validated --color CLI flag value (auto/yes/no) for
// the startup banner's color priority.
var (
	colorSetting       = "auto"
	outputEmojiSetting *bool
)

// resolveColorSetting validates the --color flag value (auto/yes/no,
// case-sensitive per AI.md PART 8), defaults an empty value to "auto",
// and stores the result for startup banner color selection.
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

// emojiEnabled delegates to the shared AI.md PART 7 priority helper.
func emojiEnabled() bool {
	return display.EmojiEnabled(outputEmojiSetting)
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

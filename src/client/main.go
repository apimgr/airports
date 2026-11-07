// airports-cli: command-line client for the airports API.
//
// This is the REQUIRED client binary per AI.md spec. It queries a configured
// Airports server and prints results to
// stdout. It does NOT bundle any airport data of its own; the server is
// the source of truth.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/airports/src/client/gui"
	"github.com/apimgr/airports/src/client/tui"
	"github.com/apimgr/airports/src/common/display"
	"github.com/apimgr/airports/src/common/i18n"
	commonversion "github.com/apimgr/airports/src/common/version"
	"github.com/apimgr/airports/src/config"
	"github.com/apimgr/airports/src/mode"
	"github.com/apimgr/airports/src/shellcompl"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Injected at build time via ldflags.
var (
	Version  = "dev"
	CommitID = "unknown"
	// BuildDate is derived from BuildEpoch in init(); "unknown" when BuildEpoch is unset
	BuildDate = "unknown"
	// BuildEpoch is the Unix build timestamp (seconds, UTC) set via -ldflags; "0" when unset
	BuildEpoch   = "0"
	OfficialSite = ""
)

// init derives BuildDate (RFC 3339 UTC) from the embedded BuildEpoch.
func init() {
	BuildDate = commonversion.BuildDateFromEpoch(BuildEpoch, BuildDate)
}

const (
	defaultBaseURL    = ""
	defaultAPIVersion = "v1"
	requestTimeout    = 30 * time.Second
)

// cliShellCommands and cliShellFlags feed shellcompl's generators (PART 32
// "Shell Completions") so completion scripts stay in sync with the
// subcommands/flags actually implemented below.
var (
	cliShellCommands = []string{"search", "get", "nearby", "health", "version", "shell"}
	cliShellFlags    = []string{
		"--server", "--api-version", "--format", "--output", "--json", "--yaml", "--version", "-v",
		"--help", "-h", "--token", "--config", "--color", "--lang", "--debug", "--shell",
		"--update",
	}
)

// honorNoColor returns true when output should suppress ANSI color codes,
// per the NO_COLOR convention (https://no-color.org/) and the spec rule
// that all binaries respect NO_COLOR.
func honorNoColor() bool {
	return os.Getenv("NO_COLOR") != ""
}

// cliConfig holds the persisted settings for airports-cli. Every flag that
// makes sense to persist gets a matching field here (PART 32: "every CLI
// flag configurable via cli.yml with a documented default").
type cliConfig struct {
	Server  cliServerConfig `yaml:"server"`
	Display struct {
		Mode string `yaml:"mode"`
	} `yaml:"display,omitempty"`
	Auth cliAuthConfig `yaml:"auth,omitempty"`
	// Token reads the legacy root-level setting and is omitted when saved.
	Token  string          `yaml:"token,omitempty"`
	Output cliOutputConfig `yaml:"output,omitempty"`
	Update cliUpdateConfig `yaml:"update,omitempty"`
	// Color reads the legacy root-level config key and is omitted when saved.
	Color string `yaml:"color,omitempty"`
	Lang  string `yaml:"lang"`
}

type cliServerConfig struct {
	Primary    string `yaml:"primary"`
	APIVersion string `yaml:"api_version"`
	Timeout    string `yaml:"timeout"`
	Retry      int    `yaml:"retry"`
	RetryDelay string `yaml:"retry_delay"`
}

type cliAuthConfig struct {
	Token     string `yaml:"token,omitempty"`
	TokenFile string `yaml:"token_file,omitempty"`
}

type cliUpdateConfig struct {
	Auto          bool   `yaml:"auto"`
	CheckInterval string `yaml:"check_interval"`
	Channel       string `yaml:"channel"`
}

type cliOutputConfig struct {
	Color  cliColorValue `yaml:"color,omitempty"`
	Emoji  *bool         `yaml:"emoji,omitempty"`
	Format string        `yaml:"format,omitempty"`
}

type cliColorValue string

func (c *cliColorValue) UnmarshalYAML(node *yaml.Node) error {
	switch node.Tag {
	case "!!null":
		*c = ""
		return nil
	case "!!bool":
		var enabled bool
		if err := node.Decode(&enabled); err != nil {
			return err
		}
		if enabled {
			*c = "yes"
		} else {
			*c = "no"
		}
		return nil
	case "!!str":
		value := node.Value
		switch value {
		case "", "auto", "yes", "no":
			*c = cliColorValue(value)
			return nil
		default:
			return fmt.Errorf("invalid output.color value %q (must be auto, yes, or no)", value)
		}
	default:
		return fmt.Errorf("invalid output.color YAML type %q", node.Tag)
	}
}

func (c cliColorValue) MarshalYAML() (interface{}, error) {
	return string(c), nil
}

// cliConfigDir returns the OS-appropriate directory for CLI config profiles.
// Precedence: $XDG_CONFIG_HOME/apimgr/airports (Linux/macOS XDG standard),
// then $HOME/.config/apimgr/airports.
func cliConfigDir() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "apimgr", "airports"), nil
}

// cliConfigPath resolves a --config NAME value to an on-disk path. Per PART
// 32: absolute paths are honored as-is; a bare name (e.g. "test") resolves
// to "test.yml" under the config dir, auto-detecting ".yml" before ".yaml"
// when both are absent; an empty name defaults to "cli".
func cliConfigPath(name string) (string, error) {
	if name != "" && filepath.IsAbs(name) {
		return name, nil
	}

	dir, err := cliConfigDir()
	if err != nil {
		return "", err
	}

	if name == "" {
		name = "cli"
	}

	if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
		return filepath.Join(dir, name), nil
	}

	ymlPath := filepath.Join(dir, name+".yml")
	if _, statErr := os.Stat(ymlPath); statErr == nil {
		return ymlPath, nil
	}
	yamlPath := filepath.Join(dir, name+".yaml")
	if _, statErr := os.Stat(yamlPath); statErr == nil {
		return yamlPath, nil
	}
	// Neither exists yet — default to .yml for a new profile.
	return ymlPath, nil
}

// loadCLIConfig reads the CLI config file for the given profile name if it
// exists. Returns an empty config (not an error) when the file is absent.
func loadCLIConfig(name string) (*cliConfig, error) {
	path, err := cliConfigPath(name)
	if err != nil {
		cfg := &cliConfig{}
		applyCLIDefaults(cfg)
		return cfg, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := &cliConfig{}
		applyCLIDefaults(cfg)
		return cfg, nil
	}
	if err != nil {
		return &cliConfig{}, fmt.Errorf("read cli config: %w", err)
	}
	var cfg cliConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &cliConfig{}, fmt.Errorf("parse cli config: %w", err)
	}
	if cfg.Output.Color == "" {
		cfg.Output.Color = cliColorValue(cfg.Color)
	}
	if cfg.Auth.Token != "" {
		cfg.Token = cfg.Auth.Token
	} else {
		cfg.Auth.Token = cfg.Token
	}
	originalChannel := cfg.Update.Channel
	applyCLIDefaults(&cfg)
	if originalChannel != "" && originalChannel != cfg.Update.Channel {
		if err := saveCLIConfig(&cfg, name); err != nil {
			return &cliConfig{}, fmt.Errorf("normalize CLI update channel: %w", err)
		}
	}
	return &cfg, nil
}

func applyCLIDefaults(cfg *cliConfig) {
	if cfg.Server.APIVersion == "" {
		cfg.Server.APIVersion = defaultAPIVersion
	}
	if cfg.Server.Timeout == "" {
		cfg.Server.Timeout = requestTimeout.String()
	}
	if cfg.Server.Retry < 0 {
		cfg.Server.Retry = 0
	}
	if cfg.Server.Retry == 0 {
		cfg.Server.Retry = 3
	}
	if cfg.Server.RetryDelay == "" {
		cfg.Server.RetryDelay = time.Second.String()
	}
	if cfg.Update.CheckInterval == "" {
		cfg.Update.CheckInterval = "per_invocation"
	}
	if channel, ok := config.NormalizeUpdateBranch(cfg.Update.Channel); ok {
		cfg.Update.Channel = channel
	} else {
		cfg.Update.Channel = "stable"
	}
}

func cliConfigToken(cfg *cliConfig) (string, error) {
	if cfg == nil {
		return "", nil
	}
	if cfg.Auth.Token != "" {
		return cfg.Auth.Token, nil
	}
	if cfg.Token != "" {
		return cfg.Token, nil
	}
	if cfg.Auth.TokenFile == "" {
		return "", nil
	}
	data, err := os.ReadFile(filepath.Clean(cfg.Auth.TokenFile))
	if err != nil {
		return "", fmt.Errorf("read API token file: %w", err)
	}
	token := strings.TrimSpace(string(data))
	if token == "" {
		return "", errors.New("API token file is empty")
	}
	return token, nil
}

// saveCLIConfig writes the CLI config for the given profile name to disk,
// creating directories as needed.
func saveCLIConfig(cfg *cliConfig, name string) error {
	path, err := cliConfigPath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	canonical := *cfg
	if canonical.Auth.Token == "" {
		canonical.Auth.Token = canonical.Token
	}
	canonical.Token = ""
	if canonical.Output.Color == "" {
		canonical.Output.Color = cliColorValue(canonical.Color)
	}
	canonical.Color = ""
	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(&canonical); err != nil {
		return fmt.Errorf("marshal cli config: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return fmt.Errorf("close cli config encoder: %w", err)
	}
	return os.WriteFile(path, output.Bytes(), 0600)
}

func colorSettingOverride(value string) *bool {
	switch value {
	case "yes":
		setting := true
		return &setting
	case "no":
		setting := false
		return &setting
	default:
		return nil
	}
}

func cliConfigColor(cfg *cliConfig) *bool {
	if cfg == nil {
		return nil
	}
	if cfg.Output.Color != "" {
		return colorSettingOverride(string(cfg.Output.Color))
	}
	return colorSettingOverride(cfg.Color)
}

// isTTY reports whether fd is an interactive terminal (not a pipe/redirect).
func isTTY(fd *os.File) bool {
	fi, err := fd.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// runFirstRunWizard configures a server only after a successful health check.
func runFirstRunWizard(defaultURL, configName string) string {
	cfg, err := loadCLIConfig(configName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read CLI configuration: %v\n", err)
		return defaultURL
	}
	if cfg.Server.Primary != "" {
		return cfg.Server.Primary
	}
	if !isTTY(os.Stdin) || !isTTY(os.Stderr) {
		return defaultURL
	}

	lang := resolveLang(cfg.Lang)
	fmt.Fprintln(os.Stderr, i18n.T(lang, "cli.first_run_title"))
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Fprint(os.Stderr, "Server URL: ")
		if !scanner.Scan() {
			return ""
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			if defaultURL == "" {
				fmt.Fprintln(os.Stderr, "A server URL is required.")
				continue
			}
			input = defaultURL
		}
		if err := validateServerURL(input); err != nil {
			fmt.Fprintf(os.Stderr, "Invalid server URL: %v\n", err)
			continue
		}
		if err := testServerConnection(input); err != nil {
			fmt.Fprintf(os.Stderr, "Connection test failed: %v\n", err)
			continue
		}
		cfg.Server.Primary = strings.TrimRight(input, "/")
		if saveErr := saveCLIConfig(cfg, configName); saveErr != nil {
			fmt.Fprintln(os.Stderr, i18n.Tf(lang, "cli.config_save_warning", "error", saveErr))
		} else {
			configPath, _ := cliConfigPath(configName)
			fmt.Fprintln(os.Stderr, i18n.Tf(lang, "cli.config_saved", "path", configPath))
		}
		return cfg.Server.Primary
	}
}

func validateServerURL(raw string) error {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(raw))
	if err != nil {
		return err
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("scheme must be http or https")
	}
	if parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("URL must include a host and cannot contain credentials, query, or fragment")
	}
	return nil
}

func testServerConnection(base string) error {
	if err := validateServerURL(base); err != nil {
		return err
	}
	parsed, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		return err
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/") + "/server/healthz"
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-ID", uuid.NewString())
	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("server health endpoint returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// resolveLang applies the CLI's --lang > config > LANG/LC_ALL > en fallback
// chain (PART 32) and silently falls back to English for any unsupported or
// empty value (PART 30: "missing key / unsupported lang" must never error).
func resolveLang(raw string) string {
	return i18n.ResolveOutputLanguage(raw, "auto")
}

// validColor reports whether v is one of the three spec-allowed --color
// values (PART 32: "must support exactly the three values auto/yes/no").
func validColor(v string) bool {
	switch v {
	case "auto", "yes", "no":
		return true
	default:
		return false
	}
}

// selectDisplayMode applies the CLI's command-over-environment priority.
// An explicit command always remains scriptable in headless environments;
// otherwise the shared detector selects GUI, TUI, CLI, or headless behavior.
func selectDisplayMode(env display.DisplayEnv, args []string) display.DisplayMode {
	if len(args) > 0 {
		return display.DisplayModeCLI
	}
	return env.Mode
}

func applyDisplayConfig(env display.DisplayEnv, detected display.DisplayMode, configured string) (display.DisplayMode, error) {
	if env.IsDumbTerminal() {
		return display.DisplayModeCLI, nil
	}
	switch strings.ToLower(strings.TrimSpace(configured)) {
	case "", "auto":
		return detected, nil
	case "gui":
		if !env.HasDisplay {
			return display.DisplayModeHeadless, errors.New("display.mode=gui requires a local graphical display")
		}
		return display.DisplayModeGUI, nil
	case "tui":
		if !env.IsTerminal {
			return display.DisplayModeHeadless, errors.New("display.mode=tui requires an interactive terminal")
		}
		return display.DisplayModeTUI, nil
	default:
		return display.DisplayModeHeadless, fmt.Errorf("invalid display.mode %q (must be auto, gui, or tui)", configured)
	}
}

func main() {
	// PART 7/32: --help/--version must show the actual invoked binary name
	// (filepath.Base(os.Args[0])), never a hardcoded literal — the internal
	// project name stays hardcoded only for User-Agent/config paths/DB ids.
	binaryName := filepath.Base(os.Args[0])

	// Suppress flag's default error output so we control formatting.
	flag.CommandLine.SetOutput(io.Discard)

	// --config NAME must be resolved before anything else touches disk, since
	// it selects which profile the first-run wizard and every other flag's
	// SaveIfEmptyOrInvalid persistence writes to/reads from.
	configName := extractConfigFlag(os.Args[1:])

	// --shell {completions|init|help} [SHELL] is a value/subcommand flag
	// (PART 32 "Shell Completions") the stdlib flag package can't express, so
	// it is extracted and handled before the first-run wizard or flag.Parse
	// ever run — matching the exit-immediately behavior of --help/--version.
	if shellFound, shellSubcmd, shellName, _ := shellcompl.Extract(os.Args[1:]); shellFound {
		os.Exit(shellcompl.Handle(os.Stdout, os.Stderr, shellSubcmd, shellName, binaryName, cliShellCommands, cliShellFlags))
	}

	savedCfg, _ := loadCLIConfig(configName)
	configuredToken, tokenErr := cliConfigToken(savedCfg)
	if tokenErr != nil {
		fmt.Fprintf(os.Stderr, "cannot load configured API token: %v\n", tokenErr)
		os.Exit(2)
	}
	resolvedBaseURL := firstNonEmptyStr(envOr("AIRPORTS_SERVER_PRIMARY", ""), savedCfg.Server.Primary, defaultBaseURL)

	var (
		baseURL    = flag.String("server", envOr("AIRPORTS_SERVER_PRIMARY", resolvedBaseURL), "Server base URL (env: AIRPORTS_SERVER_PRIMARY)")
		apiVersion = flag.String("api-version", envOr("AIRPORTS_API_VERSION", firstNonEmptyStr(savedCfg.Server.APIVersion, defaultAPIVersion)), "API version path segment")
		format     = flag.String("format", envOr("AIRPORTS_OUTPUT_FORMAT", savedCfg.Output.Format), "Output format: table|json|yaml|plain|csv")
		jsonFlag   = flag.Bool("json", false, "Output as JSON")
		yamlFlag   = flag.Bool("yaml", false, "Output as YAML")
		showVer    = flag.Bool("version", false, "Print client version and exit")
		showHelp   = flag.Bool("help", false, "Print help and exit")
		token      = flag.String("token", envOr("AIRPORTS_TOKEN", configuredToken), "API token for authenticated operations (env: AIRPORTS_TOKEN)")
		configFlag = flag.String("config", "", "Config profile name (default: cli.yml)")
		colorFlag  = flag.String("color", "auto", "Color output: auto|yes|no")
		langFlag   = flag.String("lang", "auto", "Language for output (default: config > LC_ALL/LANG env)")
		debugFlag  = flag.Bool("debug", false, "Enable debug output")
		updateFlag = flag.String("update", "", "Check or apply a client update: check|yes|no")
	)
	flag.StringVar(format, "output", *format, "Alias for --format")
	flag.BoolVar(showHelp, "h", false, "alias for --help")
	flag.BoolVar(showVer, "v", false, "alias for --version")

	preParseLang := i18n.ResolveOutputLanguage(extractLangFlag(os.Args[1:]), savedCfg.Lang)

	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T(preParseLang, "cli.error_prefix"), err)
		printUsage(os.Stderr, binaryName, preParseLang)
		os.Exit(2)
	}

	if *showVer {
		fmt.Println((commonversion.BuildInfo{Version: Version, CommitID: CommitID, BuildDate: BuildDate}).Format(binaryName))
		if OfficialSite != "" {
			fmt.Println(OfficialSite)
		}
		return
	}

	lang := i18n.ResolveOutputLanguage(*langFlag, savedCfg.Lang)
	args := flag.Args()
	if *showHelp {
		printUsage(os.Stdout, binaryName, lang)
		return
	}
	updateAction := strings.ToLower(strings.TrimSpace(*updateFlag))
	if updateAction != "" && updateAction != "check" {
		updateEnabled, parseErr := config.ParseBool(updateAction, false)
		if parseErr != nil {
			fmt.Fprintln(os.Stderr, i18n.T(lang, "cli.error_prefix"), parseErr)
			os.Exit(2)
		}
		if updateEnabled {
			updateAction = "yes"
		} else {
			updateAction = "no"
		}
	}
	if !validColor(*colorFlag) {
		fmt.Fprintln(os.Stderr, i18n.T(lang, "cli.error_prefix"), i18n.Tf(lang, "cli.err_invalid_color", "value", *colorFlag))
		os.Exit(2)
	}
	displayEnv := display.DetectDisplayEnv()
	displayMode := selectDisplayMode(displayEnv, args)
	if len(args) == 0 {
		var modeErr error
		displayMode, modeErr = applyDisplayConfig(displayEnv, displayMode, savedCfg.Display.Mode)
		if modeErr != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", binaryName, modeErr)
			os.Exit(2)
		}
	}

	if *baseURL == "" {
		if updateAction == "yes" {
			fmt.Fprintf(os.Stderr, "%s: --update yes requires a configured server URL\n", binaryName)
			os.Exit(2)
		}
		if len(args) > 0 || (displayMode != display.DisplayModeTUI && displayMode != display.DisplayModeGUI) {
			fmt.Fprintf(os.Stderr, "%s: configure --server URL or AIRPORTS_SERVER_PRIMARY\n", binaryName)
			os.Exit(2)
		}
	}
	if updateAction == "check" && *baseURL == "" {
		fmt.Fprintf(os.Stderr, "%s: --update check requires a configured server URL\n", binaryName)
		os.Exit(2)
	}
	if *baseURL != "" {
		if err := validateServerURL(*baseURL); err != nil {
			fmt.Fprintf(os.Stderr, "%s: invalid --server URL: %v\n", binaryName, err)
			os.Exit(2)
		}
		c := newConfiguredClient(*baseURL, *token, lang, savedCfg)
		if err := checkAndMaybeUpdateCLI(c, savedCfg, updateAction); err != nil {
			fmt.Fprintln(os.Stderr, i18n.T(lang, "cli.error_prefix"), err)
			if updateAction == "check" || updateAction == "yes" || strings.Contains(err.Error(), "server requires") {
				os.Exit(1)
			}
			fmt.Fprintln(os.Stderr, "CLI update check failed; continuing without an update.")
		}
		if updateAction == "check" {
			return
		}
	}
	if len(args) == 0 {
		switch displayMode {
		case display.DisplayModeGUI:
			if gui.IsGUIAvailable() {
				if err := gui.LaunchGUI(gui.Config{
					ServerURL: *baseURL, Token: *token,
					OnConnect: func(ctx context.Context, serverURL, accessToken string) (string, error) {
						return testClientConnectionWithConfig(ctx, serverURL, accessToken, savedCfg)
					},
					OnRequest: func(ctx context.Context, serverURL, accessToken, method, path string, query url.Values, body string) (string, error) {
						return executeRawRequest(ctx, newConfiguredClient(serverURL, accessToken, lang, savedCfg), method, path, query, body)
					},
					Save: func(serverURL, accessToken string) error {
						cfg := *savedCfg
						cfg.Server.Primary, cfg.Token = serverURL, accessToken
						return saveCLIConfig(&cfg, configName)
					},
				}); err != nil {
					fmt.Fprintf(os.Stderr, "%s: GUI: %v\n", binaryName, err)
					os.Exit(1)
				}
				return
			}
			if strings.EqualFold(savedCfg.Display.Mode, "gui") {
				fmt.Fprintf(os.Stderr, "%s: configured GUI mode requires a gui-tagged build\n", binaryName)
				os.Exit(2)
			}
			if !isTTY(os.Stdin) || !isTTY(os.Stdout) {
				fmt.Fprintf(os.Stderr, "%s: GUI mode requires a gui-tagged build; no interactive terminal is available\n", binaryName)
				os.Exit(2)
			}
			fallthrough
		case display.DisplayModeTUI:
			if !isTTY(os.Stdin) || !isTTY(os.Stdout) {
				fmt.Fprintf(os.Stderr, "%s: interactive mode requires a terminal; provide a command\n", binaryName)
				os.Exit(2)
			}
			if err := runTUI(*baseURL, *token, lang, savedCfg, configName, display.ColorEnabled(colorSettingOverride(*colorFlag), cliConfigColor(savedCfg))); err != nil {
				fmt.Fprintf(os.Stderr, "%s: TUI: %v\n", binaryName, err)
				os.Exit(1)
			}
			return
		default:
			printUsage(os.Stderr, binaryName, lang)
			os.Exit(2)
		}
	}
	mode.SetDebugEnabled(*debugFlag)

	// SaveIfEmptyOrInvalid: only persist --token/--server back to the config
	// profile when the currently saved value is empty, never overwrite a
	// valid existing value.
	dirty := false
	if savedCfg.Token == "" && *token != "" {
		savedCfg.Token = *token
		dirty = true
	}
	if savedCfg.Server.Primary == "" && *baseURL != "" {
		savedCfg.Server.Primary = *baseURL
		dirty = true
	}
	if savedCfg.Server.APIVersion == "" && *apiVersion != "" {
		savedCfg.Server.APIVersion = *apiVersion
		dirty = true
	}
	if savedCfg.Output.Color == "" && savedCfg.Color == "" && *colorFlag != "" {
		savedCfg.Output.Color = cliColorValue(*colorFlag)
		dirty = true
	}
	if savedCfg.Lang == "" && *langFlag != "" {
		savedCfg.Lang = *langFlag
		dirty = true
	}
	if dirty {
		_ = saveCLIConfig(savedCfg, configName)
	}

	// Resolve output format: --json and --yaml take priority over --format.
	outFormat := *format
	if *yamlFlag {
		outFormat = "yaml"
	} else if *jsonFlag {
		outFormat = "json"
	} else if outFormat == "" {
		if isTTY(os.Stdout) {
			outFormat = "table"
		} else {
			outFormat = "plain"
		}
	}

	cmd, sub := args[0], args[1:]

	c := newConfiguredClient(*baseURL, *token, lang, savedCfg)
	c.apiVersion = *apiVersion
	if err := dispatch(c, cmd, sub, outFormat, lang); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T(lang, "cli.error_prefix"), err)
		os.Exit(1)
	}

	_ = configFlag // consumed via extractConfigFlag before flag.Parse; kept for --help documentation
}

func newClient(baseURL, token, lang string) *client {
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"), apiVersion: defaultAPIVersion,
		token: token, lang: lang, http: &http.Client{Timeout: requestTimeout}, timeout: requestTimeout,
		retryCount: 3, retryDelay: time.Second,
	}
}

func newConfiguredClient(baseURL, token, lang string, cfg *cliConfig) *client {
	c := newClient(baseURL, token, lang)
	if cfg == nil {
		return c
	}
	if cfg.Server.APIVersion != "" {
		c.apiVersion = cfg.Server.APIVersion
	}
	if timeout, err := time.ParseDuration(cfg.Server.Timeout); err == nil && timeout > 0 {
		c.timeout = timeout
		c.http.Timeout = timeout
	}
	if cfg.Server.Retry >= 0 {
		c.retryCount = cfg.Server.Retry
	}
	if delay, err := time.ParseDuration(cfg.Server.RetryDelay); err == nil && delay >= 0 {
		c.retryDelay = delay
	}
	return c
}

func runTUI(baseURL, token, lang string, savedCfg *cliConfig, configName string, useColor bool) error {
	initialURL := baseURL
	if initialURL == "" {
		initialURL = savedCfg.Server.Primary
	}
	updated, err := tui.Run(tui.Config{ServerURL: initialURL, Token: token, Language: lang, UseColor: useColor}, tui.Actions{
		TestConnection: func(ctx context.Context, serverURL, accessToken string) (string, error) {
			return testClientConnectionWithConfig(ctx, serverURL, accessToken, savedCfg)
		},
		Execute: func(ctx context.Context, serverURL, accessToken string, request tui.Request) (string, error) {
			return executeTUIRequestWithConfig(ctx, serverURL, accessToken, lang, savedCfg, request)
		},
	})
	if err != nil {
		return err
	}
	if updated.ServerURL != "" && (updated.ServerURL != savedCfg.Server.Primary || updated.Token != savedCfg.Token) {
		cfg := *savedCfg
		cfg.Server.Primary, cfg.Token = updated.ServerURL, updated.Token
		if err := saveCLIConfig(&cfg, configName); err != nil {
			return fmt.Errorf("save CLI configuration: %w", err)
		}
	}
	return nil
}

func testClientConnection(ctx context.Context, serverURL, token string) (string, error) {
	return testClientConnectionWithConfig(ctx, serverURL, token, nil)
}

func testClientConnectionWithConfig(ctx context.Context, serverURL, token string, cfg *cliConfig) (string, error) {
	if err := validateServerURL(serverURL); err != nil {
		return "", err
	}
	c := newConfiguredClient(serverURL, token, "en", cfg)
	body, err := c.get(ctx, c.baseURL+"/server/healthz", nil)
	return strings.TrimSpace(string(body)), err
}

func executeTUIRequest(ctx context.Context, serverURL, token, lang string, request tui.Request) (string, error) {
	return executeTUIRequestWithConfig(ctx, serverURL, token, lang, nil, request)
}

func executeTUIRequestWithConfig(ctx context.Context, serverURL, token, lang string, cfg *cliConfig, request tui.Request) (string, error) {
	c := newConfiguredClient(serverURL, token, lang, cfg)
	if request.Operation != "api" {
		var path string
		query := url.Values{}
		switch request.Operation {
		case "search":
			path = "/airports/search"
			query.Set("q", request.Query)
		case "get":
			path = "/airports/" + url.PathEscape(strings.ToUpper(request.Code))
		case "nearby":
			path = "/airports/nearby"
			query.Set("lat", request.Latitude)
			query.Set("lon", request.Longitude)
			if request.Limit != "" {
				query.Set("n", request.Limit)
			}
		case "health":
			return executeRawRequest(ctx, c, http.MethodGet, "/server/healthz", nil, "")
		case "version":
			path = "/server/about"
		default:
			return "", fmt.Errorf("unsupported operation %q", request.Operation)
		}
		body, err := c.get(ctx, c.apiURL(path, query), nil)
		return string(body), err
	}
	query, err := url.ParseQuery(request.Query)
	if err != nil {
		return "", fmt.Errorf("invalid query string: %w", err)
	}
	return executeRawRequest(ctx, c, request.Method, request.Path, query, request.Body)
}

func executeRawRequest(ctx context.Context, c *client, method, path string, query url.Values, body string) (string, error) {
	if method == "" {
		method = http.MethodGet
	}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") || strings.Contains(path, "\\") {
		return "", errors.New("API path must be an absolute path on the configured server")
	}
	parsed, err := url.ParseRequestURI(path)
	if err != nil || parsed.IsAbs() || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("API path must not include a host, query, or fragment")
	}
	if query == nil {
		query = url.Values{}
	}
	var payload io.Reader
	if body != "" {
		payload = strings.NewReader(body)
	}
	apiPrefix := "/api/" + c.apiVersion
	apiPath := parsed.EscapedPath()
	if apiPath == apiPrefix {
		apiPath = ""
	} else if strings.HasPrefix(apiPath, apiPrefix+"/") {
		apiPath = strings.TrimPrefix(apiPath, apiPrefix)
	}
	endpoint := c.baseURL + apiPrefix + strings.TrimRight(apiPath, "/")
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, payload)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-ID", uuid.NewString())
	req.Header.Set("User-Agent", "airports-cli/"+Version)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return string(data), fmt.Errorf("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	return string(data), nil
}

// extractConfigFlag pre-scans argv for --config/-config (value or =value
// form) so the resolved profile name is available before the first-run
// wizard and SaveIfEmptyOrInvalid logic need to read/write cli.yml. This
// mirrors the pattern already used for --update in the server binary since
// Go's flag package has no notion of "parse this one flag early".
func extractConfigFlag(argv []string) string {
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		switch {
		case arg == "--config" || arg == "-config":
			if i+1 < len(argv) {
				return argv[i+1]
			}
		case strings.HasPrefix(arg, "--config="):
			return strings.TrimPrefix(arg, "--config=")
		case strings.HasPrefix(arg, "-config="):
			return strings.TrimPrefix(arg, "-config=")
		}
	}
	return ""
}

func extractLangFlag(argv []string) string {
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if arg == "--" {
			break
		}
		switch {
		case arg == "--lang" || arg == "-lang":
			if i+1 < len(argv) {
				return argv[i+1]
			}
		case strings.HasPrefix(arg, "--lang="):
			return strings.TrimPrefix(arg, "--lang=")
		case strings.HasPrefix(arg, "-lang="):
			return strings.TrimPrefix(arg, "-lang=")
		}
	}
	return ""
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// firstNonEmptyStr returns the first non-empty value, implementing the
// PART 32 CLI language chain: --lang flag > config > LANG/LC_ALL > en.
func firstNonEmptyStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func printUsage(w io.Writer, binaryName string, lang string) {
	_, _ = fmt.Fprintf(w, "%s - %s\n\n%s\n  %s [flags] <command> [args...]\n\n%s\n  search <query>            %s\n  get <code>                %s\n  nearby <lat> <lon> [n]    %s\n  health                    %s\n  version                   %s\n\n%s\n",
		binaryName, i18n.T(lang, "cli.description"),
		i18n.T(lang, "cli.usage"), binaryName,
		i18n.T(lang, "cli.commands"),
		i18n.T(lang, "cli.cmd_search"),
		i18n.T(lang, "cli.cmd_get"),
		i18n.T(lang, "cli.cmd_nearby"),
		i18n.T(lang, "cli.cmd_health"),
		i18n.T(lang, "cli.cmd_version"),
		i18n.T(lang, "cli.flags"))
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "  --server URL              %s\n", i18n.T(lang, "cli.flag_server"))
	_, _ = fmt.Fprintf(w, "  --token TOKEN              %s\n", i18n.T(lang, "cli.flag_token"))
	_, _ = fmt.Fprintf(w, "  --config NAME              %s\n", i18n.T(lang, "cli.flag_config"))
	_, _ = fmt.Fprintf(w, "  --api-version VERSION     %s\n", i18n.Tf(lang, "cli.flag_api_version", "default", defaultAPIVersion))
	_, _ = fmt.Fprintf(w, "  --json                    %s\n", i18n.T(lang, "cli.flag_json"))
	_, _ = fmt.Fprintf(w, "  --yaml                    %s\n", i18n.T(lang, "cli.flag_yaml"))
	_, _ = fmt.Fprintf(w, "  --format FORMAT           %s\n", i18n.T(lang, "cli.flag_format"))
	_, _ = fmt.Fprintln(w, "  --output FORMAT           Alias for --format")
	_, _ = fmt.Fprintf(w, "  --color {auto|yes|no}      %s\n", i18n.T(lang, "cli.flag_color"))
	_, _ = fmt.Fprintf(w, "  --lang CODE                %s\n", i18n.T(lang, "cli.flag_lang"))
	_, _ = fmt.Fprintf(w, "  --debug                    %s\n", i18n.T(lang, "cli.flag_debug"))
	_, _ = fmt.Fprintln(w, "  --update {check|yes|no}     Check or apply a CLI update")
	_, _ = fmt.Fprintf(w, "  -h, --help                %s\n", i18n.T(lang, "cli.flag_help"))
	_, _ = fmt.Fprintf(w, "  -v, --version              %s\n", i18n.T(lang, "cli.flag_version"))
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, i18n.T(lang, "cli.shell_integration"))
	_, _ = fmt.Fprintf(w, "  --shell completions [SHELL]  %s\n", i18n.T(lang, "cli.shell_completions_desc"))
	_, _ = fmt.Fprintf(w, "  --shell init [SHELL]         %s\n", i18n.T(lang, "cli.shell_init_desc"))
	_, _ = fmt.Fprintf(w, "  --shell help                 %s\n", i18n.T(lang, "cli.shell_help_desc"))
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, i18n.T(lang, "cli.config_section"))
	_, _ = fmt.Fprintln(w, i18n.T(lang, "cli.config_paragraph"))
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, i18n.T(lang, "cli.examples"))
	_, _ = fmt.Fprintf(w, `  %[1]s search "kennedy"
  %[1]s get KJFK
  %[1]s nearby 40.7128 -74.0060 5
  %[1]s --server https://airports.example.com health
`, binaryName)
}

// client wraps the HTTP client with the resolved server URL.
type client struct {
	baseURL    string
	apiVersion string
	token      string
	lang       string
	http       *http.Client
	timeout    time.Duration
	retryCount int
	retryDelay time.Duration
}

func (c *client) apiURL(path string, q url.Values) string {
	u := fmt.Sprintf("%s/api/%s%s", c.baseURL, c.apiVersion, path)
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

func (c *client) do(req *http.Request) (*http.Response, error) {
	httpClient := c.http
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	retries := c.retryCount
	if retries < 0 {
		retries = 0
	}
	delay := c.retryDelay
	if delay < 0 {
		delay = 0
	}
	for attempt := 0; ; attempt++ {
		current := req
		if attempt > 0 {
			current = req.Clone(req.Context())
			if req.Body != nil {
				if req.GetBody == nil {
					return nil, errors.New("request body cannot be replayed for retry")
				}
				body, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("recreate request body: %w", err)
				}
				current.Body = body
			}
		}
		response, err := httpClient.Do(current)
		retryable := err != nil || response != nil && (response.StatusCode == http.StatusTooManyRequests || response.StatusCode >= http.StatusInternalServerError)
		if !retryable || attempt >= retries {
			return response, err
		}
		if response != nil {
			if response.Body != nil {
				_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
				_ = response.Body.Close()
			}
			if value := response.Header.Get("Retry-After"); value != "" {
				if seconds, parseErr := strconv.Atoi(value); parseErr == nil && seconds > 0 {
					delay = time.Duration(seconds) * time.Second
				}
			}
		}
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-req.Context().Done():
				timer.Stop()
				return nil, req.Context().Err()
			case <-timer.C:
			}
		}
	}
}

// get performs a GET against the server and decodes JSON into out (if non-nil).
// Returns the raw body for callers that want to print it as-is.
func (c *client) get(ctx context.Context, fullURL string, out any) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Request-ID", uuid.NewString())
	req.Header.Set("User-Agent", "airports-cli/"+Version)
	if c.lang != "" {
		req.Header.Set("Accept-Language", c.lang)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024)) // 16 MiB cap
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if resp.StatusCode >= 400 {
		return body, fmt.Errorf("server returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if out != nil && len(body) > 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return body, fmt.Errorf("decode JSON: %w", err)
		}
	}
	return body, nil
}

func dispatch(c *client, cmd string, args []string, format string, lang string) error {
	timeout := c.timeout
	if timeout <= 0 {
		timeout = requestTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	switch cmd {
	case "search":
		if len(args) < 1 {
			return errors.New(i18n.T(lang, "cli.err_missing_query"))
		}
		q := url.Values{"q": {strings.Join(args, " ")}}
		body, err := c.get(ctx, c.apiURL("/airports/search", q), nil)
		if err != nil {
			return err
		}
		return printResult(body, format, lang)

	case "get":
		if len(args) < 1 {
			return errors.New(i18n.T(lang, "cli.err_missing_code"))
		}
		code := strings.ToUpper(args[0])
		body, err := c.get(ctx, c.apiURL("/airports/"+url.PathEscape(code), nil), nil)
		if err != nil {
			return err
		}
		return printResult(body, format, lang)

	case "nearby":
		if len(args) < 2 {
			return errors.New(i18n.T(lang, "cli.err_missing_latlon"))
		}
		q := url.Values{"lat": {args[0]}, "lon": {args[1]}}
		if len(args) >= 3 {
			q.Set("n", args[2])
		}
		body, err := c.get(ctx, c.apiURL("/airports/nearby", q), nil)
		if err != nil {
			return err
		}
		return printResult(body, format, lang)

	case "health":
		body, err := c.get(ctx, c.baseURL+"/server/healthz", nil)
		if err != nil {
			return err
		}
		return printResult(body, format, lang)

	case "version":
		fmt.Println("client: " + (commonversion.BuildInfo{Version: Version, CommitID: CommitID, BuildDate: BuildDate}).Format(filepath.Base(os.Args[0])))
		body, err := c.get(ctx, c.apiURL("/server/about", nil), nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, i18n.Tf(lang, "cli.server_unreachable", "error", err))
			return nil
		}
		fmt.Print("server: ")
		return printResult(body, format, lang)

	default:
		return errors.New(i18n.Tf(lang, "cli.err_unknown_command", "command", cmd))
	}
}

func printResult(body []byte, format string, lang string) error {
	switch format {
	case "table":
		return printTableResult(body)
	case "csv":
		return printCSVResult(body)
	case "json":
		// Pretty-print JSON when possible; otherwise print as-is.
		var v any
		if err := json.Unmarshal(body, &v); err == nil {
			out, _ := json.MarshalIndent(v, "", "  ")
			fmt.Println(string(out))
			return nil
		}
		fmt.Println(strings.TrimRight(string(body), "\n"))
		return nil
	case "yaml":
		// Unmarshal from JSON then re-encode as YAML.
		var v any
		if err := json.Unmarshal(body, &v); err != nil {
			// Fall back to raw text if not valid JSON.
			fmt.Println(strings.TrimRight(string(body), "\n"))
			return nil
		}
		out, err := yaml.Marshal(v)
		if err != nil {
			return fmt.Errorf("yaml marshal: %w", err)
		}
		fmt.Print(string(out))
		return nil
	case "text", "plain":
		fmt.Println(strings.TrimRight(string(body), "\n"))
		return nil
	default:
		return errors.New(i18n.Tf(lang, "cli.err_unsupported_format", "format", format))
	}
}

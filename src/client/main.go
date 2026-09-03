// airports-cli: command-line client for the airports API.
//
// This is the REQUIRED client binary per AI.md spec. It queries a running
// airports server (default: http://localhost:80) and prints results to
// stdout. It does NOT bundle any airport data of its own; the server is
// the source of truth.
package main

import (
	"bufio"
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

	"github.com/apimgr/airports/src/common/i18n"
	"github.com/apimgr/airports/src/mode"
	"github.com/apimgr/airports/src/shellcompl"
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

// buildEpoch parses the embedded BuildEpoch ldflag; 0 when unset or invalid.
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

const (
	defaultBaseURL    = "http://localhost:80"
	defaultAPIVersion = "v1"
	requestTimeout    = 30 * time.Second
)

// cliShellCommands and cliShellFlags feed shellcompl's generators (PART 32
// "Shell Completions") so completion scripts stay in sync with the
// subcommands/flags actually implemented below.
var (
	cliShellCommands = []string{"search", "get", "nearby", "health", "version", "shell"}
	cliShellFlags    = []string{
		"--server", "--api-version", "--format", "--json", "--yaml", "--version", "-v",
		"--help", "-h", "--token", "--config", "--color", "--lang", "--debug", "--shell",
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
	Server struct {
		Primary string `yaml:"primary"`
	} `yaml:"server"`
	Token string `yaml:"token"`
	Color string `yaml:"color"`
	Lang  string `yaml:"lang"`
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
		return &cliConfig{}, nil
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &cliConfig{}, nil
	}
	if err != nil {
		return &cliConfig{}, fmt.Errorf("read cli config: %w", err)
	}
	var cfg cliConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return &cliConfig{}, fmt.Errorf("parse cli config: %w", err)
	}
	return &cfg, nil
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
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal cli config: %w", err)
	}
	return os.WriteFile(path, data, 0600)
}

// isTTY reports whether fd is an interactive terminal (not a pipe/redirect).
func isTTY(fd *os.File) bool {
	fi, err := fd.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

// runFirstRunWizard checks whether a CLI config already exists and, if not
// and we are running interactively, prompts the user for the server URL.
// Returns the server URL to use (from config or the provided default).
func runFirstRunWizard(defaultURL, configName string) string {
	cfg, err := loadCLIConfig(configName)
	if err != nil {
		// Config load error is non-fatal — fall back to the default URL.
		return defaultURL
	}
	if cfg.Server.Primary != "" {
		return cfg.Server.Primary
	}
	// Config file exists but server.primary is empty, or file is absent.
	// Only show wizard when stdin and stderr are both interactive.
	if !isTTY(os.Stdin) || !isTTY(os.Stderr) {
		return defaultURL
	}

	lang := resolveLang(cfg.Lang)
	fmt.Fprintln(os.Stderr, i18n.T(lang, "cli.first_run_title"))
	fmt.Fprint(os.Stderr, i18n.Tf(lang, "cli.first_run_prompt", "default", defaultURL))
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		input = defaultURL
	}
	cfg.Server.Primary = input
	if saveErr := saveCLIConfig(cfg, configName); saveErr != nil {
		fmt.Fprintln(os.Stderr, i18n.Tf(lang, "cli.config_save_warning", "error", saveErr))
	} else {
		configPath, _ := cliConfigPath(configName)
		fmt.Fprintln(os.Stderr, i18n.Tf(lang, "cli.config_saved", "path", configPath))
	}
	return cfg.Server.Primary
}

// resolveLang applies the CLI's --lang > config > LANG/LC_ALL > en fallback
// chain (PART 32) and silently falls back to English for any unsupported or
// empty value (PART 30: "missing key / unsupported lang" must never error).
func resolveLang(raw string) string {
	if raw != "" && i18n.IsSupported(raw) {
		return raw
	}
	return "en"
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

	// First-run wizard: if no AIRPORTS_SERVER_PRIMARY env var is set and no
	// config file exists yet, prompt the user interactively for the server URL.
	resolvedBaseURL := defaultBaseURL
	if envOr("AIRPORTS_SERVER_PRIMARY", "") == "" {
		resolvedBaseURL = runFirstRunWizard(defaultBaseURL, configName)
	}

	savedCfg, _ := loadCLIConfig(configName)

	var (
		baseURL    = flag.String("server", envOr("AIRPORTS_SERVER_PRIMARY", resolvedBaseURL), "Server base URL (env: AIRPORTS_SERVER_PRIMARY)")
		apiVersion = flag.String("api-version", defaultAPIVersion, "API version path segment")
		format     = flag.String("format", "", "Output format: json|yaml|text (default: json)")
		jsonFlag   = flag.Bool("json", false, "Output as JSON")
		yamlFlag   = flag.Bool("yaml", false, "Output as YAML")
		showVer    = flag.Bool("version", false, "Print client version and exit")
		showHelp   = flag.Bool("help", false, "Print help and exit")
		token      = flag.String("token", envOr("AIRPORTS_TOKEN", savedCfg.Token), "API token for authenticated operations (env: AIRPORTS_TOKEN)")
		configFlag = flag.String("config", "", "Config profile name (default: cli.yml)")
		colorFlag  = flag.String("color", "auto", "Color output: auto|yes|no")
		langFlag   = flag.String("lang", firstNonEmptyStr(savedCfg.Lang, envOr("LANG", envOr("LC_ALL", ""))), "Language for output (default: config > LANG/LC_ALL env)")
		debugFlag  = flag.Bool("debug", false, "Enable debug output")
	)
	flag.BoolVar(showHelp, "h", false, "alias for --help")
	flag.BoolVar(showVer, "v", false, "alias for --version")

	preParseLang := resolveLang(firstNonEmptyStr(savedCfg.Lang, envOr("LANG", envOr("LC_ALL", ""))))

	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T(preParseLang, "cli.error_prefix"), err)
		printUsage(os.Stderr, binaryName, preParseLang)
		os.Exit(2)
	}

	if *showVer {
		fmt.Printf("%s %s (commit %s, built %s)\n", binaryName, Version, CommitID, BuildDate)
		if OfficialSite != "" {
			fmt.Println(OfficialSite)
		}
		return
	}

	lang := resolveLang(*langFlag)

	args := flag.Args()
	if *showHelp || len(args) == 0 {
		printUsage(os.Stdout, binaryName, lang)
		if *showHelp {
			return
		}
		os.Exit(2)
	}

	if !validColor(*colorFlag) {
		fmt.Fprintln(os.Stderr, i18n.T(lang, "cli.error_prefix"), i18n.Tf(lang, "cli.err_invalid_color", "value", *colorFlag))
		os.Exit(2)
	}

	mode.SetDebug(*debugFlag)

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
	if savedCfg.Color == "" && *colorFlag != "" {
		savedCfg.Color = *colorFlag
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
		outFormat = "json"
	}

	cmd, sub := args[0], args[1:]

	c := &client{
		baseURL:    strings.TrimRight(*baseURL, "/"),
		apiVersion: *apiVersion,
		token:      *token,
		lang:       lang,
		http:       &http.Client{Timeout: requestTimeout},
	}

	if err := dispatch(c, cmd, sub, outFormat, lang); err != nil {
		fmt.Fprintln(os.Stderr, i18n.T(lang, "cli.error_prefix"), err)
		os.Exit(1)
	}

	_ = configFlag // consumed via extractConfigFlag before flag.Parse; kept for --help documentation
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
	fmt.Fprintf(w, "%s - %s\n\n%s\n  %s [flags] <command> [args...]\n\n%s\n  search <query>            %s\n  get <code>                %s\n  nearby <lat> <lon> [n]    %s\n  health                    %s\n  version                   %s\n\n%s\n",
		binaryName, i18n.T(lang, "cli.description"),
		i18n.T(lang, "cli.usage"), binaryName,
		i18n.T(lang, "cli.commands"),
		i18n.T(lang, "cli.cmd_search"),
		i18n.T(lang, "cli.cmd_get"),
		i18n.T(lang, "cli.cmd_nearby"),
		i18n.T(lang, "cli.cmd_health"),
		i18n.T(lang, "cli.cmd_version"),
		i18n.T(lang, "cli.flags"))
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  --server URL              %s\n", i18n.Tf(lang, "cli.flag_server", "default", defaultBaseURL))
	fmt.Fprintf(w, "  --token TOKEN              %s\n", i18n.T(lang, "cli.flag_token"))
	fmt.Fprintf(w, "  --config NAME              %s\n", i18n.T(lang, "cli.flag_config"))
	fmt.Fprintf(w, "  --api-version VERSION     %s\n", i18n.Tf(lang, "cli.flag_api_version", "default", defaultAPIVersion))
	fmt.Fprintf(w, "  --json                    %s\n", i18n.T(lang, "cli.flag_json"))
	fmt.Fprintf(w, "  --yaml                    %s\n", i18n.T(lang, "cli.flag_yaml"))
	fmt.Fprintf(w, "  --format FORMAT           %s\n", i18n.T(lang, "cli.flag_format"))
	fmt.Fprintf(w, "  --color {auto|yes|no}      %s\n", i18n.T(lang, "cli.flag_color"))
	fmt.Fprintf(w, "  --lang CODE                %s\n", i18n.T(lang, "cli.flag_lang"))
	fmt.Fprintf(w, "  --debug                    %s\n", i18n.T(lang, "cli.flag_debug"))
	fmt.Fprintf(w, "  -h, --help                %s\n", i18n.T(lang, "cli.flag_help"))
	fmt.Fprintf(w, "  -v, --version              %s\n", i18n.T(lang, "cli.flag_version"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T(lang, "cli.shell_integration"))
	fmt.Fprintf(w, "  --shell completions [SHELL]  %s\n", i18n.T(lang, "cli.shell_completions_desc"))
	fmt.Fprintf(w, "  --shell init [SHELL]         %s\n", i18n.T(lang, "cli.shell_init_desc"))
	fmt.Fprintf(w, "  --shell help                 %s\n", i18n.T(lang, "cli.shell_help_desc"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T(lang, "cli.config_section"))
	fmt.Fprintln(w, i18n.T(lang, "cli.config_paragraph"))
	fmt.Fprintln(w)
	fmt.Fprintln(w, i18n.T(lang, "cli.examples"))
	fmt.Fprintf(w, `  %[1]s search "kennedy"
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
}

func (c *client) apiURL(path string, q url.Values) string {
	u := fmt.Sprintf("%s/api/%s%s", c.baseURL, c.apiVersion, path)
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	return u
}

// get performs a GET against the server and decodes JSON into out (if non-nil).
// Returns the raw body for callers that want to print it as-is.
func (c *client) get(ctx context.Context, fullURL string, out any) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "airports-cli/"+Version)
	if c.lang != "" {
		req.Header.Set("Accept-Language", c.lang)
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

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
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
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
		fmt.Printf("client: %s %s (commit %s, built %s)\n", filepath.Base(os.Args[0]), Version, CommitID, BuildDate)
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
	case "text":
		fmt.Println(strings.TrimRight(string(body), "\n"))
		return nil
	default:
		return errors.New(i18n.Tf(lang, "cli.err_unsupported_format", "format", format))
	}
}

// airports-cli: command-line client for the airports API.
//
// This is the REQUIRED client binary per AI.md spec. It queries a running
// airports server (default: http://localhost:80) and prints results to
// stdout. It does NOT bundle any airport data of its own; the server is
// the source of truth.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Injected at build time via ldflags.
var (
	Version      = "dev"
	CommitID     = "unknown"
	BuildDate    = "unknown"
	OfficialSite = ""
)

const (
	defaultBaseURL    = "http://localhost:80"
	defaultAPIVersion = "v1"
	requestTimeout    = 30 * time.Second
)

// honorNoColor returns true when output should suppress ANSI color codes,
// per the NO_COLOR convention (https://no-color.org/) and the spec rule
// that all binaries respect NO_COLOR.
func honorNoColor() bool {
	if v := os.Getenv("NO_COLOR"); v != "" {
		return true
	}
	return false
}

func main() {
	// Suppress flag's default error output so we control formatting.
	flag.CommandLine.SetOutput(io.Discard)

	var (
		baseURL    = flag.String("server", envOr("AIRPORTS_SERVER", defaultBaseURL), "Server base URL (env: AIRPORTS_SERVER)")
		apiVersion = flag.String("api-version", defaultAPIVersion, "API version path segment")
		format     = flag.String("format", "", "Output format: json|yaml|text (default: json)")
		jsonFlag   = flag.Bool("json", false, "Output as JSON")
		yamlFlag   = flag.Bool("yaml", false, "Output as YAML")
		showVer    = flag.Bool("version", false, "Print client version and exit")
		showHelp   = flag.Bool("help", false, "Print help and exit")
	)
	flag.BoolVar(showHelp, "h", false, "alias for --help")

	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		printUsage(os.Stderr)
		os.Exit(2)
	}

	if *showVer {
		fmt.Printf("airports-cli %s (commit %s, built %s)\n", Version, CommitID, BuildDate)
		if OfficialSite != "" {
			fmt.Println(OfficialSite)
		}
		return
	}

	args := flag.Args()
	if *showHelp || len(args) == 0 {
		printUsage(os.Stdout)
		if *showHelp {
			return
		}
		os.Exit(2)
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
		http:       &http.Client{Timeout: requestTimeout},
	}

	if err := dispatch(c, cmd, sub, outFormat); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func printUsage(w io.Writer) {
	fmt.Fprintf(w, `airports-cli - command-line client for the airports API

Usage:
  airports-cli [flags] <command> [args...]

Commands:
  search <query>            Full-text search across airports
  get <code>                Look up an airport by ICAO or IATA code
  nearby <lat> <lon> [n]    Find the N nearest airports (default 10)
  health                    Print server health status
  version                   Print client and server versions

Flags:
  --server URL              Server base URL (default %s; env: AIRPORTS_SERVER)
  --api-version VERSION     API version (default %s)
  --json                    Output as JSON (default)
  --yaml                    Output as YAML
  --format FORMAT           Output format: json|yaml|text
  -h, --help                Show this help
  --version                 Show client version

Examples:
  airports-cli search "kennedy"
  airports-cli get KJFK
  airports-cli nearby 40.7128 -74.0060 5
  airports-cli --server https://airports.example.com health
`, defaultBaseURL, defaultAPIVersion)
}

// client wraps the HTTP client with the resolved server URL.
type client struct {
	baseURL    string
	apiVersion string
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

func dispatch(c *client, cmd string, args []string, format string) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	switch cmd {
	case "search":
		if len(args) < 1 {
			return fmt.Errorf("search: missing query")
		}
		q := url.Values{"q": {strings.Join(args, " ")}}
		body, err := c.get(ctx, c.apiURL("/search", q), nil)
		if err != nil {
			return err
		}
		return printResult(body, format)

	case "get":
		if len(args) < 1 {
			return fmt.Errorf("get: missing airport code")
		}
		code := strings.ToUpper(args[0])
		body, err := c.get(ctx, c.apiURL("/airports/"+url.PathEscape(code), nil), nil)
		if err != nil {
			return err
		}
		return printResult(body, format)

	case "nearby":
		if len(args) < 2 {
			return fmt.Errorf("nearby: need lat and lon")
		}
		q := url.Values{"lat": {args[0]}, "lon": {args[1]}}
		if len(args) >= 3 {
			q.Set("n", args[2])
		}
		body, err := c.get(ctx, c.apiURL("/nearby", q), nil)
		if err != nil {
			return err
		}
		return printResult(body, format)

	case "health":
		body, err := c.get(ctx, c.baseURL+"/server/healthz", nil)
		if err != nil {
			return err
		}
		return printResult(body, format)

	case "version":
		fmt.Printf("client: airports-cli %s (commit %s, built %s)\n", Version, CommitID, BuildDate)
		body, err := c.get(ctx, c.apiURL("/server/about", nil), nil)
		if err != nil {
			fmt.Fprintln(os.Stderr, "server: unreachable:", err)
			return nil
		}
		fmt.Print("server: ")
		return printResult(body, format)

	default:
		return fmt.Errorf("unknown command: %s (run with --help)", cmd)
	}
}

func printResult(body []byte, format string) error {
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
		return fmt.Errorf("unsupported format: %s (use json, yaml, or text)", format)
	}
}

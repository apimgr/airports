//go:build gui && !freebsd && !netbsd && !openbsd

package gui

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gogpu/gg"
	_ "github.com/gogpu/gg/gpu"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/core/button"
	"github.com/gogpu/ui/core/dialog"
	"github.com/gogpu/ui/core/scrollview"
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"

	"github.com/apimgr/airports/src/common/display"
	"github.com/apimgr/airports/src/common/i18n"
)

// NativeGUIConfig supplies the client operations so the GUI stays independent of the
// command package and can be embedded by other clients.
type NativeGUIConfig struct {
	ServerURL string
	Token     string
	Language  string
	OnConnect func(context.Context, string, string) (string, error)
	OnRequest func(context.Context, string, string, string, string, url.Values, string) (string, error)
	Save      func(string, string) error
}

// IsGUIAvailable reports whether a supported local graphical session exists.
func IsGUIAvailable() bool {
	env := display.DetectDisplayEnv()
	return env.HasDisplay && !env.IsSSH && !env.IsMosh
}

// LaunchGUI opens the native Airports API client.
func LaunchGUI(config NativeGUIConfig) error {
	if !IsGUIAvailable() {
		return errors.New("GUI requires a local display session; use the terminal client over SSH or Mosh")
	}
	if config.OnConnect == nil || config.OnRequest == nil || config.Save == nil {
		return errors.New("GUI requires connect, request, and save callbacks")
	}

	window := gogpu.NewApp(gogpu.DefaultConfig().WithTitle(guiText(config.Language, "client_title", "Airports API Client")).WithSize(1024, 760))
	uiApp := app.New(
		app.WithWindowProvider(window),
		app.WithPlatformProvider(window),
		app.WithEventSource(window.EventSource()),
		app.WithRenderMode(app.RenderModeFrameworkManaged),
	)
	state := &viewState{serverURL: config.ServerURL, token: config.Token}
	if strings.TrimSpace(config.ServerURL) == "" {
		uiApp.SetRoot(buildSetup(config, state, uiApp))
	} else {
		uiApp.SetRoot(buildWorkspace(config, state, uiApp))
	}

	var canvas *ggcanvas.Canvas
	window.OnDraw(func(dc *gogpu.Context) {
		w, h := dc.Width(), dc.Height()
		if w <= 0 || h <= 0 {
			return
		}
		if canvas == nil {
			provider := window.GPUContextProvider()
			if provider == nil {
				return
			}
			var err error
			canvas, err = ggcanvas.New(provider, w, h)
			if err != nil {
				state.setMessage(guiText(config.Language, "graphics_error", "Graphics initialization failed") + ": " + err.Error())
				return
			}
		}
		uiApp.Frame()
		cw, ch := canvas.Size()
		if cw != w || ch != h {
			if err := canvas.Resize(w, h); err != nil {
				state.setMessage(guiText(config.Language, "resize_error", "Window resize failed") + ": " + err.Error())
				return
			}
			cw, ch = w, h
		}
		canvas.Draw(func(cc *gg.Context) {
			uiApp.Window().DrawTo(render.NewCanvas(cc, cw, ch))
		})
		surfaceWidth, surfaceHeight := dc.SurfaceSize()
		if err := canvas.RenderDirect(dc.RenderTarget().SurfaceView(), surfaceWidth, surfaceHeight); err != nil {
			state.setMessage(guiText(config.Language, "render_error", "Rendering failed") + ": " + err.Error())
		}
	})
	window.OnClose(func() { gg.CloseAccelerator() })
	return window.Run()
}

type viewState struct {
	mu        sync.RWMutex
	operation string
	serverURL string
	token     string
	method    string
	path      string
	query     string
	body      string
	message   string
	result    string
}

func (s *viewState) setMessage(message string) {
	s.mu.Lock()
	s.message = message
	s.mu.Unlock()
}

func (s *viewState) setResult(result string) {
	s.mu.Lock()
	s.result = result
	s.mu.Unlock()
}

func (s *viewState) snapshot() viewState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return viewState{
		operation: s.operation,
		serverURL: s.serverURL,
		token:     s.token,
		method:    s.method,
		path:      s.path,
		query:     s.query,
		body:      s.body,
		message:   s.message,
		result:    s.result,
	}
}

func buildSetup(config NativeGUIConfig, state *viewState, uiApp *app.App) *scrollview.Widget {
	urlField := textfield.New(
		textfield.A11yLabel(guiText(config.Language, "server_url", "Airports server URL")),
		textfield.Placeholder("https://airports.example.org"),
		textfield.InitialValue(state.serverURL),
		textfield.OnChange(func(value string) { state.mu.Lock(); state.serverURL = strings.TrimSpace(value); state.mu.Unlock() }),
	)
	tokenField := textfield.New(
		textfield.A11yLabel(guiText(config.Language, "token_optional", "Access token (optional)")),
		textfield.Placeholder(guiText(config.Language, "token_optional", "Access token (optional)")),
		textfield.InputTypeOpt(textfield.TypePassword),
		textfield.InitialValue(state.token),
		textfield.OnChange(func(value string) { state.mu.Lock(); state.token = value; state.mu.Unlock() }),
	)
	status := primitives.TextFn(func() string { return state.snapshot().message }).FontSize(14)
	var connect *button.Widget
	showConnectionError := func(message string) {
		state.setMessage(message)
		showErrorDialog(config, uiApp, connect, message)
	}
	connect = button.New(button.TextOpt(guiText(config.Language, "connect_save", "Connect and save")), button.OnClick(func() {
		v := state.snapshot()
		parsed, err := url.ParseRequestURI(v.serverURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			showConnectionError(guiText(config.Language, "invalid_server_url", "Enter a valid HTTP or HTTPS server URL without credentials, query, or fragment."))
			return
		}
		state.setMessage(guiText(config.Language, "checking_connection", "Checking server connection..."))
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		info, err := config.OnConnect(ctx, strings.TrimRight(v.serverURL, "/"), v.token)
		if err != nil {
			showConnectionError(guiText(config.Language, "connection_failed", "Connection failed") + ": " + err.Error())
			return
		}
		if err := config.Save(strings.TrimRight(v.serverURL, "/"), v.token); err != nil {
			showConnectionError(guiText(config.Language, "save_failed", "Connected, but saving configuration failed") + ": " + err.Error())
			return
		}
		state.setMessage(guiText(config.Language, "connected", "Connected successfully.") + " " + info)
		uiApp.SetRoot(buildWorkspace(config, state, uiApp))
	}))
	content := primitives.Box(
		primitives.Text(guiText(config.Language, "title", "Airports")).FontSize(30).Bold(),
		primitives.Text(guiText(config.Language, "connect_title", "Connect to your Airports server")).FontSize(17),
		primitives.Text(guiText(config.Language, "connect_instructions", "Enter the server address and optional access token. The connection is checked before it is saved.")).FontSize(14),
		primitives.Text(guiText(config.Language, "server_url_label", "Server URL")).FontSize(14).Bold(), urlField,
		primitives.Text(guiText(config.Language, "token_label", "Access token")).FontSize(14).Bold(), tokenField,
		connect,
		status,
	).Padding(32).Gap(14)
	return scrollview.New(content, scrollview.DirectionOpt(scrollview.Vertical))
}

func buildWorkspace(config NativeGUIConfig, state *viewState, uiApp *app.App) *scrollview.Widget {
	v := state.snapshot()
	if v.operation == "" {
		v.operation = "api"
	}
	pathLabel := guiText(config.Language, "api_path", "API path")
	pathPlaceholder := defaultString(v.path, "/airports")
	if v.operation == "get" {
		pathLabel = guiText(config.Language, "airport_code", "ICAO or IATA code")
		pathPlaceholder = guiText(config.Language, "airport_code_placeholder", "Enter an airport code")
	}
	pathInitial := defaultString(v.path, "/airports")
	if v.operation == "get" {
		pathInitial = v.path
	}
	methodField := textfield.New(
		textfield.A11yLabel(guiText(config.Language, "http_method", "HTTP method")),
		textfield.Placeholder("GET"),
		textfield.InitialValue(defaultString(v.method, http.MethodGet)),
		textfield.OnChange(func(value string) {
			state.mu.Lock()
			state.method = strings.ToUpper(strings.TrimSpace(value))
			state.mu.Unlock()
		}),
	)
	pathField := textfield.New(
		textfield.A11yLabel(pathLabel),
		textfield.Placeholder(pathPlaceholder),
		textfield.InitialValue(pathInitial),
		textfield.OnChange(func(value string) {
			state.mu.Lock()
			state.path = strings.TrimSpace(value)
			state.mu.Unlock()
		}),
	)
	queryField := textfield.New(
		textfield.A11yLabel(guiText(config.Language, "query_parameters", "Query parameters")),
		textfield.Placeholder("limit=25&search=JFK"),
		textfield.InitialValue(v.query),
		textfield.OnChange(func(value string) {
			state.mu.Lock()
			state.query = value
			state.mu.Unlock()
		}),
	)
	bodyField := textfield.New(
		textfield.A11yLabel(guiText(config.Language, "request_body_json", "Request body (JSON)")),
		textfield.Placeholder(guiText(config.Language, "request_body_optional", "Request body (JSON, optional)")),
		textfield.InitialValue(v.body),
		textfield.OnChange(func(value string) {
			state.mu.Lock()
			state.body = value
			state.mu.Unlock()
		}),
	)
	status := primitives.TextFn(func() string { return state.snapshot().message }).FontSize(14)
	result := primitives.TextFn(func() string {
		value := state.snapshot().result
		if value == "" {
			return guiText(config.Language, "response_placeholder", "Response will appear here.")
		}
		return value
	}).FontSize(13)
	var request *button.Widget
	showRequestError := func(message string) {
		state.setMessage(message)
		showErrorDialog(config, uiApp, request, message)
	}
	request = button.New(button.TextOpt(guiText(config.Language, "send_request", "Send request")), button.OnClick(func() {
		requestState := state.snapshot()
		method := strings.ToUpper(strings.TrimSpace(requestState.method))
		if method == "" {
			method = "GET"
		}
		switch method {
		case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions:
		default:
			showRequestError(guiText(config.Language, "unsupported_method", "Choose a supported HTTP method: GET, POST, PUT, PATCH, DELETE, HEAD, or OPTIONS."))
			return
		}
		path := strings.TrimSpace(requestState.path)
		if requestState.operation == "get" {
			if path == "" {
				showRequestError(guiText(config.Language, "airport_code_required", "Enter an airport code."))
				return
			}
			path = "/airports/" + url.PathEscape(strings.ToUpper(path))
		}
		parsedPath, err := url.ParseRequestURI(path)
		if err != nil || !strings.HasPrefix(path, "/") || parsedPath.IsAbs() || strings.ContainsAny(path, "?#\r\n") {
			showRequestError(guiText(config.Language, "invalid_api_path", "API path must be a valid absolute path beginning with /."))
			return
		}
		query, err := url.ParseQuery(requestState.query)
		if err != nil {
			showRequestError(guiText(config.Language, "invalid_query", "Query parameters are invalid") + ": " + err.Error())
			return
		}
		if requestState.operation == "search" {
			if strings.TrimSpace(requestState.query) == "" {
				showRequestError(guiText(config.Language, "search_query_required", "Enter a search query."))
				return
			}
			query = url.Values{"q": []string{strings.TrimSpace(requestState.query)}}
		}
		if requestState.body != "" && !json.Valid([]byte(requestState.body)) {
			showRequestError(guiText(config.Language, "invalid_json", "Request body must be valid JSON."))
			return
		}
		state.setMessage(guiText(config.Language, "request_in_progress", "Request in progress..."))
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		response, err := config.OnRequest(ctx, requestState.serverURL, requestState.token, method, path, query, requestState.body)
		if err != nil {
			showRequestError(guiText(config.Language, "request_failed", "Request failed") + ": " + err.Error())
			state.setResult("")
			return
		}
		state.setMessage(guiText(config.Language, "request_completed", "Request completed"))
		formattedResponse := prettyResponse(response)
		state.setResult(formattedResponse)
		if method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete {
			showDialog(config, uiApp, request, guiText(config.Language, "request_completed", "Request completed"), formattedResponse)
		}
	}))
	setup := button.New(
		button.TextOpt(guiText(config.Language, "change_server", "Change server")),
		button.VariantOpt(button.Outlined),
		button.OnClick(func() { uiApp.SetRoot(buildSetup(config, state, uiApp)) }),
	)
	startOperation := func(name, method, path string) {
		state.mu.Lock()
		state.operation = name
		state.method = method
		state.path = path
		state.query = ""
		state.body = ""
		state.mu.Unlock()
		uiApp.SetRoot(buildWorkspace(config, state, uiApp))
	}
	quickActions := primitives.Box(
		button.New(button.TextOpt(guiText(config.Language, "search_airports", "Search airports")), button.VariantOpt(button.Outlined), button.OnClick(func() { startOperation("search", http.MethodGet, "/airports/search") })),
		button.New(button.TextOpt(guiText(config.Language, "airport_lookup", "Airport lookup")), button.VariantOpt(button.Outlined), button.OnClick(func() { startOperation("get", http.MethodGet, "/airports/") })),
		button.New(button.TextOpt(guiText(config.Language, "nearby_airports", "Nearby airports")), button.VariantOpt(button.Outlined), button.OnClick(func() { startOperation("nearby", http.MethodGet, "/airports/nearby") })),
		button.New(button.TextOpt(guiText(config.Language, "server_health", "Server health")), button.VariantOpt(button.Outlined), button.OnClick(func() { startOperation("health", http.MethodGet, "/server/healthz") })),
		button.New(button.TextOpt(guiText(config.Language, "version_information", "Version information")), button.VariantOpt(button.Outlined), button.OnClick(func() { startOperation("version", http.MethodGet, "/server/about") })),
		button.New(button.TextOpt(guiText(config.Language, "api_explorer", "API explorer")), button.VariantOpt(button.Outlined), button.OnClick(func() { startOperation("api", http.MethodGet, "/airports") })),
	).Gap(8)
	form := primitives.Box(
		primitives.Text(guiText(config.Language, "method", "Method")).FontSize(14).Bold(), methodField,
		primitives.Text(guiText(config.Language, "path", "Path")).FontSize(14).Bold(), pathField,
		primitives.Text(guiText(config.Language, "query_parameters", "Query parameters")).FontSize(14).Bold(), queryField,
		primitives.Text(guiText(config.Language, "request_body", "Request body")).FontSize(14).Bold(), bodyField,
	).Padding(12).Gap(8)
	switch v.operation {
	case "search":
		form = primitives.Box(primitives.Text(guiText(config.Language, "search_query", "Search query")).FontSize(14).Bold(), queryField).Padding(12).Gap(8)
	case "get":
		form = primitives.Box(primitives.Text(pathLabel).FontSize(14).Bold(), pathField).Padding(12).Gap(8)
	case "nearby":
		form = primitives.Box(
			primitives.Text(guiText(config.Language, "nearby_query", "Coordinates (lat=...&lon=...&n=...)")).FontSize(14).Bold(), queryField,
		).Padding(12).Gap(8)
	case "health", "version":
		form = primitives.Box(primitives.Text(guiText(config.Language, "read_only_operation", "This operation reads the configured server.")).FontSize(14)).Padding(12).Gap(8)
	}
	content := primitives.Box(
		primitives.Text(guiText(config.Language, "client_title", "Airports API Client")).FontSize(28).Bold(),
		primitives.Text(guiText(config.Language, "connected_to", "Connected to")+" "+v.serverURL).FontSize(14),
		primitives.HBox(setup).Gap(8),
		quickActions,
		primitives.Text(guiText(config.Language, "operation_"+v.operation, guiText(config.Language, "request", "Request"))).FontSize(20).Bold(),
		form,
		request,
		status,
		primitives.Text(guiText(config.Language, "response", "Response")).FontSize(18).Bold(),
		primitives.Box(result).Padding(12).Rounded(8),
	).Padding(28).Gap(10)
	return scrollview.New(content, scrollview.DirectionOpt(scrollview.Vertical))
}

func guiText(language, key, fallback string) string {
	lookupKey := "client.gui." + key
	translated := i18n.TranslateText(language, lookupKey)
	if translated == lookupKey || translated == "" {
		return fallback
	}
	return translated
}

func defaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func showErrorDialog(config NativeGUIConfig, uiApp *app.App, restore widget.Focusable, message string) {
	showDialog(config, uiApp, restore, guiText(config.Language, "error_title", "Error"), message)
}

func showDialog(config NativeGUIConfig, uiApp *app.App, restore widget.Focusable, title, message string) {
	context := uiApp.Window().Context()
	alert := dialog.New(
		dialog.Title(title),
		dialog.Content(primitives.Text(message)),
		dialog.Actions(dialog.Action{Label: guiText(config.Language, "dialog_ok", "OK"), Default: true}),
		dialog.DismissibleOpt(true),
		dialog.EscapeToCloseOpt(true),
		dialog.OnClose(func() {
			if restore != nil {
				context.RequestFocus(restore)
			}
		}),
	)
	alert.Show(context)
}

func prettyResponse(value string) string {
	var formatted strings.Builder
	if json.Indent(&formatted, []byte(value), "", "  ") == nil {
		return formatted.String()
	}
	return value
}

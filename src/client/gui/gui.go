//go:build gui && !freebsd && !netbsd && !openbsd

package gui

import (
	"context"
	"errors"
	"net/url"
	"strings"
	"sync"

	"github.com/gogpu/gg"
	_ "github.com/gogpu/gg/gpu"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/ui/app"
	"github.com/gogpu/ui/core/button"
	"github.com/gogpu/ui/core/scrollview"
	"github.com/gogpu/ui/core/textfield"
	"github.com/gogpu/ui/primitives"
	"github.com/gogpu/ui/render"

	"github.com/apimgr/airports/src/common/display"
)

// Config supplies the client operations so the GUI stays independent of the
// command package and can be embedded by other clients.
type Config struct {
	ServerURL string
	Token     string
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
func LaunchGUI(config Config) error {
	if !IsGUIAvailable() {
		return errors.New("GUI requires a local display session; use the terminal client over SSH or Mosh")
	}
	if config.OnConnect == nil || config.OnRequest == nil || config.Save == nil {
		return errors.New("GUI requires connect, request, and save callbacks")
	}

	window := gogpu.NewApp(gogpu.DefaultConfig().WithTitle("Airports Client").WithSize(1024, 760))
	uiApp := app.New(
		app.WithWindowProvider(window),
		app.WithPlatformProvider(window),
		app.WithEventSource(window.EventSource()),
		app.WithRenderMode(app.RenderModeFrameworkManaged),
	)
	state := &viewState{serverURL: config.ServerURL, token: config.Token}
	uiApp.SetRoot(buildSetup(config, state, uiApp))

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
				state.setMessage("Graphics initialization failed: " + err.Error())
				return
			}
		}
		uiApp.Frame()
		cw, ch := canvas.Size()
		if cw != w || ch != h {
			if err := canvas.Resize(w, h); err != nil {
				state.setMessage("Window resize failed: " + err.Error())
				return
			}
			cw, ch = w, h
		}
		canvas.Draw(func(cc *gg.Context) {
			uiApp.Window().DrawTo(render.NewCanvas(cc, cw, ch))
		})
		surfaceWidth, surfaceHeight := dc.SurfaceSize()
		if err := canvas.RenderDirect(dc.RenderTarget().SurfaceView(), surfaceWidth, surfaceHeight); err != nil {
			state.setMessage("Rendering failed: " + err.Error())
		}
	})
	window.OnClose(func() { gg.CloseAccelerator() })
	return window.Run()
}

type viewState struct {
	mu        sync.RWMutex
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

func buildSetup(config Config, state *viewState, uiApp *app.App) *scrollview.Widget {
	urlField := textfield.New(
		textfield.A11yLabel("Airports server URL"),
		textfield.Placeholder("https://airports.example.org"),
		textfield.InitialValue(state.serverURL),
		textfield.OnChange(func(value string) { state.mu.Lock(); state.serverURL = strings.TrimSpace(value); state.mu.Unlock() }),
	)
	tokenField := textfield.New(
		textfield.A11yLabel("Access token (optional)"),
		textfield.Placeholder("Access token (optional)"),
		textfield.InputTypeOpt(textfield.TypePassword),
		textfield.InitialValue(state.token),
		textfield.OnChange(func(value string) { state.mu.Lock(); state.token = value; state.mu.Unlock() }),
	)
	status := primitives.TextFn(func() string { return state.snapshot().message }).FontSize(14)
	connect := button.New(button.TextOpt("Connect and save"), button.OnClick(func() {
		v := state.snapshot()
		parsed, err := url.ParseRequestURI(v.serverURL)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
			state.setMessage("Enter a valid HTTP or HTTPS server URL without credentials, query, or fragment.")
			return
		}
		state.setMessage("Checking server connection…")
		info, err := config.OnConnect(context.Background(), strings.TrimRight(v.serverURL, "/"), v.token)
		if err != nil {
			state.setMessage("Connection failed: " + err.Error())
			return
		}
		if err := config.Save(strings.TrimRight(v.serverURL, "/"), v.token); err != nil {
			state.setMessage("Connected, but saving configuration failed: " + err.Error())
			return
		}
		state.setMessage("Connected successfully. " + info)
		uiApp.SetRoot(buildWorkspace(config, state, uiApp))
	}))
	content := primitives.Box(
		primitives.Text("Airports").FontSize(30).Bold(),
		primitives.Text("Connect to your Airports server").FontSize(17),
		primitives.Text("Enter the server address and optional access token. The connection is checked before it is saved.").FontSize(14),
		primitives.Text("Server URL").FontSize(14).Bold(), urlField,
		primitives.Text("Access token").FontSize(14).Bold(), tokenField,
		connect,
		status,
	).Padding(32).Gap(14)
	return scrollview.New(content, scrollview.DirectionOpt(scrollview.Vertical))
}

func buildWorkspace(config Config, state *viewState, uiApp *app.App) *scrollview.Widget {
	v := state.snapshot()
	methodField := textfield.New(
		textfield.A11yLabel("HTTP method"),
		textfield.Placeholder("GET"),
		textfield.InitialValue("GET"),
		textfield.OnChange(func(value string) {
			state.mu.Lock()
			state.method = strings.ToUpper(strings.TrimSpace(value))
			state.mu.Unlock()
		}),
	)
	pathField := textfield.New(
		textfield.A11yLabel("API path"),
		textfield.Placeholder("/airports"),
		textfield.InitialValue("/airports"),
		textfield.OnChange(func(value string) {
			state.mu.Lock()
			state.path = strings.TrimSpace(value)
			state.mu.Unlock()
		}),
	)
	queryField := textfield.New(
		textfield.A11yLabel("Query parameters"),
		textfield.Placeholder("limit=25&search=JFK"),
		textfield.OnChange(func(value string) {
			state.mu.Lock()
			state.query = value
			state.mu.Unlock()
		}),
	)
	bodyField := textfield.New(
		textfield.A11yLabel("Request body (JSON)"),
		textfield.Placeholder("Request body (JSON, optional)"),
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
			return "Response will appear here."
		}
		return value
	}).FontSize(13)
	request := button.New(button.TextOpt("Send request"), button.OnClick(func() {
		requestState := state.snapshot()
		method := strings.ToUpper(strings.TrimSpace(requestState.method))
		if method == "" {
			method = "GET"
		}
		if strings.ContainsAny(method, " \t\r\n") {
			state.setMessage("HTTP method must be a single token.")
			return
		}
		path := strings.TrimSpace(requestState.path)
		parsedPath, err := url.ParseRequestURI(path)
		if err != nil || !strings.HasPrefix(path, "/") || parsedPath.IsAbs() {
			state.setMessage("API path must be a valid absolute path beginning with /.")
			return
		}
		query, err := url.ParseQuery(requestState.query)
		if err != nil {
			state.setMessage("Query parameters are invalid: " + err.Error())
			return
		}
		state.setMessage("Request in progress…")
		response, err := config.OnRequest(context.Background(), requestState.serverURL, requestState.token, method, path, query, requestState.body)
		if err != nil {
			state.setMessage("Request failed: " + err.Error())
			state.setResult("")
			return
		}
		state.setMessage("Request completed")
		state.setResult(response)
	}))
	setup := button.New(
		button.TextOpt("Change server"),
		button.VariantOpt(button.Outlined),
		button.OnClick(func() { uiApp.SetRoot(buildSetup(config, state, uiApp)) }),
	)
	content := primitives.Box(
		primitives.Text("Airports API Client").FontSize(28).Bold(),
		primitives.Text("Connected to "+v.serverURL).FontSize(14),
		primitives.HBox(setup).Gap(8),
		primitives.Text("Request").FontSize(20).Bold(),
		primitives.Text("Method").FontSize(14).Bold(), methodField,
		primitives.Text("Path").FontSize(14).Bold(), pathField,
		primitives.Text("Query parameters").FontSize(14).Bold(), queryField,
		primitives.Text("Request body").FontSize(14).Bold(), bodyField,
		request,
		status,
		primitives.Text("Response").FontSize(18).Bold(),
		primitives.Box(result).Padding(12).Rounded(8),
	).Padding(28).Gap(10)
	return scrollview.New(content, scrollview.DirectionOpt(scrollview.Vertical))
}

// Package tui provides the interactive terminal client for Airports.
package tui

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/apimgr/airports/src/common/terminal"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const actionTimeout = 30 * time.Second

type Config struct {
	ServerURL string
	Token     string
	Language  string
	Theme     string
	UseColor  bool
}

type Request struct {
	Operation string
	Query     string
	Code      string
	Latitude  string
	Longitude string
	Limit     string
	Method    string
	Path      string
	Body      string
}

type Actions struct {
	TestConnection func(context.Context, string, string) (string, error)
	Execute        func(context.Context, string, string, Request) (string, error)
}

type page int

const (
	pageMenu page = iota
	pageSetup
	pageInput
	pageResult
	pageHelp
)

type operation int

const (
	opSearch operation = iota
	opGet
	opNearby
	opHealth
	opVersion
	opAPI
	opSetup
)

type model struct {
	config      Config
	actions     Actions
	page        page
	operation   operation
	selected    int
	filter      string
	inputs      []textinput.Model
	viewport    viewport.Model
	width       int
	height      int
	layout      LayoutConfig
	styles      themeStyles
	status      string
	statusError bool
	result      string
	loading     bool
}

type actionResult struct {
	title string
	body  string
	err   error
	setup bool
}

var menuItems = []struct {
	operation   operation
	label       string
	description string
}{
	{opSearch, "Search airports", "Find airports by name, city, country, or code."},
	{opGet, "Airport lookup", "Look up an airport by ICAO or IATA code."},
	{opNearby, "Nearby airports", "Find airports around geographic coordinates."},
	{opHealth, "Server health", "Inspect the configured server health response."},
	{opVersion, "Version information", "Show client and server version details."},
	{opAPI, "API explorer", "Send a request to any server API endpoint."},
	{opSetup, "Connection settings", "Test or change the server URL and API token."},
}

// Run starts the interactive client and returns any configuration changes
// made by the setup screen. The caller supplies network operations so the TUI
// reuses the same authenticated, localized HTTP client as command mode.
func Run(config Config, actions Actions) (Config, error) {
	m := newModel(config, actions)
	program := tea.NewProgram(m, tea.WithAltScreen())
	final, err := program.Run()
	if err != nil {
		return config, err
	}
	if result, ok := final.(model); ok {
		return result.config, nil
	}
	return config, errors.New("TUI exited without returning its model")
}

func newModel(config Config, actions Actions) model {
	if config.Theme != "light" {
		config.Theme = "dark"
	}
	size := terminal.GetTerminalSize()
	width, height := size.Cols, size.Rows
	viewportModel := viewport.New(width-4, height-8)
	m := model{
		config:   config,
		actions:  actions,
		page:     pageMenu,
		width:    width,
		height:   height,
		layout:   GetLayoutConfig(terminalSizeMode(width, height)),
		styles:   stylesForTheme(config.Theme, config.UseColor),
		viewport: viewportModel,
	}
	if strings.TrimSpace(config.ServerURL) == "" {
		m.page = pageSetup
		m.operation = opSetup
		m.inputs = []textinput.Model{newInput("Server URL", ""), newPasswordInput("API token", "")}
		m.inputs[0].Focus()
		m.status = "No server configured. Enter a server URL to begin."
	}
	return m
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(message tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := message.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout = GetLayoutConfig(terminalSizeMode(msg.Width, msg.Height))
		m.resizeInputs()
		m.resizeViewport()
		return m, nil
	case actionResult:
		m.loading = false
		if msg.setup {
			if msg.err != nil {
				m.setError("Connection failed: " + msg.err.Error())
				return m, nil
			}
			m.config.ServerURL = strings.TrimRight(m.inputs[0].Value(), "/")
			m.config.Token = m.inputs[1].Value()
			m.setStatus("Connected. Configuration will be saved when you exit. "+msg.body, false)
			m.page = pageMenu
			m.inputs = nil
			return m, nil
		}
		m.page = pageResult
		m.result = msg.body
		if msg.err != nil {
			m.setError("Request failed: " + msg.err.Error())
		} else {
			m.setStatus(msg.title+" completed", false)
		}
		m.viewport.SetContent(m.result)
		m.resizeViewport()
		return m, nil
	case tea.KeyMsg:
		return m.updateKey(msg)
	}
	if m.page == pageInput || m.page == pageSetup {
		return m.updateInputs(message)
	}
	if m.page == pageResult {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(message)
		return m, cmd
	}
	return m, nil
}

func (m model) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.loading {
		return m, nil
	}
	key := msg.String()
	switch m.page {
	case pageMenu:
		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.filter == "" {
				return m, tea.Quit
			}
			m.filter += key
			m.selected = 0
		case "up", "k":
			if indices := m.filteredMenu(); len(indices) > 0 {
				m.selected = (m.selected - 1 + len(indices)) % len(indices)
			}
		case "down", "j":
			if indices := m.filteredMenu(); len(indices) > 0 {
				m.selected = (m.selected + 1) % len(indices)
			}
		case "enter":
			if indices := m.filteredMenu(); len(indices) > 0 {
				return m.chooseMenuItem(indices[m.selected])
			}
		case "?":
			m.page = pageHelp
		case "/":
			m.filter = ""
			m.selected = 0
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = string([]rune(m.filter)[:len([]rune(m.filter))-1])
				m.selected = 0
			}
		case "esc":
			m.filter = ""
			m.selected = 0
		default:
			if len([]rune(key)) == 1 && key >= " " && key <= "~" {
				m.filter += key
				m.selected = 0
			}
		}
	case pageSetup:
		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			if m.config.ServerURL == "" {
				return m, tea.Quit
			}
			m.page = pageMenu
			m.inputs = nil
			m.status = ""
		case "tab", "down":
			m.focusNextInput()
		case "shift+tab", "up":
			m.focusPreviousInput()
		case "enter":
			return m.submitSetup()
		}
		return m.updateInputs(msg)
	case pageInput:
		switch key {
		case "ctrl+c":
			return m, tea.Quit
		case "esc":
			m.page = pageMenu
			m.inputs = nil
			m.status = ""
		case "tab", "down":
			m.focusNextInput()
		case "shift+tab", "up":
			m.focusPreviousInput()
		case "enter":
			return m.submitRequest()
		}
		return m.updateInputs(msg)
	case pageResult:
		switch key {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "esc", "enter":
			m.page = pageMenu
			m.status = ""
			m.result = ""
			m.viewport.SetContent("")
		}
	case pageHelp:
		if key == "esc" || key == "enter" || key == "?" {
			m.page = pageMenu
		}
	}
	return m, nil
}

func (m model) chooseMenuItem(index int) (tea.Model, tea.Cmd) {
	item := menuItems[index]
	m.operation = item.operation
	m.status = ""
	switch item.operation {
	case opSetup:
		m.page = pageSetup
		m.inputs = []textinput.Model{newInput("Server URL", m.config.ServerURL), newPasswordInput("API token", m.config.Token)}
		m.focusInput(0)
		return m, nil
	case opSearch:
		m.page = pageInput
		m.inputs = []textinput.Model{newInput("Search query", "")}
	case opGet:
		m.page = pageInput
		m.inputs = []textinput.Model{newInput("ICAO or IATA code", "")}
	case opNearby:
		m.page = pageInput
		m.inputs = []textinput.Model{newInput("Latitude (-90 to 90)", ""), newInput("Longitude (-180 to 180)", ""), newInput("Result limit", "10")}
	case opAPI:
		m.page = pageInput
		m.inputs = []textinput.Model{newInput("Method (GET, POST, PUT, PATCH, DELETE)", "GET"), newInput("API path (for example /airports)", ""), newInput("Query string (optional)", ""), newInput("JSON body (optional)", "")}
	case opHealth, opVersion:
		return m.executeRequest(Request{Operation: operationName(item.operation)})
	}
	m.focusInput(0)
	m.resizeInputs()
	return m, nil
}

func (m model) submitSetup() (tea.Model, tea.Cmd) {
	serverURL, err := validateServerURL(m.inputs[0].Value())
	if err != nil {
		m.setError(err.Error())
		return m, nil
	}
	if m.actions.TestConnection == nil {
		m.setError("Server connection testing is not configured.")
		return m, nil
	}
	m.loading = true
	m.status = "Testing server connection…"
	token := m.inputs[1].Value()
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		body, testErr := m.actions.TestConnection(ctx, serverURL, token)
		return actionResult{title: "Connection test", body: body, err: testErr, setup: true}
	}
}

func (m model) submitRequest() (tea.Model, tea.Cmd) {
	request := Request{Operation: operationName(m.operation)}
	switch m.operation {
	case opSearch:
		request.Query = strings.TrimSpace(m.inputs[0].Value())
		if request.Query == "" {
			m.setError("Enter a search query.")
			return m, nil
		}
		if len([]rune(request.Query)) > 200 {
			m.setError("Search query must be 200 characters or fewer.")
			return m, nil
		}
	case opGet:
		request.Code = strings.ToUpper(strings.TrimSpace(m.inputs[0].Value()))
		if !validAirportCode(request.Code) {
			m.setError("Enter a 3 or 4 character airport code using letters or digits.")
			return m, nil
		}
	case opNearby:
		request.Latitude = strings.TrimSpace(m.inputs[0].Value())
		request.Longitude = strings.TrimSpace(m.inputs[1].Value())
		request.Limit = strings.TrimSpace(m.inputs[2].Value())
		if !validCoordinate(request.Latitude, -90, 90) || !validCoordinate(request.Longitude, -180, 180) {
			m.setError("Enter a valid latitude and longitude.")
			return m, nil
		}
		if request.Limit != "" {
			limit, err := strconv.Atoi(request.Limit)
			if err != nil || limit < 1 || limit > 1000 {
				m.setError("Result limit must be between 1 and 1000.")
				return m, nil
			}
		}
	case opAPI:
		request.Method = strings.ToUpper(strings.TrimSpace(m.inputs[0].Value()))
		request.Path = strings.TrimSpace(m.inputs[1].Value())
		request.Query = strings.TrimSpace(m.inputs[2].Value())
		request.Body = strings.TrimSpace(m.inputs[3].Value())
		switch request.Method {
		case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS":
		default:
			m.setError("Choose a supported HTTP method: GET, POST, PUT, PATCH, DELETE, HEAD, or OPTIONS.")
			return m, nil
		}
		if !strings.HasPrefix(request.Path, "/") || strings.ContainsAny(request.Path, "\r\n#") {
			m.setError("API path must start with / and cannot contain a fragment.")
			return m, nil
		}
		if strings.ContainsAny(request.Query, "\r\n#?") {
			m.setError("Enter a query string without a leading ? or fragment.")
			return m, nil
		}
		if request.Body != "" && !json.Valid([]byte(request.Body)) {
			m.setError("Request body must be valid JSON.")
			return m, nil
		}
	}
	return m.executeRequest(request)
}

func (m model) executeRequest(request Request) (tea.Model, tea.Cmd) {
	if strings.TrimSpace(m.config.ServerURL) == "" {
		m.page = pageSetup
		m.inputs = []textinput.Model{newInput("Server URL", ""), newPasswordInput("API token", "")}
		m.focusInput(0)
		m.status = "Configure a server before making requests."
		return m, nil
	}
	if m.actions.Execute == nil {
		m.setError("Client operations are not configured.")
		return m, nil
	}
	m.loading = true
	m.status = "Request in progress…"
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), actionTimeout)
		defer cancel()
		body, err := m.actions.Execute(ctx, m.config.ServerURL, m.config.Token, request)
		return actionResult{title: operationLabel(m.operation), body: body, err: err}
	}
}

func (m model) updateInputs(message tea.Msg) (tea.Model, tea.Cmd) {
	commands := make([]tea.Cmd, len(m.inputs))
	for index := range m.inputs {
		m.inputs[index], commands[index] = m.inputs[index].Update(message)
	}
	return m, tea.Batch(commands...)
}

func (m *model) focusInput(index int) {
	for i := range m.inputs {
		if i == index {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m *model) focusNextInput() {
	for i := range m.inputs {
		if m.inputs[i].Focused() {
			m.inputs[i].Blur()
			next := (i + 1) % len(m.inputs)
			m.inputs[next].Focus()
			return
		}
	}
	if len(m.inputs) > 0 {
		m.inputs[0].Focus()
	}
}

func (m *model) focusPreviousInput() {
	for i := range m.inputs {
		if m.inputs[i].Focused() {
			m.inputs[i].Blur()
			previous := (i - 1 + len(m.inputs)) % len(m.inputs)
			m.inputs[previous].Focus()
			return
		}
	}
	if len(m.inputs) > 0 {
		m.inputs[0].Focus()
	}
}

func (m *model) resizeInputs() {
	width := m.width - 14
	if width < 12 {
		width = 12
	}
	if width > 80 {
		width = 80
	}
	for index := range m.inputs {
		m.inputs[index].Width = width
	}
}

func (m *model) resizeViewport() {
	width, height := m.width-4, m.height-8
	if width < 16 {
		width = 16
	}
	if height < 3 {
		height = 3
	}
	m.viewport.Width = width
	m.viewport.Height = height
}

func (m model) View() string {
	if m.width < 20 || m.height < 8 {
		return "Resize terminal to at least 20 columns by 8 rows."
	}
	var content string
	switch m.page {
	case pageSetup:
		content = m.viewSetup()
	case pageInput:
		content = m.viewInput()
	case pageResult:
		content = m.viewResult()
	case pageHelp:
		content = m.viewHelp()
	default:
		content = m.viewMenu()
	}
	if m.layout.ShowBorders {
		content = lipgloss.NewStyle().Border(m.styles.border).BorderForeground(m.styles.borderColor).Padding(0, 1).Width(max(12, m.width-4)).Render(content)
	}
	footer := "↑/↓ navigate • Enter select • ? help • q quit"
	if m.page != pageMenu {
		footer = "Tab move between fields • Enter submit • Esc back • ? help • q quit"
	}
	if m.page == pageHelp {
		footer = "Esc or ? close help • q quit"
	}
	if m.status != "" {
		status := truncate(m.status, max(10, m.width-6))
		if m.statusError {
			status = m.styles.error.Render(status)
		} else {
			status = m.styles.muted.Render(status)
		}
		content += "\n\n" + status
	}
	if m.layout.ShowFooter {
		content += "\n\n" + m.styles.muted.Render(truncate(footer, m.width-4))
	}
	return m.styles.app.Width(m.width).Height(m.height).Render(content)
}

func (m model) viewMenu() string {
	var lines []string
	lines = append(lines, m.styles.heading.Render("Airports API"))
	server := "No server configured"
	if m.config.ServerURL != "" {
		server = "Server: " + truncateMiddle(m.config.ServerURL, max(12, m.width-24))
	}
	lines = append(lines, m.styles.muted.Render(server), "")
	if m.filter != "" {
		lines = append(lines, m.styles.muted.Render("Filter: "+m.filter), "")
	} else if m.width >= 60 {
		lines = append(lines, m.styles.muted.Render("Press / to filter operations"), "")
	}
	indices := m.filteredMenu()
	if len(indices) == 0 {
		lines = append(lines, "No operations match this filter.")
	}
	for visibleIndex, index := range indices {
		item := menuItems[index]
		label := item.label
		if visibleIndex == m.selected {
			lines = append(lines, m.styles.selected.Render("› "+label))
		} else {
			lines = append(lines, "  "+label)
		}
		if visibleIndex == m.selected && m.width >= 60 {
			lines = append(lines, m.styles.muted.Render("    "+truncate(item.description, m.width-12)))
		}
	}
	return strings.Join(lines, "\n")
}

func (m model) filteredMenu() []int {
	needle := strings.ToLower(strings.TrimSpace(m.filter))
	indices := make([]int, 0, len(menuItems))
	for index, item := range menuItems {
		if needle == "" || strings.Contains(strings.ToLower(item.label+" "+item.description), needle) {
			indices = append(indices, index)
		}
	}
	return indices
}

func (m model) viewSetup() string {
	lines := []string{m.styles.heading.Render("Connect to an Airports server"), "", "Enter an HTTP or HTTPS URL. The token is optional.", ""}
	return m.viewInputs(lines)
}

func (m model) viewInput() string {
	lines := []string{m.styles.heading.Render(operationLabel(m.operation)), ""}
	return m.viewInputs(lines)
}

func (m model) viewInputs(lines []string) string {
	for index := range m.inputs {
		lines = append(lines, m.inputs[index].View())
	}
	return strings.Join(lines, "\n")
}

func (m model) viewResult() string {
	title := m.styles.heading.Render(operationLabel(m.operation))
	if m.loading {
		return title + "\n\nWorking…"
	}
	return title + "\n\n" + m.viewport.View()
}

func (m model) viewHelp() string {
	return m.styles.heading.Render("Keyboard help") + "\n\n" + strings.Join([]string{
		"↑/↓ or j/k  Move through the menu",
		"Enter       Open an operation or submit a form",
		"Tab         Move to the next form field",
		"Shift+Tab   Move to the previous form field",
		"Esc         Return to the previous screen",
		"?           Show or close this help screen",
		"q / Ctrl+C  Quit",
		"On results, use ↑/↓, PgUp/PgDn, Home/End to scroll.",
	}, "\n")
}

func (m *model) setStatus(value string, isError bool) {
	m.status, m.statusError = value, isError
}

func (m *model) setError(value string) { m.setStatus(value, true) }

func newInput(prompt, value string) textinput.Model {
	input := textinput.New()
	input.Prompt = prompt + ": "
	input.Placeholder = ""
	input.SetValue(value)
	input.CharLimit = 256
	return input
}

func newPasswordInput(prompt, value string) textinput.Model {
	input := newInput(prompt, value)
	input.EchoMode = textinput.EchoPassword
	input.EchoCharacter = '•'
	return input
}

func validateServerURL(value string) (string, error) {
	value = strings.TrimSpace(value)
	parsed, err := url.ParseRequestURI(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("server URL must include an http or https scheme and host, without credentials, query, or fragment")
	}
	parsed.Path = strings.TrimRight(parsed.Path, "/")
	return strings.TrimRight(parsed.String(), "/"), nil
}

func validAirportCode(value string) bool {
	if len(value) < 3 || len(value) > 4 {
		return false
	}
	for _, character := range value {
		if (character < 'A' || character > 'Z') && (character < '0' || character > '9') {
			return false
		}
	}
	return true
}

func validCoordinate(value string, minimum, maximum float64) bool {
	number, err := strconv.ParseFloat(value, 64)
	return err == nil && number >= minimum && number <= maximum
}

func operationName(value operation) string {
	switch value {
	case opSearch:
		return "search"
	case opGet:
		return "get"
	case opNearby:
		return "nearby"
	case opAPI:
		return "api"
	case opHealth:
		return "health"
	case opVersion:
		return "version"
	default:
		return ""
	}
}

func operationLabel(value operation) string {
	switch value {
	case opSearch:
		return "Airport search"
	case opGet:
		return "Airport lookup"
	case opNearby:
		return "Nearby airports"
	case opAPI:
		return "API explorer"
	case opHealth:
		return "Server health"
	case opVersion:
		return "Version information"
	default:
		return "Connection settings"
	}
}

var _ tea.Model = model{}

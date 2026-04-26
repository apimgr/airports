package admin

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handler wraps admin HTTP handlers
type Handler struct {
	auth      *AuthManager
	startTime time.Time
	version   string
	commit    string
	buildDate string
}

// NewHandler creates a new admin handler
func NewHandler(adminUser, adminPass, apiToken string, sessionTimeout int, sslEnabled bool, version, commit, buildDate string) *Handler {
	return &Handler{
		auth:      NewAuthManager(adminUser, adminPass, apiToken, sessionTimeout, sslEnabled),
		startTime: time.Now(),
		version:   version,
		commit:    commit,
		buildDate: buildDate,
	}
}

// RegisterRoutes registers admin routes on a chi router
func (h *Handler) RegisterRoutes(r chi.Router) {
	// Admin web routes (session auth)
	r.Get("/admin", h.requireAuth(h.handleDashboard))
	r.Get("/admin/", h.requireAuth(h.handleDashboard))
	r.Get("/admin/login", h.handleLogin)
	r.Post("/admin/login", h.handleLoginPost)
	r.Get("/admin/logout", h.handleLogout)
	r.Get("/admin/settings", h.requireAuth(h.handleSettings))

	// Admin API routes (bearer token auth)
	r.Route("/api/v1/admin", func(r chi.Router) {
		r.Get("/status", h.requireAPIAuth(h.apiStatus))
		r.Get("/config", h.requireAPIAuth(h.apiConfig))
		r.Put("/config", h.requireAPIAuth(h.apiConfigUpdate))
		r.Post("/reload", h.requireAPIAuth(h.apiReload))
	})
}

// requireAuth middleware checks for valid admin session
func (h *Handler) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, ok := h.auth.GetSessionFromRequest(r)
		if !ok {
			http.Redirect(w, r, "/admin/login?redirect="+r.URL.Path, http.StatusSeeOther)
			return
		}

		h.auth.RefreshSession(session.ID)
		next(w, r)
	}
}

// requireAPIAuth middleware checks for valid bearer token
func (h *Handler) requireAPIAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := GetTokenFromRequest(r)
		if token == "" {
			h.jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if !h.auth.ValidateAPIToken(token) {
			h.jsonError(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

// handleLogin renders the admin login page
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.auth.GetSessionFromRequest(r); ok {
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	errorMsg := r.URL.Query().Get("error")
	h.renderLoginPage(w, errorMsg)
}

// handleLoginPost processes login form submission
func (h *Handler) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/admin/login?error=Invalid+form+data", http.StatusSeeOther)
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if !h.auth.Authenticate(username, password) {
		log.Printf("[Admin] Failed login attempt for user: %s from %s", username, GetClientIP(r))
		http.Redirect(w, r, "/admin/login?error=Invalid+credentials", http.StatusSeeOther)
		return
	}

	session := h.auth.CreateSession(username, GetClientIP(r))
	h.auth.SetSessionCookie(w, session)

	log.Printf("[Admin] Successful login for user: %s from %s", username, GetClientIP(r))

	redirect := r.FormValue("redirect")
	if redirect == "" || redirect == "/admin/login" {
		redirect = "/admin"
	}
	http.Redirect(w, r, redirect, http.StatusSeeOther)
}

// handleLogout handles admin logout
func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if session, ok := h.auth.GetSessionFromRequest(r); ok {
		h.auth.DeleteSession(session.ID)
		log.Printf("[Admin] User logged out: %s", session.Username)
	}
	h.auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
}

// handleDashboard renders the admin dashboard
func (h *Handler) handleDashboard(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	h.renderDashboardPage(w, map[string]interface{}{
		"version":       h.version,
		"commit":        h.commit,
		"buildDate":     h.buildDate,
		"goVersion":     runtime.Version(),
		"uptime":        formatDuration(time.Since(h.startTime)),
		"goroutines":    runtime.NumGoroutine(),
		"memAlloc":      formatBytes(m.Alloc),
		"memTotal":      formatBytes(m.TotalAlloc),
		"numCPU":        runtime.NumCPU(),
	})
}

// handleSettings renders the settings page
func (h *Handler) handleSettings(w http.ResponseWriter, r *http.Request) {
	h.renderSettingsPage(w)
}

// API handlers

func (h *Handler) apiStatus(w http.ResponseWriter, r *http.Request) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	status := map[string]interface{}{
		"status":     "ok",
		"version":    h.version,
		"commit":     h.commit,
		"build_date": h.buildDate,
		"go_version": runtime.Version(),
		"uptime":     time.Since(h.startTime).String(),
		"memory": map[string]interface{}{
			"alloc":       m.Alloc,
			"total_alloc": m.TotalAlloc,
			"sys":         m.Sys,
		},
		"goroutines": runtime.NumGoroutine(),
	}

	h.jsonResponse(w, status, http.StatusOK)
}

func (h *Handler) apiConfig(w http.ResponseWriter, r *http.Request) {
	cfg := map[string]interface{}{
		"server": map[string]interface{}{
			"version": h.version,
		},
	}
	h.jsonResponse(w, cfg, http.StatusOK)
}

func (h *Handler) apiConfigUpdate(w http.ResponseWriter, r *http.Request) {
	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		h.jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Config updates would be applied here
	h.jsonResponse(w, map[string]string{"status": "updated"}, http.StatusOK)
}

func (h *Handler) apiReload(w http.ResponseWriter, r *http.Request) {
	log.Println("[Admin API] Configuration reload requested")
	h.jsonResponse(w, map[string]string{"status": "reloaded"}, http.StatusOK)
}

// Helper functions

func (h *Handler) jsonResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) jsonError(w http.ResponseWriter, message string, status int) {
	h.jsonResponse(w, map[string]string{"error": message}, status)
}

func formatDuration(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Page rendering functions

func (h *Handler) renderLoginPage(w http.ResponseWriter, errorMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin Login - Airports API</title>
    <style>
        :root {
            --bg-color: #282a36;
            --fg-color: #f8f8f2;
            --accent: #bd93f9;
            --red: #ff5555;
            --green: #50fa7b;
            --input-bg: #44475a;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: var(--bg-color);
            color: var(--fg-color);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .login-container {
            background: var(--input-bg);
            padding: 2rem;
            border-radius: 8px;
            width: 100%%;
            max-width: 400px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.3);
        }
        h1 { text-align: center; margin-bottom: 1.5rem; color: var(--accent); }
        .error { background: var(--red); color: #fff; padding: 0.75rem; border-radius: 4px; margin-bottom: 1rem; }
        label { display: block; margin-bottom: 0.5rem; font-weight: 500; }
        input[type="text"], input[type="password"] {
            width: 100%%;
            padding: 0.75rem;
            border: none;
            border-radius: 4px;
            background: var(--bg-color);
            color: var(--fg-color);
            margin-bottom: 1rem;
            font-size: 1rem;
        }
        input:focus { outline: 2px solid var(--accent); }
        button {
            width: 100%%;
            padding: 0.75rem;
            background: var(--accent);
            color: var(--bg-color);
            border: none;
            border-radius: 4px;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            transition: opacity 0.2s;
        }
        button:hover { opacity: 0.9; }
    </style>
</head>
<body>
    <div class="login-container">
        <h1>Admin Login</h1>
        %s
        <form method="POST" action="/admin/login">
            <label for="username">Username</label>
            <input type="text" id="username" name="username" required autofocus>
            <label for="password">Password</label>
            <input type="password" id="password" name="password" required>
            <button type="submit">Login</button>
        </form>
    </div>
</body>
</html>`, func() string {
		if errorMsg != "" {
			return `<div class="error">` + errorMsg + `</div>`
		}
		return ""
	}())
}

func (h *Handler) renderDashboardPage(w http.ResponseWriter, data map[string]interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin Dashboard - Airports API</title>
    <style>
        :root {
            --bg-color: #282a36;
            --fg-color: #f8f8f2;
            --accent: #bd93f9;
            --green: #50fa7b;
            --input-bg: #44475a;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: var(--bg-color);
            color: var(--fg-color);
            min-height: 100vh;
        }
        nav {
            background: var(--input-bg);
            padding: 1rem 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        nav h1 { color: var(--accent); font-size: 1.25rem; }
        nav a { color: var(--fg-color); text-decoration: none; margin-left: 1rem; }
        nav a:hover { color: var(--accent); }
        .container { max-width: 1200px; margin: 0 auto; padding: 2rem; }
        .cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 1rem; }
        .card {
            background: var(--input-bg);
            padding: 1.5rem;
            border-radius: 8px;
        }
        .card h3 { color: var(--accent); margin-bottom: 0.5rem; font-size: 0.875rem; text-transform: uppercase; }
        .card p { font-size: 1.5rem; font-weight: 600; }
        .status { color: var(--green); }
    </style>
</head>
<body>
    <nav>
        <h1>Airports Admin</h1>
        <div>
            <a href="/admin">Dashboard</a>
            <a href="/admin/settings">Settings</a>
            <a href="/admin/logout">Logout</a>
        </div>
    </nav>
    <div class="container">
        <h2 style="margin-bottom: 1.5rem;">Dashboard</h2>
        <div class="cards">
            <div class="card">
                <h3>Status</h3>
                <p class="status">Healthy</p>
            </div>
            <div class="card">
                <h3>Version</h3>
                <p>%s</p>
            </div>
            <div class="card">
                <h3>Uptime</h3>
                <p>%s</p>
            </div>
            <div class="card">
                <h3>Go Version</h3>
                <p>%s</p>
            </div>
            <div class="card">
                <h3>Memory</h3>
                <p>%s</p>
            </div>
            <div class="card">
                <h3>Goroutines</h3>
                <p>%d</p>
            </div>
            <div class="card">
                <h3>CPUs</h3>
                <p>%d</p>
            </div>
        </div>
    </div>
</body>
</html>`,
		data["version"],
		data["uptime"],
		data["goVersion"],
		data["memAlloc"],
		data["goroutines"],
		data["numCPU"],
	)
}

func (h *Handler) renderSettingsPage(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Admin Settings - Airports API</title>
    <style>
        :root {
            --bg-color: #282a36;
            --fg-color: #f8f8f2;
            --accent: #bd93f9;
            --input-bg: #44475a;
        }
        * { box-sizing: border-box; margin: 0; padding: 0; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background: var(--bg-color);
            color: var(--fg-color);
            min-height: 100vh;
        }
        nav {
            background: var(--input-bg);
            padding: 1rem 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
        }
        nav h1 { color: var(--accent); font-size: 1.25rem; }
        nav a { color: var(--fg-color); text-decoration: none; margin-left: 1rem; }
        nav a:hover { color: var(--accent); }
        .container { max-width: 800px; margin: 0 auto; padding: 2rem; }
        .settings-section {
            background: var(--input-bg);
            padding: 1.5rem;
            border-radius: 8px;
            margin-bottom: 1rem;
        }
        .settings-section h3 { color: var(--accent); margin-bottom: 1rem; }
        p { margin-bottom: 0.5rem; opacity: 0.8; }
    </style>
</head>
<body>
    <nav>
        <h1>Airports Admin</h1>
        <div>
            <a href="/admin">Dashboard</a>
            <a href="/admin/settings">Settings</a>
            <a href="/admin/logout">Logout</a>
        </div>
    </nav>
    <div class="container">
        <h2 style="margin-bottom: 1.5rem;">Settings</h2>
        <div class="settings-section">
            <h3>Server Configuration</h3>
            <p>Server configuration can be edited in the server.yml file or via the API.</p>
        </div>
        <div class="settings-section">
            <h3>API</h3>
            <p>Use the Admin API at /api/v1/admin/* with a Bearer token for programmatic access.</p>
        </div>
    </div>
</body>
</html>`)
}

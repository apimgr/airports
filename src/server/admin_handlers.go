package server

import (
	"encoding/json"
	"net/http"

	"github.com/apimgr/airports/src/database"
)

// Web UI Handlers (Basic Auth)

// handleAdminDashboard shows the admin dashboard
func (s *Server) handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Admin Dashboard",
	}
	s.renderTemplate(w, "admin/dashboard.html", data)
}

// handleAdminSettings shows the settings management page
func (s *Server) handleAdminSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := database.GetAllSettings()
	if err != nil {
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":    "Server Settings",
		"Settings": settings,
	}

	s.renderTemplate(w, "admin/settings.html", data)
}

// handleAdminSettingsUpdate updates settings (web form POST)
func (s *Server) handleAdminSettingsUpdate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Update each setting from form
	for key := range r.PostForm {
		value := r.PostForm.Get(key)

		// Get existing setting to preserve metadata
		existing, err := database.GetSetting(key)
		if err != nil {
			continue // Skip unknown settings
		}

		// Update setting
		err = database.SetSetting(key, value, existing.Type, existing.Category, existing.Description)
		if err != nil {
			http.Error(w, "Failed to update setting: "+key, http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to settings page
	http.Redirect(w, r, "/admin/settings", http.StatusSeeOther)
}

// handleAdminDatabase shows database management page
func (s *Server) handleAdminDatabase(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Database Management",
	}
	s.renderTemplate(w, "admin/database.html", data)
}

// handleAdminDatabaseTest tests database connection (web form POST)
func (s *Server) handleAdminDatabaseTest(w http.ResponseWriter, r *http.Request) {
	if err := database.Ping(); err != nil {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]interface{}{
			"status":  "disconnected",
			"message": err.Error(),
		})
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "connected",
		"message": "Database connection successful",
	})
}

// handleAdminLogs shows logs viewer page
func (s *Server) handleAdminLogs(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Logs",
	}
	s.renderTemplate(w, "admin/logs.html", data)
}

// handleAdminHealth shows health status page
func (s *Server) handleAdminHealth(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Server Health",
	}
	s.renderTemplate(w, "admin/health.html", data)
}

// API Handlers (Bearer Token)

// handleAdminAPI returns admin info
func (s *Server) handleAdminAPI(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"version": "1.0.0",
		"admin":   true,
	})
}

// handleAdminSettingsAPI returns all settings as JSON
func (s *Server) handleAdminSettingsAPI(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")

	if category != "" {
		settings, err := database.GetSettingsByCategory(category)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, "FETCH_FAILED", err.Error())
			return
		}
		s.respondJSON(w, http.StatusOK, settings)
	} else {
		settings, err := database.GetAllSettings()
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, "FETCH_FAILED", err.Error())
			return
		}
		s.respondJSON(w, http.StatusOK, settings)
	}
}

// handleAdminSettingsUpdateAPI updates settings via JSON API
func (s *Server) handleAdminSettingsUpdateAPI(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Settings map[string]string `json:"settings"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON")
		return
	}

	// Update each setting
	for key, value := range req.Settings {
		// Get existing setting to preserve metadata
		existing, err := database.GetSetting(key)
		if err != nil {
			s.respondError(w, http.StatusNotFound, "SETTING_NOT_FOUND", "Setting not found: "+key)
			return
		}

		// Update setting
		err = database.SetSetting(key, value, existing.Type, existing.Category, existing.Description)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
			return
		}
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Settings updated successfully",
		"count":   len(req.Settings),
	})
}

// handleAdminDatabaseAPI returns database status
func (s *Server) handleAdminDatabaseAPI(w http.ResponseWriter, r *http.Request) {
	status := "connected"
	if err := database.Ping(); err != nil {
		status = "disconnected"
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"type":   database.GetType(),
		"status": status,
	})
}

// handleAdminDatabaseTestAPI tests database connection
func (s *Server) handleAdminDatabaseTestAPI(w http.ResponseWriter, r *http.Request) {
	if err := database.Ping(); err != nil {
		s.respondError(w, http.StatusServiceUnavailable, "DB_UNAVAILABLE", err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"status":  "ok",
		"message": "Database connection successful",
	})
}

// handleAdminLogsAPI returns available logs
func (s *Server) handleAdminLogsAPI(w http.ResponseWriter, r *http.Request) {
	s.respondJSON(w, http.StatusOK, []string{
		"access.log",
		"error.log",
		"audit.log",
	})
}

// handleAdminHealthAPI returns detailed health status
func (s *Server) handleAdminHealthAPI(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status": "healthy",
		"database": map[string]interface{}{
			"status": "connected",
			"type":   database.GetType(),
		},
		"airports": map[string]interface{}{
			"loaded": s.airports.Stats()["total_airports"],
		},
		"geoip": map[string]interface{}{
			"loaded": true,
		},
	}

	// Check database
	if err := database.Ping(); err != nil {
		health["status"] = "degraded"
		health["database"].(map[string]interface{})["status"] = "disconnected"
	}

	s.respondJSON(w, http.StatusOK, health)
}

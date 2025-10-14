package server

import (
	"encoding/json"
	"net/http"

	"github.com/apimgr/airports/src/database"
)

// handleConfig shows the configuration page
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	settings, err := database.GetAllSettings()
	if err != nil {
		http.Error(w, "Failed to load settings", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":    "Configuration",
		"Settings": settings,
	}

	s.renderTemplate(w, "base.html", data)
}

// handleConfigUpdate updates a single setting
func (s *Server) handleConfigUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	var req struct {
		Key   string `json:"key"`
		Value string `json:"value"`
		Type  string `json:"type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, "INVALID_JSON", "Invalid JSON")
		return
	}

	// Get existing setting to preserve category and description
	existing, err := database.GetSetting(req.Key)
	if err != nil {
		s.respondError(w, http.StatusNotFound, "SETTING_NOT_FOUND", "Setting not found")
		return
	}

	// Update setting
	err = database.SetSetting(req.Key, req.Value, req.Type, existing.Category, existing.Description)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Setting updated successfully",
	})
}

// handleConfigReset resets all settings to defaults
func (s *Server) handleConfigReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Method not allowed")
		return
	}

	if err := database.ResetToDefaults(); err != nil {
		s.respondError(w, http.StatusInternalServerError, "RESET_FAILED", err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Settings reset to defaults",
	})
}

// handleConfigExport exports all settings as JSON
func (s *Server) handleConfigExport(w http.ResponseWriter, r *http.Request) {
	settings, err := database.GetAllSettings()
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, "EXPORT_FAILED", err.Error())
		return
	}

	s.respondJSON(w, http.StatusOK, settings)
}

// handleConfigAPI returns settings as JSON API
func (s *Server) handleConfigAPI(w http.ResponseWriter, r *http.Request) {
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

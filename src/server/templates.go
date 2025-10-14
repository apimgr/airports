package server

import (
	"embed"
	"html/template"
	"net/http"
)

//go:embed templates/*.html
var templateFS embed.FS

//go:embed static/**/*
var staticFS embed.FS

var templates *template.Template

// initTemplates loads and parses all HTML templates
func initTemplates() error {
	var err error
	templates, err = template.ParseFS(templateFS, "templates/*.html")
	return err
}

// renderTemplate renders an HTML template with data
func (s *Server) renderTemplate(w http.ResponseWriter, name string, data interface{}) {
	err := templates.ExecuteTemplate(w, name, data)
	if err != nil {
		http.Error(w, "Template error", http.StatusInternalServerError)
	}
}

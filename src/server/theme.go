package server

// themeCookieName is the server-readable preference used for theme rendering.
const themeCookieName = "theme"

// validThemes are the only theme preference values accepted by the server.
var validThemes = map[string]bool{"light": true, "dark": true, "auto": true}

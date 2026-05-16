package swagger

import "fmt"

// renderSwaggerHTML returns the Swagger UI page that loads the embedded
// assets at assetsPrefix and fetches the OpenAPI spec from specURL.
//
// The page uses the project's site CSS for the header and overrides
// Swagger UI colors with the project theme variables so light/dark/auto
// matches the rest of the site (see AI.md PART 14 "Swagger & GraphQL
// Theming").
func renderSwaggerHTML(specURL, assetsPrefix string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="dark">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>API Documentation - Airports API</title>
    <link rel="stylesheet" href="/static/css/main.css">
    <link rel="stylesheet" href="%[2]sswagger-ui.css">
    <style>
      body { margin: 0; padding: 0; display: flex; flex-direction: column; min-height: 100vh; }
      #swagger-container { flex: 1; }
      .swagger-ui { background: var(--bg-primary); }
      .swagger-ui .topbar { display: none; }
      .swagger-ui .info { color: var(--text-primary); }
      .swagger-ui .scheme-container { background: var(--bg-secondary); }
      .swagger-ui .opblock { background: var(--bg-secondary); border-color: var(--border-color); }
      .swagger-ui .opblock-tag { color: var(--text-primary); border-color: var(--border-color); }
      .swagger-ui .opblock-summary { background: var(--bg-tertiary); }
      .swagger-ui .opblock-description { color: var(--text-secondary); }
      .swagger-ui table thead tr td, .swagger-ui table thead tr th { color: var(--text-primary); border-color: var(--border-color); }
      .swagger-ui .parameter__name { color: var(--accent-primary); }
      .swagger-ui .response-col_status { color: var(--accent-success); }
      .swagger-ui input, .swagger-ui select, .swagger-ui textarea { background: var(--bg-tertiary); color: var(--text-primary); border-color: var(--border-color); }
      .swagger-ui .btn { background: var(--accent-primary); color: white; }
    </style>
  </head>
  <body>
    <header id="main-header">
      <div class="header-container">
        <div class="header-left">
          <button class="mobile-menu-toggle" onclick="toggleMobileMenu()">&#9776;</button>
          <a class="logo" href="/">Airports API</a>
        </div>
        <nav id="main-nav" class="header-center">
          <a href="/">Home</a>
          <a href="/search">Search</a>
          <a href="/nearby">Nearby</a>
          <a href="/stats">Stats</a>
          <a href="/server/docs/swagger" class="active">API Docs</a>
          <a href="/server/docs/graphql">GraphQL</a>
        </nav>
        <div class="header-right">
          <button class="theme-toggle" onclick="toggleTheme()">
            <span class="theme-icon">Theme</span>
          </button>
        </div>
      </div>
    </header>

    <div id="swagger-container">
      <div id="swagger-ui"></div>
    </div>

    <script src="/static/js/main.js"></script>
    <script src="%[2]sswagger-ui-bundle.js"></script>
    <script src="%[2]sswagger-ui-standalone-preset.js"></script>
    <script>
      window.onload = function() {
        window.ui = SwaggerUIBundle({
          url: %[1]q,
          dom_id: '#swagger-ui',
          deepLinking: true,
          presets: [
            SwaggerUIBundle.presets.apis,
            SwaggerUIStandalonePreset
          ],
          plugins: [
            SwaggerUIBundle.plugins.DownloadUrl
          ],
          layout: "StandaloneLayout"
        });
      };
    </script>
  </body>
</html>
`, specURL, assetsPrefix)
}

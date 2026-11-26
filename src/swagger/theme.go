package swagger

import (
	"fmt"
	"html"
)

// ThemeCSS returns the external stylesheet used to synchronize Swagger UI with
// the application theme without adding inline CSS to the document.
func ThemeCSS() string {
	return `body{margin:0;display:flex;min-height:100vh;flex-direction:column}#swagger-container{flex:1}.swagger-ui{background:var(--color-bg);color:var(--color-text)}.swagger-ui .topbar{display:none}.swagger-ui .info,.swagger-ui .info .title,.swagger-ui .opblock-tag{color:var(--color-text)}.swagger-ui .scheme-container,.swagger-ui .opblock{background:var(--color-bg-secondary);border-color:var(--color-border)}.swagger-ui .opblock-summary{background:var(--color-bg-hover)}.swagger-ui .opblock-description{color:var(--color-muted)}.swagger-ui table thead tr td,.swagger-ui table thead tr th{color:var(--color-text);border-color:var(--color-border)}.swagger-ui .parameter__name{color:var(--color-primary)}.swagger-ui .response-col_status{color:var(--color-success)}.swagger-ui input,.swagger-ui select,.swagger-ui textarea{background:var(--color-bg-hover);color:var(--color-text);border-color:var(--color-border)}.swagger-ui .btn{background:var(--color-primary);color:var(--color-text-on-primary)}.swagger-ui .model-box,.swagger-ui section.models{background:var(--color-bg-secondary);border-color:var(--color-border)}.swagger-ui .markdown p,.swagger-ui .renderedMarkdown p{color:var(--color-text)}.swagger-ui .response-col_description__inner div.markdown{color:var(--color-text)}`
}

// renderSwaggerHTML returns the Swagger UI page that loads the vendored
// assets at assetsPrefix and fetches the OpenAPI spec from specURL.
//
// The page uses the project's site CSS for the header and overrides
// Swagger UI colors with the project theme variables so light/dark/auto
// matches the rest of the site (see AI.md PART 14 "Swagger & GraphQL
// Theming").
func renderSwaggerHTML(specURL, assetsPrefix string, baseURLs ...string) string {
	baseURL := ""
	if len(baseURLs) > 0 {
		baseURL = html.EscapeString(baseURLs[0])
	}
	specURL = html.EscapeString(specURL)
	assetsPrefix = html.EscapeString(assetsPrefix)
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" class="theme-dark">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>API Documentation - Airports API</title>
    <link rel="stylesheet" href="%[3]s/static/css/common.css">
    <link rel="stylesheet" href="%[3]s/static/css/components.css">
    <link rel="stylesheet" href="%[3]s/static/css/public.css">
    <link rel="stylesheet" href="%[3]s/static/css/docs.css">
    <link rel="stylesheet" href="%[2]sswagger-ui.css">
    <link rel="stylesheet" href="%[3]s/server/docs/swagger/theme.css">
  </head>
  <body>
    <header id="main-header">
      <div class="header-container">
        <div class="header-left">
          <button class="mobile-menu-toggle" data-action="toggle-mobile-menu">&#9776;</button>
          <a class="logo" href="%[3]s/">Airports API</a>
        </div>
        <nav id="main-nav" class="header-center">
          <a href="%[3]s/">Home</a>
          <a href="%[3]s/airports/search">Search</a>
          <a href="%[3]s/airports/nearby">Nearby</a>
          <a href="%[3]s/stats">Stats</a>
          <a href="%[3]s/server/docs/swagger" class="active">API Docs</a>
          <a href="%[3]s/server/docs/graphql">GraphQL</a>
        </nav>
        <div class="header-right">
          <button class="theme-toggle" data-action="toggle-theme">
            <span class="theme-icon">Theme</span>
          </button>
        </div>
      </div>
    </header>

    <div id="swagger-container">
      <div id="swagger-ui" data-spec-url=%[1]q></div>
    </div>

    <script src="%[2]sswagger-ui-bundle.js"></script>
    <script src="%[2]sswagger-ui-standalone-preset.js"></script>
    <script src="%[3]s/static/js/app.js"></script>
  </body>
</html>
`, specURL, assetsPrefix, baseURL)
}

package graphql

import "fmt"

// renderGraphiQLHTML returns the GraphiQL page that loads the embedded
// React + GraphiQL bundles at assetsPrefix and posts queries to
// endpointURL. The page wraps GraphiQL in the site chrome and applies
// the project theme variables (AI.md PART 14 "Swagger & GraphQL
// Theming").
func renderGraphiQLHTML(endpointURL, assetsPrefix string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="dark">
  <head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>GraphQL Playground - Airports API</title>
    <link rel="stylesheet" href="/static/css/main.css">
    <link rel="stylesheet" href="%[2]sgraphiql.min.css">
    <style>
      body { margin: 0; padding: 0; display: flex; flex-direction: column; min-height: 100vh; }
      #graphql-container { flex: 1; display: flex; flex-direction: column; min-height: 0; }
      #graphiql { flex: 1; height: 100%%; }
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
          <a href="/server/docs/swagger">API Docs</a>
          <a href="/server/docs/graphql" class="active">GraphQL</a>
        </nav>
        <div class="header-right">
          <button class="theme-toggle" onclick="toggleTheme()">
            <span class="theme-icon">Theme</span>
          </button>
        </div>
      </div>
    </header>

    <div id="graphql-container">
      <div id="graphiql"></div>
    </div>

    <script src="/static/js/main.js"></script>
    <script src="%[2]sreact.production.min.js"></script>
    <script src="%[2]sreact-dom.production.min.js"></script>
    <script src="%[2]sgraphiql.min.js"></script>
    <script>
      const fetcher = GraphiQL.createFetcher({ url: %[1]q });
      const defaultQuery = '# Welcome to the Airports GraphQL API\n# Press the Play button to run a query\n\nquery GetAirport {\n  airport(code: "KJFK") {\n    icao\n    iata\n    name\n    city\n    country\n    coordinates { lat lon }\n  }\n}\n\nquery SearchNearby {\n  nearby(lat: 40.6398, lon: -73.7789, radius: 50) {\n    icao\n    name\n    distance\n  }\n}\n';
      ReactDOM.createRoot(document.getElementById('graphiql'))
        .render(React.createElement(GraphiQL, { fetcher: fetcher, defaultQuery: defaultQuery }));
    </script>
  </body>
</html>
`, endpointURL, assetsPrefix)
}

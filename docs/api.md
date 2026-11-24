# Airports API Documentation

Complete API reference for the Airports API Server.

## Base URLs

- **Production**: `https://YOUR_SERVER`
- **Development**: `http://localhost:PORT`
- **API Base**: `/api/{api_version}` (the version segment is configurable via `server.api_version`; `v1` is the default)

Versioned routes are canonical. The operational aliases `/api/swagger`,
`/api/graphql`, `/api/healthz`, `/api/metrics[/{service}]`, `/api/mcp`,
`/mcp`, and `/api/debug/*` use the current API version's same handlers
directly; they do not redirect. `/api/autodiscover` is unversioned by design.
Data-shaped resource routes remain versioned. The optional root `/healthz`
alias is disabled unless `server.healthz.root.enabled` is true.

## Interactive Documentation

- **Swagger UI**: `/server/docs/swagger`
- **GraphQL Playground**: `/server/docs/graphql`

## Public web pages

- **Airport directory**: `GET /airports` renders a server-side paginated directory; use `?page=N` to move between pages.
- **Search**: `GET /airports/search?q=...`
- **Nearby airports**: `GET /airports/nearby?lat=...&lon=...`
- **Airport details**: `GET /airports/{ident}`
- **Statistics**: `GET /stats`
- **GeoIP lookup**: `GET /geoip`
- **Server pages**: `GET /server/about`, `/server/help`, `/server/help/mcp`, `/server/healthz`, `/server/privacy`, `/server/preferences`, `/server/preferences/sync`, `/server/preferences/tokens`, `/server/dpo`, `/server/terms`, `/server/contact`, `/server/security`, `/server/security/policy`, `/server/security/thanks`, `/server/security/report/{tracking_id}`, `/server/docs/swagger`, and `/server/docs/graphql`
- **Security report status**: `/server/security/report/{tracking_id}` is a one-time emailed link; it shows triage status only and has no API mirror.

These pages use the same airport data as the API and remain usable without JavaScript.

## Discovery and Syndication

The public frontend also serves `GET /sitemap.xml`, `GET /feed.xml` (Atom
1.0), and `GET /rss.xml` (RSS 2.0). Airports has no public user-created
content, so both feeds are valid and currently contain zero entries. URLs are
absolute for the current request host; feed responses support `ETag`,
`Last-Modified`, and conditional requests.

## Authentication

Public airport-data and informational reads require no authentication. Owner-token management, MCP, and metrics use the credential requirements described below. There are no login accounts or admin panel. See [security.md](./security.md) for the full security model.

## `.txt` / Plain-Text Support

Every `/api/**` endpoint that returns data also supports a plain-text representation, in addition to its default JSON response. Request it either by:

- appending `.txt` to the path (e.g. `/api/{api_version}/airports/search.txt`), or
- sending `Accept: text/plain`

Endpoints with an explicit `.txt` route: `/api/{api_version}/airports/{ident}.txt`, `/api/{api_version}/airports/search.txt`, `/api/{api_version}/airports/nearby.txt`, `/api/{api_version}/countries.txt`, `/api/{api_version}/stats.txt`, `/api/{api_version}/geoip.txt`, `/api/{api_version}/geoip/{ip}.txt`, `/api/{api_version}/server/healthz.txt`. The rest of the endpoints below are JSON-only.

---

## Response Envelope

All JSON API responses use the canonical envelope:

**Single item:**
```json
{
  "ok": true,
  "data": { }
}
```

**Paginated list:**
```json
{
  "ok": true,
  "data": [ ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 35479,
    "pages": 710
  }
}
```

**Error:**
```json
{
  "ok": false,
  "error": "CODE",
  "message": "Human readable message"
}
```

---

## Airport Endpoints

### Get All Airports

```http
GET /api/{api_version}/airports
```

**Query Parameters:**
- `limit` (int, optional) - Results per page (default: 250, max: 1000)
- `page` (int, optional) - Page number, 1-indexed (default: 1)

**Response:**
```json
{
  "ok": true,
  "data": [
    {
      "icao": "KJFK",
      "iata": "JFK",
      "name": "John F Kennedy International Airport",
      "city": "New York",
      "state": "New York",
      "country": "US",
      "elevation": 13,
      "lat": 40.63980103,
      "lon": -73.77890015,
      "tz": "America/New_York"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 250,
    "total": 35479,
    "pages": 142
  }
}
```

### Get Airport by Code

```http
GET /api/{api_version}/airports/{ident}
GET /api/{api_version}/airports/{ident}.txt
```

**Parameters:**
- `ident` - ICAO or IATA code (e.g., "KJFK" or "JFK")

**Response:**
```json
{
  "ok": true,
  "data": {
    "icao": "KJFK",
    "iata": "JFK",
    "name": "John F Kennedy International Airport",
    "city": "New York",
    "state": "New York",
    "country": "US",
    "elevation": 13,
    "lat": 40.63980103,
    "lon": -73.77890015,
    "tz": "America/New_York"
  }
}
```

### Search Airports

```http
GET /api/{api_version}/airports/search
```

**Query Parameters:**
- `q` (string) - Search query (name, city, code)
- `limit` (int, optional) - Max results (default: 50, max: 1000)
- `page` (int, optional) - Page number, 1-indexed (default: 1)

**Response:**
```json
{
  "ok": true,
  "data": [ ],
  "pagination": {
    "page": 1,
    "limit": 50,
    "total": 156,
    "pages": 4
  }
}
```

### Find Nearby Airports

```http
GET /api/{api_version}/airports/nearby
```

**Query Parameters:**
- `lat` (float, required) - Latitude
- `lon` (float, required) - Longitude
- `radius` (float, optional) - Search radius (default: 50, max: 500)
- `limit` (int, optional) - Max results (default: 20)
- `units` (string, optional) - `imperial` (default) or `metric`

The default radius is 50 miles. Use `units=metric` to interpret the radius in kilometers.

**Response:**
```json
{
  "ok": true,
  "data": {
    "data": [
      {
        "icao": "KJFK",
        "iata": "JFK",
        "name": "John F Kennedy International Airport",
        "distance": 15.2
      }
    ],
    "center": { "lat": 40.6398, "lon": -73.7789 },
    "radius": 50,
    "radius_unit": "mi",
    "units": "imperial",
    "count": 12
  }
}
```

### Bounding Box Search

```http
GET /api/{api_version}/airports/within
```

**Query Parameters:**
- `minLat` (float, required) - Minimum latitude
- `maxLat` (float, required) - Maximum latitude
- `minLon` (float, required) - Minimum longitude
- `maxLon` (float, required) - Maximum longitude

**Response:**
```json
{
  "ok": true,
  "data": {
    "data": [ ],
    "count": 45
  }
}
```

### Autocomplete

```http
GET /api/{api_version}/airports/autocomplete
```

**Query Parameters:**
- `q` (string, required) - Search query (min 2 chars)
- `limit` (int, optional) - Max suggestions (default: 10, max: 50)

**Response:**
```json
{
  "ok": true,
  "data": {
    "suggestions": [ ],
    "query": "JFK"
  }
}
```

---

## Countries and States

### List Countries

```http
GET /api/{api_version}/countries
GET /api/{api_version}/countries.txt
```

**Response:**
```json
{
  "ok": true,
  "data": {
    "US": 5234,
    "CA": 890
  }
}
```

### List States in a Country

```http
GET /api/{api_version}/states/{country}
```

**Parameters:**
- `country` - Country code (e.g., "US")

**Response:**
```json
{
  "ok": true,
  "data": {
    "New York": 234,
    "California": 745
  }
}
```

---

## Statistics

```http
GET /api/{api_version}/stats
GET /api/{api_version}/stats.txt
```

**Response:**
```json
{
  "ok": true,
  "data": {
    "total_airports": 35479,
    "countries": 249,
    "with_iata": 8745,
    "with_icao": 35479
  }
}
```

---

## Export Endpoints

These export the full database and are JSON/CSV/GeoJSON only — there is no `.csv`/`.geojson` variant of `search` or any other query endpoint.

### Export as JSON

```http
GET /api/{api_version}/airports.json
```

### Export as CSV

```http
GET /api/{api_version}/airports.csv
```

### Export as GeoJSON

```http
GET /api/{api_version}/airports.geojson
```

---

## GeoIP Endpoints

### Lookup Current IP

```http
GET /api/{api_version}/geoip
GET /api/{api_version}/geoip.txt
```

**Response:**
```json
{
  "ok": true,
  "data": {
    "country": "US",
    "city": "Mountain View",
    "latitude": 37.386,
    "longitude": -122.0838
  }
}
```

### Lookup Specific IP

```http
GET /api/{api_version}/geoip/{ip}
GET /api/{api_version}/geoip/{ip}.txt
```

**Parameters:**
- `ip` - IPv4 or IPv6 address

### Find Nearby Airports (IP-based)

```http
GET /api/{api_version}/geoip/airports/nearby
```

---

## Health Endpoints

The JSON response reports Tor as an object with `enabled`, `running`, `status`, and `hostname`. I2P also includes `state` (the provider lifecycle) and `provider`. For example, an opted-out I2P network reports `enabled: false`, `status: "disabled"`, `state: "disabled"`, and `provider: "none"`; an enabled configuration with no installed provider reports `enabled: false`, `status: "disabled"`, `state: "no_provider"`, and `provider: "none"`. The `checks.i2p` key is omitted in both cases.

### Server Health (root/versioned alias)

```http
GET /healthz
GET /server/healthz
GET /api/healthz
GET /api/{api_version}/server/healthz
GET /api/{api_version}/server/auth/check
GET /server/healthz.txt
GET /api/healthz.txt
GET /api/{api_version}/server/healthz.txt
```

All paths serve the same handler. The `/healthz` root alias is optional and only exists when configured. Content negotiation follows route type: frontend routes return HTML to browsers, API routes return JSON by default, and `.txt`, `Accept: text/plain`, or a non-interactive HTTP client receives plain text.

---

## Settings

```http
GET /api/{api_version}/settings
```

Read-only, no auth required. Returns a safe subset of server configuration (theme, CORS origins, metrics enabled) — never credentials or internal topology.

---

## Server Info Endpoints

Mirrors of the public `/server/*` frontend pages, JSON/versioned under the API tree:

```http
GET /api/{api_version}/server/healthz
GET /api/{api_version}/server/about
GET /api/{api_version}/server/help
GET /api/{api_version}/server/privacy
GET /api/{api_version}/server/preferences
POST /api/{api_version}/server/preferences
GET /api/{api_version}/server/preferences/sync
GET /api/{api_version}/server/preferences/sync/import
GET /api/{api_version}/server/dpo
GET /api/{api_version}/server/terms
GET /api/{api_version}/server/contact
POST /api/{api_version}/server/contact
GET /api/{api_version}/server/preferences/tokens
POST /api/{api_version}/server/preferences/tokens/primary
POST /api/{api_version}/server/preferences/tokens/primary/reveal
POST /api/{api_version}/server/preferences/tokens/primary/rotate
POST /api/{api_version}/server/preferences/tokens
POST /api/{api_version}/server/preferences/tokens/{id}/reveal
POST /api/{api_version}/server/preferences/tokens/{id}/rotate
PATCH /api/{api_version}/server/preferences/tokens/{id}
DELETE /api/{api_version}/server/preferences/tokens/{id}
GET /api/{api_version}/server/help/mcp
GET /api/{api_version}/server/security
GET /api/{api_version}/server/security/policy
GET /api/{api_version}/server/security/thanks
GET /api/{api_version}/server/swagger
POST /api/{api_version}/server/graphql
```

`/api/{api_version}/server/auth/check` requires an operator or owner Bearer token
and reports only its kind (`operator` or `owner`). It never returns the token.

The About, Help, Terms, and MCP help endpoints return JSON objects with their
documented `title`/`content`/`format` fields (Help also returns `pages`; MCP
help returns `enabled`, `url`, `aliases`, `auth`, `tools`, and `project`). The
security status page has no API mirror. Language and theme preferences are
available on `/server/preferences` and mirrored by the four preference
routes above. The sync response includes the effective theme, language and
app-specific preference map, a shareable import URL, and an unpadded base64url
code. Imports accept direct query values or a pasted URL/query/code and return
`applied` and `ignored` parameter arrays. Consent and credential cookies are
never exported or imported. Owner tokens have a separate page at
`/server/preferences/tokens` and the versioned API routes listed above. Token
management accepts a primary owner Bearer token only; token issuance needs no
token and returns the newly issued primary token once. Linked tokens cannot
manage tokens. Browser create, reveal, and rotate actions use POST/303 plus a
one-use card-selection cookie; the following page load reveals only that
token, disables caching, and clears the cookie. Reloads return to masked values.
The API equivalents return the raw token in the JSON response body without a
redirect.

The one-shot security report status page is available at
`GET /server/security/report/{tracking_id}` and has no API mirror. Contact
`GET` returns the field descriptors, a newly created CAPTCHA, and a
form token. The form offers abuse, DMCA, and privacy reports; security reports
are accepted only through a valid security ID from the security page or
`security.txt`. `POST` submissions use `step=submit` to validate and return a
review without delivery, then repeat every field with `step=confirm`, the
review token, and `confirmed=true` to deliver. Form tokens enforce minimum
completion time and single use; review tokens enforce a confirmation delay and
bind the review to the submitted fields. CAPTCHA challenges are one-use and
expire after ten minutes. Accepted reports are limited by client IP, sender
email, and a global window; content checks reject markup, blocked terms/domains,
and disallowed links. The response does not disclose which content rule failed.

The DPO endpoint returns `name`, `email`, and an absolute `privacy_url`. Its
email uses `server.contact.dpo.email`, then `server.contact.general.email`;
it never falls back to the private admin address. Security report status is
available only at the emailed `/server/security/report/{tracking_id}` link
and has no API mirror. The one-shot token is removed before access and
application request logging. The page shows the current triage state,
submission date, maintainer comment, and expected disclosure date; it never
shows the vulnerability report body.

---

## GraphQL API

The canonical, versioned GraphQL endpoint is:

```http
POST /api/{api_version}/server/graphql
```

An unversioned alias is also mounted at the same handler:

```http
POST /api/graphql
```

**Interactive Playground**: `/server/docs/graphql`

There is no GET-based GraphQL endpoint and no `/api/{api_version}/graphql` or `/api/{api_version}/docs` path — those were removed; use the paths above.

GraphQL `nearby` uses kilometers and defaults to a 50 km radius. The REST
`/airports/nearby` endpoint defaults to a 50 mile radius and accepts
`units=metric` for kilometers.

### Example Query

```graphql
query {
  airport(code: "KJFK") {
    icao
    iata
    name
    city
    country
    coordinates {
      lat
      lon
    }
  }

  nearby(lat: 40.6398, lon: -73.7789, radius: 50) {
    icao
    name
    distance
  }
}
```

### Schema

Available via introspection at the GraphQL endpoint.

---

## Autodiscover

```http
GET /api/autodiscover
```

Unversioned. Returns the standard `{"ok":true,"data":...}` envelope. `data` contains only `name`, `version`, `api.current_version`, `cli_versions` (platform entries with `version` and `sha256`), `cli_min_version`, and `urls` (public origins, with the current request origin first). It may contain `onion` or `i2p` only when the corresponding overlay is enabled and its `server.tor.expose` or `server.i2p.expose` setting is true. It does not publish server settings or an endpoint map.

## Metrics

When metrics are enabled in configuration, Prometheus metrics are available at
`/metrics` and the Prometheus service route `/metrics/prometheus`. The
Prometheus, Grafana, and Loki services are also available under
`/server/metrics/{service}`, `/api/metrics/{service}`, and
`/api/{api_version}/server/metrics/{service}`. Each service has a separate
bearer token configured under `server.metrics.auth.tokens`. A matching token
is mandatory; firewalling the endpoint is additional defense in depth. Root
metrics aliases can be disabled with `server.metrics.root.enabled: false`.

## Debug API

When debug mode is enabled, the diagnostics under `/debug/*` are also mounted
at `/api/{api_version}/debug/*` and `/api/debug/*`. Every debug route requires
the operator token from `server.token`; versioned routes include the standard
API version and no-cache headers. The route set includes the custom diagnostics,
pprof routes when `server.debug.pprof` is enabled, and `/vars` when
`server.debug.runtime_endpoints` is enabled.

## Model Context Protocol

Airports exposes its API operations as tools over Streamable HTTP. Every
request requires an `Authorization: Bearer` linked owner token created for
that client; never use the primary owner token or operator token
(`server.token`). Cookies are ignored. Token management and token issuance are
never MCP tools. The canonical endpoint is
`/api/{api_version}/server/mcp`; `/api/mcp` and `/mcp` are direct aliases.
Requests use `POST`, protocol version `2026-07-28`, and an allowed `Origin`
when an Origin header is sent. The `MCP-Protocol-Version` header must match
the request `_meta` protocol version, `Mcp-Method` must match the JSON-RPC
method, and name-bearing calls must provide the matching `Mcp-Name`. Origin,
authentication, protocol-version, header, and method checks run in that order.
Invalid credentials return `401` with `WWW-Authenticate: Bearer`; unsupported
versions return `400`; mismatched headers return `400` with `-32020`
(`HeaderMismatch`); unknown methods return `404` with `-32601`.

The live tool names and schemas, endpoint, aliases, connection examples,
protocol versions, and troubleshooting guidance are available at
`/server/help/mcp`. Its JSON mirror is
`/api/{api_version}/server/help/mcp`. MCP does not create a session, and tool
calls use the same API handlers, authorization, audit, and rate limits.

---

## Error Responses

All errors follow this format:

```json
{
  "ok": false,
  "error": "NOT_FOUND",
  "message": "Airport not found: XXXX",
  "details": {}
}
```

**Common Error Codes:**
- `BAD_REQUEST` - Invalid parameters
- `NOT_FOUND` - Resource not found
- `SERVER_ERROR` - Server error

**HTTP Status Codes:**
- `200` - Success
- `400` - Bad Request
- `404` - Not Found
- `429` - Too Many Requests
- `500` - Internal Server Error
- `503` - Service Unavailable

---

## Rate Limiting

Default limits (configurable in `server.yml`):

| Category | Default |
|----------|---------|
| Read | 120 requests/minute |
| Write | 10 requests/minute |
| Health checks | 120 requests/minute |
| Global burst | 240 requests/minute |

On `429 Too Many Requests`, the retry delay is communicated via the `Retry-After` header only — never in the JSON body.

---

## SDK Examples

### JavaScript/Node.js

```javascript
// Using fetch
const response = await fetch('https://YOUR_SERVER/api/{api_version}/airports/KJFK');
const data = await response.json();
console.log(data.data);

// Search nearby
const nearby = await fetch(
  'https://YOUR_SERVER/api/{api_version}/airports/nearby?lat=40.6398&lon=-73.7789&radius=50'
);
const airports = await nearby.json();
```

### Python

```python
import requests

# Get airport
response = requests.get('https://YOUR_SERVER/api/{api_version}/airports/KJFK')
airport = response.json()['data']

# Search
params = {'q': 'New York', 'limit': 10}
response = requests.get('https://YOUR_SERVER/api/{api_version}/airports/search', params=params)
results = response.json()['data']
```

### cURL

```bash
# Get airport
curl -q -LSsf https://YOUR_SERVER/api/{api_version}/airports/KJFK

# Search nearby
curl -q -LSsf "https://YOUR_SERVER/api/{api_version}/airports/nearby?lat=40.6398&lon=-73.7789&radius=50"

# Public airport read endpoints need no token. Protected server and metrics endpoints
# document their authentication requirements in the sections above.
curl -q -LSsf "https://YOUR_SERVER/api/{api_version}/airports/search?q=Tokyo"

# Plain-text response instead of JSON
curl -q -LSsf https://YOUR_SERVER/api/{api_version}/airports/KJFK.txt
```

---

## Support

- **Documentation home**: [index.md](./index.md)
- **Security & operations**: [security.md](./security.md), [admin.md](./admin.md)
- **Issues**: https://github.com/apimgr/airports/issues

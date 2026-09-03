# Airports API Documentation

Complete API reference for the Airports API Server.

## Base URLs

- **Production**: `https://your-domain.com`
- **Development**: `http://localhost:PORT`
- **API Base**: `/api/v1` (the version segment is configurable via `server.api_version`; `v1` is the default)

## Interactive Documentation

- **Swagger UI**: `/server/docs/swagger`
- **GraphQL Playground**: `/server/docs/graphql`

## Authentication

All endpoints are **public** and require no authentication. There are no user accounts, no admin panel, and no API keys. Abuse protection is rate-limit-only. See [security.md](./security.md) for the full security model.

## `.txt` / Plain-Text Support

Every `/api/**` endpoint that returns data also supports a plain-text representation, in addition to its default JSON response. Request it either by:

- appending `.txt` to the path (e.g. `/api/v1/airports/search.txt`), or
- sending `Accept: text/plain`

Endpoints with an explicit `.txt` route: `/api/v1/airports/{ident}.txt`, `/api/v1/airports/search.txt`, `/api/v1/airports/nearby.txt`, `/api/v1/search.txt`, `/api/v1/nearby.txt`, `/api/v1/countries.txt`, `/api/v1/stats.txt`, `/api/v1/geoip.txt`, `/api/v1/geoip/{ip}.txt`, `/api/v1/health.txt`. The rest of the endpoints below are JSON-only.

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
GET /api/v1/airports
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
GET /api/v1/airports/{ident}
GET /api/v1/airports/{ident}.txt
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
GET /api/v1/airports/search
GET /api/v1/search
```

Flat `/api/v1/search` is a retained alias of `/api/v1/airports/search` for existing clients — both use the same handler.

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
GET /api/v1/airports/nearby
GET /api/v1/nearby
```

**Query Parameters:**
- `lat` (float, required) - Latitude
- `lon` (float, required) - Longitude
- `radius` (float, optional) - Search radius (default: 50, max: 500)
- `limit` (int, optional) - Max results (default: 20)
- `units` (string, optional) - `imperial` (default) or `metric`

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
GET /api/v1/airports/within
GET /api/v1/bbox
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
GET /api/v1/airports/autocomplete
GET /api/v1/autocomplete
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
GET /api/v1/countries
GET /api/v1/countries.txt
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
GET /api/v1/states/{country}
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
GET /api/v1/stats
GET /api/v1/stats.txt
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
GET /api/v1/airports.json
```

### Export as CSV

```http
GET /api/v1/airports.csv
```

### Export as GeoJSON

```http
GET /api/v1/airports.geojson
```

---

## GeoIP Endpoints

### Lookup Current IP

```http
GET /api/v1/geoip
GET /api/v1/geoip.txt
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
GET /api/v1/geoip/{ip}
GET /api/v1/geoip/{ip}.txt
```

**Parameters:**
- `ip` - IPv4 or IPv6 address

### Find Nearby Airports (IP-based)

```http
GET /api/v1/geoip/airports/nearby
```

---

## Health Endpoints

### Server Health (root/versioned alias)

```http
GET /healthz
GET /server/healthz
GET /api/healthz
GET /api/v1/server/healthz
```

All four paths serve the same handler. Content negotiation applies: browsers get HTML, `curl`/CLI clients get plain text, `Accept: application/json` gets JSON.

### Airport-Data Health

```http
GET /api/v1/health
GET /api/v1/health.txt
```

**Response:**
```json
{
  "ok": true,
  "data": {
    "status": "healthy",
    "version": "1.0.0",
    "checks": {
      "airports": { "status": "loaded", "total": 35479 },
      "geoip": { "status": "loaded" }
    }
  }
}
```

---

## Settings

```http
GET /api/v1/settings
```

Read-only, no auth required. Returns a safe subset of server configuration (theme, CORS origins, metrics enabled) — never credentials or internal topology.

---

## Server Info Endpoints

Mirrors of the public `/server/*` frontend pages, JSON/versioned under the API tree:

```http
GET /api/v1/server/healthz
GET /api/v1/server/about
GET /api/v1/server/help
GET /api/v1/server/privacy
GET /api/v1/server/terms
GET /api/v1/server/contact
GET /api/v1/server/swagger
GET /api/v1/server/swagger.json
POST /api/v1/server/graphql
```

---

## GraphQL API

The canonical, versioned GraphQL endpoint is:

```http
POST /api/v1/server/graphql
```

An unversioned alias is also mounted at the same handler:

```http
POST /api/graphql
```

**Interactive Playground**: `/server/docs/graphql`

There is no GET-based GraphQL endpoint and no `/api/v1/graphql` or `/api/v1/docs` path — those were removed; use the paths above.

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

Unversioned. Returns server identity/version, the API version list, the primary URL, and the endpoint map — used by `airports-cli` and other agent consumers to self-configure.

---

## Error Responses

All errors follow this format:

```json
{
  "ok": false,
  "error": "NOT_FOUND",
  "message": "Airport not found: XXXX"
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
const response = await fetch('https://api.example.com/api/v1/airports/KJFK');
const data = await response.json();
console.log(data.data);

// Search nearby
const nearby = await fetch(
  'https://api.example.com/api/v1/airports/nearby?lat=40.6398&lon=-73.7789&radius=50'
);
const airports = await nearby.json();
```

### Python

```python
import requests

# Get airport
response = requests.get('https://api.example.com/api/v1/airports/KJFK')
airport = response.json()['data']

# Search
params = {'q': 'New York', 'limit': 10}
response = requests.get('https://api.example.com/api/v1/airports/search', params=params)
results = response.json()['data']
```

### cURL

```bash
# Get airport
curl https://api.example.com/api/v1/airports/KJFK

# Search nearby
curl "https://api.example.com/api/v1/airports/nearby?lat=40.6398&lon=-73.7789&radius=50"

# All endpoints are public — no token, no auth header needed
curl "https://api.example.com/api/v1/airports/search?q=Tokyo"

# Plain-text response instead of JSON
curl https://api.example.com/api/v1/airports/KJFK.txt
```

---

## Support

- **Documentation home**: [index.md](./index.md)
- **Security & operations**: [security.md](./security.md), [admin.md](./admin.md)
- **Issues**: https://github.com/apimgr/airports/issues

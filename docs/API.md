# Airports API Documentation

Complete API reference for the Airports API Server.

## Base URLs

- **Production**: `https://your-domain.com`
- **Development**: `http://localhost:PORT`
- **API Base**: `/api/v1`

## Interactive Documentation

- **Swagger UI**: `/api/v1/docs` or `/docs`
- **GraphQL Playground**: `/api/v1/graphql` or `/graphql`

## Authentication

### Public Endpoints
All airport data endpoints are **public** and require no authentication.

### Admin Endpoints
Admin endpoints require authentication. See [SERVER.md](./SERVER.md#authentication) for details.

**Authentication Methods:**
- **Bearer Token**: `Authorization: Bearer <token>`
- **Basic Auth**: Username and password for web UI

---

## Airport Endpoints

### Get All Airports

```http
GET /api/v1/airports
```

**Query Parameters:**
- `limit` (int, optional) - Results per page (default: 50, max: 1000)
- `offset` (int, optional) - Pagination offset (default: 0)

**Response:**
```json
{
  "success": true,
  "data": {
    "airports": [
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
    "total": 35000,
    "limit": 50,
    "offset": 0
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Get Airport by Code

```http
GET /api/v1/airports/{code}
```

**Parameters:**
- `code` - ICAO or IATA code (e.g., "KJFK" or "JFK")

**Response:**
```json
{
  "success": true,
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
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Search Airports

```http
GET /api/v1/airports/search
```

**Query Parameters:**
- `q` (string) - Search query (name, city, code)
- `city` (string, optional) - Filter by city
- `country` (string, optional) - Filter by country code (e.g., "US")
- `state` (string, optional) - Filter by state
- `limit` (int, optional) - Max results (default: 50, max: 1000)
- `offset` (int, optional) - Pagination offset

**Response:**
```json
{
  "success": true,
  "data": {
    "airports": [...],
    "total": 156,
    "limit": 50,
    "offset": 0,
    "query": "New York"
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Find Nearby Airports

```http
GET /api/v1/airports/nearby
```

**Query Parameters:**
- `lat` (float, required) - Latitude
- `lon` (float, required) - Longitude
- `radius` (int, optional) - Radius in km (default: 50, max: 500)
- `limit` (int, optional) - Max results (default: 20)

**Response:**
```json
{
  "success": true,
  "data": {
    "airports": [
      {
        "icao": "KJFK",
        "iata": "JFK",
        "name": "John F Kennedy International Airport",
        "distance_km": 15.2,
        ...
      }
    ],
    "center": {
      "lat": 40.6398,
      "lon": -73.7789
    },
    "radius_km": 50,
    "count": 12
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Bounding Box Search

```http
GET /api/v1/airports/bbox
```

**Query Parameters:**
- `minLat` (float, required) - Minimum latitude
- `maxLat` (float, required) - Maximum latitude
- `minLon` (float, required) - Minimum longitude
- `maxLon` (float, required) - Maximum longitude

**Response:**
```json
{
  "success": true,
  "data": {
    "airports": [...],
    "bbox": {
      "minLat": 40.0,
      "maxLat": 41.0,
      "minLon": -74.0,
      "maxLon": -73.0
    },
    "count": 45
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Autocomplete

```http
GET /api/v1/airports/autocomplete
```

**Query Parameters:**
- `q` (string, required) - Search query (min 2 chars)
- `limit` (int, optional) - Max suggestions (default: 10)

**Response:**
```json
{
  "success": true,
  "data": {
    "suggestions": [
      {
        "icao": "KJFK",
        "iata": "JFK",
        "name": "John F Kennedy International Airport",
        "city": "New York",
        "country": "US"
      }
    ],
    "query": "JFK",
    "count": 1
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### List Countries

```http
GET /api/v1/airports/countries
```

**Response:**
```json
{
  "success": true,
  "data": {
    "countries": [
      {
        "code": "US",
        "name": "United States",
        "airport_count": 5234
      }
    ],
    "total": 249
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### List States

```http
GET /api/v1/airports/states/{country}
```

**Parameters:**
- `country` - Country code (e.g., "US")

**Response:**
```json
{
  "success": true,
  "data": {
    "country": "US",
    "states": [
      {
        "code": "NY",
        "name": "New York",
        "airport_count": 234
      }
    ],
    "total": 52
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Database Statistics

```http
GET /api/v1/airports/stats
```

**Response:**
```json
{
  "success": true,
  "data": {
    "total_airports": 35479,
    "countries": 249,
    "with_iata": 8745,
    "with_icao": 35479
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

---

## Export Endpoints

### Export as JSON

```http
GET /api/v1/airports.json
```

Returns the complete airport database as JSON.

### Export as CSV

```http
GET /api/v1/airports.csv
GET /api/v1/airports/search.csv?q=New+York
```

Returns airports as CSV format.

### Export as GeoJSON

```http
GET /api/v1/airports.geojson
GET /api/v1/airports/search.geojson?q=New+York
```

Returns airports as GeoJSON FeatureCollection.

---

## GeoIP Endpoints

### Lookup Current IP

```http
GET /api/v1/geoip
```

**Response:**
```json
{
  "success": true,
  "data": {
    "ip": "8.8.8.8",
    "country": "US",
    "country_name": "United States",
    "region": "CA",
    "region_name": "California",
    "city": "Mountain View",
    "latitude": 37.386,
    "longitude": -122.0838,
    "timezone": "America/Los_Angeles"
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

### Lookup Specific IP

```http
GET /api/v1/geoip/{ip}
```

**Parameters:**
- `ip` - IPv4 or IPv6 address

### Find Nearby Airports (IP-based)

```http
GET /api/v1/geoip/airports/nearby
```

**Query Parameters:**
- `ip` (string, optional) - IP to geolocate (defaults to request IP)
- `radius` (int, optional) - Search radius in km (default: 100)
- `limit` (int, optional) - Max results (default: 10)

**Response:**
```json
{
  "success": true,
  "data": {
    "location": {
      "ip": "8.8.8.8",
      "city": "Mountain View",
      "country": "US",
      "latitude": 37.386,
      "longitude": -122.0838
    },
    "nearby_airports": [
      {
        "icao": "KSJC",
        "iata": "SJC",
        "name": "San Jose International Airport",
        "distance_km": 15.2
      }
    ],
    "count": 5
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

---

## Health Check

### Server Health

```http
GET /healthz
```

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T12:00:00Z",
  "version": "1.0.0",
  "uptime_seconds": 86400,
  "checks": {
    "database": {
      "status": "connected",
      "type": "sqlite"
    },
    "airports": {
      "loaded": 35479
    },
    "geoip": {
      "status": "active"
    }
  }
}
```

---

## Error Responses

All errors follow this format:

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable message",
    "field": "field_name"
  },
  "timestamp": "2024-01-01T12:00:00Z"
}
```

**Common Error Codes:**
- `INVALID_REQUEST` - Invalid parameters
- `NOT_FOUND` - Resource not found
- `RATE_LIMIT_EXCEEDED` - Too many requests
- `INTERNAL_ERROR` - Server error

**HTTP Status Codes:**
- `200` - Success
- `400` - Bad Request
- `404` - Not Found
- `429` - Too Many Requests
- `500` - Internal Server Error
- `503` - Service Unavailable

---

## Rate Limiting

- **Public endpoints**: 100 requests/minute per IP
- **Admin endpoints**: 1000 requests/minute per token

Rate limit headers:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 2024-01-01T12:15:00Z
```

---

## GraphQL API

GraphQL endpoint available at `/api/v1/graphql` or `/graphql`.

**Interactive Playground**: `/graphql`

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
results = response.json()['data']['airports']
```

### cURL

```bash
# Get airport
curl https://api.example.com/api/v1/airports/KJFK

# Search nearby
curl "https://api.example.com/api/v1/airports/nearby?lat=40.6398&lon=-73.7789&radius=50"

# Admin endpoint (with token)
curl -H "Authorization: Bearer YOUR_TOKEN" \
  https://api.example.com/api/v1/admin/settings
```

---

## Support

- **Documentation**: [docs/README.md](./README.md)
- **Server Admin**: [docs/SERVER.md](./SERVER.md)
- **Issues**: GitHub Issues

package graphql

// Schema is the human-readable GraphQL schema string for the Airports
// API. Per AI.md PART 14 it MUST stay in sync with the resolvers in
// resolvers.go and with the OpenAPI spec in ../swagger/annotations.go.
const Schema = `# Airports API GraphQL schema
type Airport {
  icao: String
  iata: String
  name: String
  city: String
  country: String
  coordinates: Coordinates
}

type Coordinates {
  lat: Float
  lon: Float
}

type AirportWithDistance {
  icao: String
  iata: String
  name: String
  city: String
  country: String
  distance: Float
}

type NameCount {
  name: String
  count: Int
}

type Stats {
  totalAirports: Int
  countries: Int
  cities: Int
  withIata: Int
}

type GeoLocation {
  ip: String
  country: String
  countryName: String
  region: String
  regionName: String
  city: String
  latitude: Float
  longitude: Float
  timezone: String
  postalCode: String
  asn: Int
  asnOrg: String
}

type Query {
  airport(code: String!): Airport
  nearby(lat: Float!, lon: Float!, radius: Float = 50): [AirportWithDistance!]!
  airports(limit: Int = 250, page: Int = 1): [Airport!]!
  search(q: String!, limit: Int = 50, page: Int = 1): [Airport!]!
  within(latMin: Float!, latMax: Float!, lonMin: Float!, lonMax: Float!): [Airport!]!
  autocomplete(q: String!, limit: Int = 10): [Airport!]!
  countries: [NameCount!]!
  states(country: String!): [NameCount!]!
  stats: Stats
  geoip(ip: String): GeoLocation
}
`

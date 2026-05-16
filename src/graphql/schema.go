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

type Query {
  airport(code: String!): Airport
  nearby(lat: Float!, lon: Float!, radius: Float = 50): [AirportWithDistance!]!
}
`

#!/bin/bash
# End-to-end tests for airports server

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Test counters
PASS=0
FAIL=0

# Start server
echo "Starting airports server..."
./binaries/airports --port 9999 >/tmp/airports_e2e.log 2>&1 &
SERVER_PID=$!
sleep 3

# Function to stop server on exit
cleanup() {
    echo "Stopping server..."
    kill $SERVER_PID 2>/dev/null || true
    wait $SERVER_PID 2>/dev/null || true
}
trap cleanup EXIT

# Base URL
BASE_URL="http://localhost:9999"

# Test function
test_api() {
    local name="$1"
    local method="$2"
    local endpoint="$3"
    local expected_status="$4"

    echo -n "Testing: $name... "

    response=$(curl -s -w "\n%{http_code}" -X "$method" "$BASE_URL$endpoint")
    status_code=$(echo "$response" | tail -n1)
    body=$(echo "$response" | head -n-1)

    if [ "$status_code" = "$expected_status" ]; then
        echo -e "${GREEN}✅ PASS${NC} (HTTP $status_code)"
        ((PASS++))
    else
        echo -e "${RED}❌ FAIL${NC} (Expected HTTP $expected_status, got $status_code)"
        echo "Response: $body" | head -c 200
        ((FAIL++))
    fi
}

echo ""
echo "=== E2E Tests ==="
echo ""

# Core endpoints
test_api "Health check" "GET" "/healthz" "200"
test_api "Homepage" "GET" "/" "200"
test_api "API health" "GET" "/api/v1/health" "200"

# Airport lookups
test_api "Get JFK by ICAO" "GET" "/api/v1/airports/KJFK" "200"
test_api "Get JFK by IATA" "GET" "/api/v1/airports/JFK" "200"
test_api "Get LAX" "GET" "/api/v1/airports/KLAX" "200"
test_api "Not found airport" "GET" "/api/v1/airports/XXXXXX" "404"

# Search functionality
test_api "Search New York" "GET" "/api/v1/airports/search?q=New+York" "200"
test_api "Search empty query" "GET" "/api/v1/airports/search?q=" "200"
test_api "Autocomplete JFK" "GET" "/api/v1/airports/autocomplete?q=JFK" "200"
test_api "Autocomplete too short" "GET" "/api/v1/airports/autocomplete?q=J" "400"

# Geographic queries
test_api "Nearby JFK" "GET" "/api/v1/airports/nearby?lat=40.6398&lon=-73.7789&radius=50" "200"
test_api "Nearby invalid lat" "GET" "/api/v1/airports/nearby?lat=invalid&lon=-73.7789" "400"
test_api "Bounding box NYC" "GET" "/api/v1/airports/bbox?minLat=40&maxLat=41&minLon=-74&maxLon=-73" "200"

# Country/State queries
test_api "List countries" "GET" "/api/v1/airports/countries" "200"
test_api "States in US" "GET" "/api/v1/airports/states/US" "200"
test_api "States in Canada" "GET" "/api/v1/airports/states/CA" "200"

# Statistics
test_api "Airport stats" "GET" "/api/v1/airports/stats" "200"

# Full export
test_api "Full database JSON" "GET" "/api/v1/airports.json" "200"

# GeoIP
test_api "GeoIP 8.8.8.8" "GET" "/api/v1/geoip/8.8.8.8" "200"
test_api "GeoIP 1.1.1.1" "GET" "/api/v1/geoip/1.1.1.1" "200"
test_api "GeoIP invalid" "GET" "/api/v1/geoip/invalid" "400"
test_api "GeoIP nearby airports" "GET" "/api/v1/geoip/airports/nearby?ip=8.8.8.8" "200"

# Pagination
test_api "List airports limit 10" "GET" "/api/v1/airports?limit=10" "200"
test_api "List airports offset" "GET" "/api/v1/airports?limit=10&offset=100" "200"

echo ""
echo "=== Summary ==="
echo -e "${GREEN}Passed: $PASS${NC}"
echo -e "${RED}Failed: $FAIL${NC}"
echo "Total: $((PASS + FAIL))"
echo ""

if [ $FAIL -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed${NC}"
    exit 1
fi

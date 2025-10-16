#!/bin/bash
# Docker testing script for airports API
# Uses docker-compose.test.yml with /tmp volumes

set -e

PROJECT="airports"
ROOTFS="/tmp/${PROJECT}/rootfs"
COMPOSE_FILE="../docker-compose.test.yml"

cd "$(dirname "$0")"

echo "🧹 Cleaning up old test environment..."
docker-compose -f "$COMPOSE_FILE" down 2>/dev/null || true
sudo rm -rf "$ROOTFS"

echo "📁 Creating test directories..."
sudo mkdir -p "$ROOTFS"/{config,data,logs,db}
sudo chown -R $(id -u):$(id -g) "$ROOTFS"

echo "🔨 Building Docker image..."
docker-compose -f "$COMPOSE_FILE" build

echo "🚀 Starting containers..."
docker-compose -f "$COMPOSE_FILE" up -d

echo "⏳ Waiting for server to start..."
sleep 8

echo ""
echo "✅ Server started! Testing endpoints..."
echo ""

# Test public endpoint
echo "📍 Testing public endpoint:"
curl -s http://localhost:8080/api/v1 | python3 -m json.tool | head -10
echo ""

# Test health endpoint
echo "🏥 Testing health endpoint:"
curl -s http://localhost:8080/healthz | python3 -m json.tool
echo ""

# Get admin credentials
if [ -f "$ROOTFS/config/admin_credentials" ]; then
    echo "🔑 Admin credentials:"
    cat "$ROOTFS/config/admin_credentials"
    echo ""

    # Extract token
    TOKEN=$(grep "Token:" "$ROOTFS/config/admin_credentials" | awk '{print $NF}')

    # Test admin API
    echo "🔐 Testing admin API:"
    curl -s -H "Authorization: Bearer $TOKEN" \
        http://localhost:8080/api/v1/admin/settings | python3 -m json.tool | head -20
    echo ""
fi

echo "📊 Testing airport search:"
curl -s "http://localhost:8080/api/v1/airports/search?q=JFK&limit=2" | python3 -m json.tool
echo ""

echo "✨ Test complete!"
echo ""
echo "Access: http://localhost:8080"
echo "Admin:  http://localhost:8080/admin"
echo ""
echo "Cleanup:"
echo "  docker-compose -f $COMPOSE_FILE down"
echo "  sudo rm -rf $ROOTFS"

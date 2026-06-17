#!/bin/bash

echo "🚀 StarkGateway Setup Script"
echo "=============================="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "❌ Go is not installed. Install Go 1.22+ from https://go.dev/dl"
    exit 1
fi

echo "✅ Go detected: $(go version)"

# Copy environment file
if [ ! -f .env ]; then
    cp .env.example .env
    echo "✅ Created .env file (edit to customize)"
fi

# Download dependencies
echo "📦 Downloading Go dependencies..."
go mod download

# Build
echo "🔨 Building binary..."
go build -o starkgateway .

echo ""
echo "✅ Build complete!"
echo ""
echo "🚀 Start your gateway with:"
echo "   ./starkgateway"
echo ""
echo "📡 Your RPC endpoint will be at:"
echo "   http://localhost:8545"
echo ""
echo "📊 Monitor metrics at:"
echo "   curl http://localhost:8545/metrics"

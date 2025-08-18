#!/bin/bash
set -e

echo "🔨 Building k8s-diagnostic for Docker deployment..."
echo "================================================="

# Create build directory if it doesn't exist
mkdir -p build

# Build the Linux binary
echo "📦 Building Go binary for Linux..."
make build-linux

# Check if binary was created
if [ ! -f "build/k8s-diagnostic-linux-amd64" ]; then
    echo "❌ Failed to build Linux binary"
    exit 1
fi

# Build Docker containers
echo "🐳 Building Docker containers..."
docker compose build

# Verify containers were built
if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Build complete!"
    echo "================================================="
    echo "🚀 To start the services, run:"
    echo "   docker compose up -d k8s-diagnostic-ui"
    echo ""
    echo "🖥️  UI will be available at: http://localhost:3000"
    echo "🛠️  To run CLI tests:"
    echo "   docker compose run --rm k8s-diagnostic-cli test --help"
    echo ""
    echo "📋 Available make targets:"
    echo "   make docker-up    # Build and start services"
    echo "   make docker-down  # Stop services"
    echo "   make docker-clean # Clean up containers"
else
    echo "❌ Docker build failed"
    exit 1
fi

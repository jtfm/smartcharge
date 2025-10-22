#!/bin/bash

# Build script for battery-dashboard lambda function

set -e

echo "Building battery dashboard lambda function..."

# Set environment variables for Linux AMD64 build (temporary fix)
export GOOS=linux
export GOARCH=amd64
export CGO_ENABLED=0

# Build the binary
go build -ldflags="-s -w" -o bootstrap main.go

# Create deployment package
if [ -f battery-dashboard.zip ]; then
    rm battery-dashboard.zip
fi

zip battery-dashboard.zip bootstrap

echo "Build complete! Created battery-dashboard.zip"
echo "File size: $(du -h battery-dashboard.zip | cut -f1)"
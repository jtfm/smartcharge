#!/bin/bash

# Build script for API lambda function

set -e

echo "Building API lambda function..."

# Set environment variables for Linux ARM64 build
export GOOS=linux
export GOARCH=arm64
export CGO_ENABLED=0

# Build the binary
go build -ldflags="-s -w" -o bootstrap main.go

echo "Build complete! Created bootstrap binary."
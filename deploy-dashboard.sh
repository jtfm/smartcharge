#!/bin/bash

# Deployment script for battery dashboard

set -e

echo "🔋 Deploying Battery Dashboard..."

# Build the API lambda function
echo "📦 Building API lambda function..."
cd api
./build.sh
cd ..

# Build the battery dashboard lambda function
echo "📦 Building battery dashboard lambda function..."
cd battery-dashboard
./build.sh
cd ..

# Deploy infrastructure
echo "🚀 Deploying infrastructure..."
cd infra
pulumi up --yes

echo "✅ Deployment complete!"
echo "🌐 Dashboard URLs will be displayed in the Pulumi output."
#!/bin/bash

# Build script for all components
echo "Building all components..."

# Build API Lambda function
echo "Building API Lambda function..."
cd api
./build.sh

if [ $? -eq 0 ]; then
    echo "✅ API Lambda build successful"
else
    echo "❌ API Lambda build failed"
    exit 1
fi

cd ..

# Build Battery Dashboard Lambda function
echo "Building Battery Dashboard Lambda function..."
cd battery-dashboard
./build.sh

if [ $? -eq 0 ]; then
    echo "✅ Battery Dashboard Lambda build successful"
else
    echo "❌ Battery Dashboard Lambda build failed"
    exit 1
fi

cd ..

echo "Building React frontend..."
cd frontend

# Install dependencies if node_modules doesn't exist
if [ ! -d "node_modules" ]; then
    echo "Installing npm dependencies..."
    npm install
fi

# Build the React app
npm run build

if [ $? -eq 0 ]; then
    echo "✅ Frontend build successful"
else
    echo "❌ Frontend build failed"
    exit 1
fi

cd ..

echo "🚀 All builds completed successfully!"
echo ""
echo "Next steps:"
echo "1. Run 'pulumi up' to deploy infrastructure"
echo "2. Note the API endpoint URL from the output"
echo "3. Set REACT_APP_API_ENDPOINT environment variable in Amplify"
echo "4. Push to Git to trigger Amplify deployment"
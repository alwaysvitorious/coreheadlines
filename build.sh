#!/bin/bash

# Call from root directory: ./build.sh [--local|--lambda]

ROOT_DIR=$(pwd)
BUILD_DIR="./bin"
FUNCTION_NAME="coreheadlines"

MODE="lambda" # default
if [ "$1" = "--local" ]; then
    MODE="local"
elif [ "$1" = "--lambda" ]; then
    MODE="lambda"
fi

if [ "$MODE" = "local" ]; then
    echo "🏃 Running locally..."
    go run .
    exit $?
fi

echo "Cleaning build directory..."
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

echo "Building for AWS Lambda..."
GOOS=linux GOARCH=arm64 go build -o "$BUILD_DIR/bootstrap"

if [ $? -ne 0 ]; then
    echo "❌ Build failed!"
    exit 1
fi

cd "$BUILD_DIR"
zip coreheadlines.zip bootstrap
cd "$ROOT_DIR"

if [ $? -ne 0 ]; then
    echo "❌ Zip creation failed!"
    exit 1
fi

echo "✅ Lambda build complete. Output: $BUILD_DIR/coreheadlines.zip"

echo "🚀 Uploading to Lambda function: $FUNCTION_NAME"
aws lambda update-function-code \
    --function-name "$FUNCTION_NAME" \
    --zip-file "fileb://$BUILD_DIR/coreheadlines.zip"

if [ $? -eq 0 ]; then
    echo "✅ Lambda function updated successfully!"
else
    echo "❌ Failed to update Lambda function!"
    exit 1
fi

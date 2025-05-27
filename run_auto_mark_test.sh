#!/bin/bash

# Build the test tool
echo "Building auto-mark test tool..."
cd "$(dirname "$0")"
mkdir -p cmd/test_auto_mark/bin
go build -o cmd/test_auto_mark/bin/test_auto_mark cmd/test_auto_mark/main.go

# Check if build was successful
if [ $? -ne 0 ]; then
    echo "Build failed!"
    exit 1
fi

echo "Build successful!"

# Run the test tool
echo "Running auto-mark test tool..."
./cmd/test_auto_mark/bin/test_auto_mark

echo "Test completed!" 
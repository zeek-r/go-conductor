#!/bin/bash

# Navigate to the root of the project
cd "$(dirname "$0")/.." || exit

# Build the mock server and conductor
go build -o bin/mockserver ./cmd/mockserver
go build -o bin/conductor ./cmd/go-conductor

# Create bin directory if it doesn't exist
mkdir -p bin

# Start mock servers
echo "Starting mock servers..."
./bin/mockserver -port 8081 -name "api-primary" &
API_PRIMARY_PID=$!
./bin/mockserver -port 8082 -name "api-secondary" &
API_SECONDARY_PID=$!
./bin/mockserver -port 8083 -name "web" &
WEB_PID=$!
./bin/mockserver -port 8084 -name "default" &
DEFAULT_PID=$!

# Wait for servers to start
sleep 1

# Start the conductor proxy
echo "Starting conductor proxy..."
./bin/conductor --config examples/config.yaml &
PROXY_PID=$!

# Wait for proxy to start
sleep 1

# Run some test requests
echo -e "\nTesting API route (should use primary)..."
curl -v http://localhost:8080/api/users

echo -e "\nTesting Web route..."
curl -v http://localhost:8080/web/index.html

echo -e "\nTesting default route..."
curl -v http://localhost:8080/

# Cleanup
echo -e "\nCleaning up..."
kill $PROXY_PID $API_PRIMARY_PID $API_SECONDARY_PID $WEB_PID $DEFAULT_PID

echo "Done!" 
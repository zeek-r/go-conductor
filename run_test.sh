#!/bin/bash

# Build the mock server and conductor
go build -o mockserver ./test/mockserver.go
go build -o conductor .

# Start mock servers
echo "Starting mock servers..."
./mockserver -port 8081 -name "api-primary" &
API_PRIMARY_PID=$!
./mockserver -port 8082 -name "api-secondary" &
API_SECONDARY_PID=$!
./mockserver -port 8083 -name "web" &
WEB_PID=$!
./mockserver -port 8084 -name "default" &
DEFAULT_PID=$!

# Wait for servers to start
sleep 1

# Start the conductor proxy
echo "Starting conductor proxy..."
./conductor &
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
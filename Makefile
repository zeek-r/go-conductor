.PHONY: build run test clean install

# Build binaries
build:
	@mkdir -p bin
	go build -o bin/conductor ./cmd/go-conductor
	go build -o bin/mockserver ./cmd/mockserver

# Run the go-conductor
run: build
	./bin/conductor --config examples/config.yaml

# Run the test script
test: build
	./scripts/run_test.sh

# Clean up binaries
clean:
	rm -rf bin/

# Install to GOPATH/bin
install:
	go install ./cmd/go-conductor 
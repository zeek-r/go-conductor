# go-conductor

[![GitHub build status](https://github.com/zeek-r/go-conductor/workflows/build/badge.svg)](https://github.com/zeek-r/go-conductor/actions)
[![GitHub test status](https://github.com/zeek-r/go-conductor/workflows/tests/badge.svg)](https://github.com/zeek-r/go-conductor/actions)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](https://opensource.org/licenses/MIT)

A lightweight HTTP request multiplexer that sends a single request to multiple backend services and returns the response from the primary service.

## How It Works

```
                                       ┌─────────────────────┐
                                       │                     │
                                  ┌───▶│ Service A (PRIMARY) │───┐
                                  │    │                     │   │
                                  │    └─────────────────────┘   │
                                  │                              │
                                  │    ┌─────────────────────┐   │
┌───────────┐    ┌────────────┐   │    │                     │   │    ┌────────────┐
│           │    │            │───┼───▶│ Service B           │   │    │            │
│  Client   │───▶│ go-conductor│───┘    │                     │   └───▶│  Response   │
│           │    │            │        └─────────────────────┘        │  from A     │
└───────────┘    └────────────┘                                       └────────────┘
                                       ┌─────────────────────┐
                                       │                     │
                                       │ Service C           │
                                       │                     │
                                       └─────────────────────┘
                                       (Not matching route)
```

1. When a request arrives, go-conductor finds all matching services based on the URL path
2. The request is sent simultaneously to all matching services
3. The conductor waits for responses from all services (or until timeout)
4. The response from the primary service is returned to the client
5. If the primary service fails, a successful response from any other service is used as fallback
6. If all services fail, a 502 Bad Gateway error is returned

## Features

- Fan-out a single HTTP request to multiple backend services
- Return the response from the designated primary service
- Fall back to any successful response if the primary service fails
- Add custom headers to proxied requests
- Simple YAML configuration
- Configurable timeout handling

## Installation

### Using go install

```bash
go install github.com/zeek-r/go-conductor/cmd/go-conductor@latest
```

### Building from Source

```bash
git clone https://github.com/zeek-r/go-conductor.git
cd go-conductor
make build
```

## Project Structure

```
├── cmd/                    # Command executables
│   ├── go-conductor/       # Main application
│   └── mockserver/         # Test mock server
├── internal/               # Internal packages
│   ├── conf/               # Configuration package
│   └── proxy/              # Proxy implementation
├── examples/               # Example configuration
├── scripts/                # Scripts for testing/deployment
└── bin/                    # Build output (gitignored)
```

## Usage

1. Create a configuration file (config.yaml):

```yaml
port: 8080
timeout: 10  # request timeout in seconds

# Logging configuration
logging:
  level: info          # debug, info, warn, error, fatal
  format: json         # json, pretty
  output: stdout       # stdout, stderr, file
  includeCaller: true  # include caller information
  timeFormat: "2006-01-02T15:04:05Z07:00"  # RFC3339 format
  disableTimestamp: false

# Metrics configuration
metrics:
  enabled: true              # enable metrics collection
  endpoint: "/metrics"       # endpoint to expose metrics
  enablePrometheus: true     # use Prometheus format metrics

services:
  - name: api-service-primary
    url: http://localhost:8081
    pathPrefix: /api
    primary: true
    headers:
      X-Proxy-Service: api-service-primary

  - name: api-service-secondary
    url: http://localhost:8082
    pathPrefix: /api
    headers:
      X-Proxy-Service: api-service-secondary

  - name: web-service
    url: http://localhost:8083
    pathPrefix: /web
    primary: true
    headers:
      X-Proxy-Service: web-service
      
  - name: default-service
    url: http://localhost:8084
    path: /
    primary: true
    headers:
      X-Proxy-Service: default-service
```

2. Run the proxy:

```bash
go-conductor --config config.yaml
```

## Testing with Mock Servers

The repository includes a mock server implementation for testing purposes. To test the proxy with mock servers:

1. Build the mock server:

```bash
go build -o bin/mockserver ./cmd/mockserver
```

2. Start the mock servers on different ports:

```bash
./bin/mockserver -port 8081 -name "api-primary" &
./bin/mockserver -port 8082 -name "api-secondary" &
./bin/mockserver -port 8083 -name "web" &
./bin/mockserver -port 8084 -name "default" &
```

3. Run the proxy with the updated port (if necessary):

```bash
go run cmd/go-conductor/main.go
```

4. Send test requests:

```bash
curl http://localhost:8080/api/users
curl http://localhost:8080/web/index.html
curl http://localhost:8080/
```

5. Observe the failover behavior by stopping one of the primary services.

You can also use the test script to run a complete test flow:

```bash
./scripts/run_test.sh
```

## Flow Diagram

```
┌─────────┐     ┌─────────────┐     ┌────────────────┐     ┌──────────────┐
│ Request │────▶│ Match Route │────▶│ Fan-out Request│────▶│ Wait for     │
└─────────┘     └─────────────┘     └────────────────┘     │ Responses    │
                                                          └───────┬──────┘
                                                                  │
                                                                  ▼
┌─────────┐     ┌─────────────┐     ┌────────────────┐     ┌──────────────┐
│ Response│◀────│ Return to   │◀────│ Select Primary │◀────│ Collect      │
│         │     │ Client      │     │ Response       │     │ Results      │
└─────────┘     └─────────────┘     └────────────────┘     └──────────────┘
```

## Configuration Options

### Top-level Configuration

- `port`: The port on which the proxy will listen (default: 8080)
- `timeout`: Request timeout in seconds (default: 30)
- `services`: A list of backend services to proxy to
- `logging`: Logging configuration options
- `metrics`: Metrics collection configuration options

### Service Configuration

- `name`: A descriptive name for the service
- `url`: The URL of the backend service
- `primary`: Set to true for the service whose response should be returned (at least one per path pattern)
- `path`: The base path for this service (used as fallback)
- `pathPrefix`: Route requests with this path prefix to the service
- `pathExact`: Route requests with exactly this path to the service
- `headers`: Map of custom headers to add to requests

### Logging Configuration

- `level`: Minimum log level to output (debug, info, warn, error, fatal)
- `format`: Log format (json, pretty)
- `output`: Where logs are written (stdout, stderr, file)
- `file`: Path to log file when output is set to "file"
- `includeCaller`: Whether to include caller information (file/line) in logs
- `timeFormat`: Time format string for log timestamps (default: RFC3339)
- `disableTimestamp`: If true, timestamps will be omitted from logs

### Metrics Configuration

- `enabled`: Enable metrics collection (true/false)
- `endpoint`: Path to expose metrics (default: "/metrics")
- `enablePrometheus`: Use Prometheus format for metrics instead of JSON (true/false)

## Development

### Running Tests

```bash
make test
```

### Building Binaries

```bash
make build
```

### Installing Locally

```bash
make install
```

## Practical Use Cases

- **Testing in Production**: Send requests to both production and staging environments, but only return production responses
- **Shadow Traffic**: Test new service implementations by sending them real traffic without affecting users
- **Resiliency**: Add redundancy by sending requests to multiple instances and using the first successful response
- **Data Collection**: Send the same data to multiple systems while preserving the primary response flow
- **A/B Testing**: Send traffic to different implementations for analysis without affecting the user experience

## Example Scenarios

### Scenario 1: API Migration

```
┌───────────┐    ┌────────────┐    ┌──────────────────────┐
│           │    │            │───▶│ API v1 (PRIMARY)     │
│  Client   │───▶│ go-conductor│    └──────────────────────┘
│           │    │            │    ┌──────────────────────┐
└───────────┘    └────────────┘───▶│ API v2 (new version) │
                                   └──────────────────────┘
```

Test a new API version with real production traffic while ensuring clients still get responses from the stable version.

### Scenario 2: Redundant Services

```
┌───────────┐    ┌────────────┐    ┌──────────────────────┐
│           │    │            │───▶│ Service A (PRIMARY)  │
│  Client   │───▶│ go-conductor│    └──────────────────────┘
│           │    │            │    ┌──────────────────────┐
└───────────┘    └────────────┘───▶│ Service B (backup)   │
                                   └──────────────────────┘
```

Send requests to both primary and backup services, automatically falling back to the backup if the primary fails.

## License

MIT 
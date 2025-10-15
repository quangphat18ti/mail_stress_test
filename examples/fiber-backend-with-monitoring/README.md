# Fiber Backend Example with Prometheus Monitoring

This example shows how to setup a Fiber backend with Prometheus monitoring for stress testing.

## Setup

```bash
cd examples/fiber-backend-with-monitoring

# Install dependencies
go mod init fiber-example
go get github.com/gofiber/fiber/v2
go get github.com/gofiber/adaptor/v2
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promauto
go get github.com/prometheus/client_golang/prometheus/promhttp
go get go.mongodb.org/mongo-driver/mongo

# Run the server
go run main.go
```

## Test Endpoints

```bash
# Health check
curl http://localhost:3000/health

# Prometheus metrics
curl http://localhost:3000/metrics

# Create mail
curl -X POST http://localhost:3000/api/mails \
  -H "Content-Type: application/json" \
  -d '{
    "userId": "user123",
    "from": "sender@example.com",
    "to": ["recipient@example.com"],
    "subject": "Test Email",
    "content": "This is a test email"
  }'

# List mails
curl "http://localhost:3000/api/mails?userId=user123&page=1&limit=10"

# Search mails
curl "http://localhost:3000/api/mails/search?userId=user123&query=test"
```

## Run Stress Test Against This Server

```bash
cd ../..

# Update config to use API
# Set use_api: true and api_endpoint: "http://localhost:3000"

# Enable monitoring
# Set monitoring.enabled: true
# Set monitoring.prometheus_url: "http://localhost:3000/metrics"

# Run stress test
./mail-stress-test -stress -use-api -config config/default.yaml
```

## Metrics Exposed

- `http_requests_total` - Total HTTP requests by method, path, status
- `http_request_duration_seconds` - Request latency histogram
- `http_errors_total` - Total HTTP errors
- `fiber_connections_active` - Active connections count
- `db_queries_total` - Database queries count
- `db_query_duration_seconds` - Database query latency
- Plus standard Go metrics (memory, goroutines, GC, etc.)

## Monitoring Dashboard

Access Prometheus metrics at `http://localhost:3000/metrics`

Example metrics output:
```
# HELP http_requests_total Total number of HTTP requests
# TYPE http_requests_total counter
http_requests_total{method="POST",path="/api/mails",status="200"} 1523

# HELP http_request_duration_seconds HTTP request duration in seconds
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="POST",path="/api/mails",le="0.005"} 1245
http_request_duration_seconds_bucket{method="POST",path="/api/mails",le="0.01"} 1432
...

# HELP fiber_connections_active Number of active connections
# TYPE fiber_connections_active gauge
fiber_connections_active 42
```

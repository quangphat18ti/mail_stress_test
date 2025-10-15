# Monitoring Setup Guide

## Overview

This guide explains how to setup monitoring for your Fiber backend during stress testing. The monitoring system collects:

- **Prometheus Metrics**: HTTP requests, response times, errors, goroutines, memory usage
- **System Metrics**: CPU, RAM, network, TCP connections
- **Performance Insights**: Automatic detection of bottlenecks

## Architecture

```
┌─────────────────┐         ┌──────────────────┐         ┌─────────────────┐
│  Stress Test    │ Monitor │  Fiber Backend   │ Expose  │   Prometheus    │
│  (This Project) │────────▶│  (Your App)      │────────▶│   Endpoint      │
└─────────────────┘         └──────────────────┘         └─────────────────┘
                                     │
                                     ▼
                              System Resources
                              (CPU, RAM, Connections)
```

## Setup Options

### Option 1: Prometheus Metrics (Recommended ⭐)

Monitor your Fiber app via Prometheus `/metrics` endpoint.

#### Step 1: Add Prometheus to Your Fiber App

Install dependencies:
```bash
go get github.com/gofiber/fiber/v2
go get github.com/gofiber/adaptor/v2
go get github.com/prometheus/client_golang/prometheus
go get github.com/prometheus/client_golang/prometheus/promhttp
```

Add to your Fiber app:

```go
package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/adaptor/v2"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"time"
)

var (
	// HTTP Metrics
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	
	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	
	httpErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "Total number of HTTP errors",
		},
		[]string{"method", "path", "status"},
	)
	
	activeConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fiber_connections_active",
			Help: "Number of active connections",
		},
	)
	
	// Database Metrics (optional)
	dbQueriesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		},
	)
	
	dbQueryDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
	)
)

// Prometheus middleware
func PrometheusMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		
		// Track active connections
		activeConnections.Inc()
		defer activeConnections.Dec()
		
		// Continue with request
		err := c.Next()
		
		// Record metrics
		duration := time.Since(start).Seconds()
		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()
		
		httpRequestsTotal.WithLabelValues(method, path, string(status)).Inc()
		httpRequestDuration.WithLabelValues(method, path).Observe(duration)
		
		if status >= 400 {
			httpErrorsTotal.WithLabelValues(method, path, string(status)).Inc()
		}
		
		return err
	}
}

func main() {
	app := fiber.New()
	
	// Add Prometheus middleware
	app.Use(PrometheusMiddleware())
	
	// Expose Prometheus metrics endpoint
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
	
	// Your existing routes
	app.Post("/api/mails", createMailHandler)
	app.Get("/api/mails", listMailsHandler)
	app.Get("/api/mails/search", searchMailsHandler)
	
	app.Listen(":3000")
}

// Example handler with DB metrics
func createMailHandler(c *fiber.Ctx) error {
	// Track DB operation
	start := time.Now()
	
	// Your DB logic here
	// ...
	
	dbQueriesTotal.Inc()
	dbQueryDuration.Observe(time.Since(start).Seconds())
	
	return c.JSON(fiber.Map{"status": "ok"})
}
```

#### Step 2: Configure Stress Test

Update `config/default.yaml`:

```yaml
monitoring:
  enabled: true
  prometheus_url: "http://localhost:3000/metrics"  # Your Fiber app metrics endpoint
  scrape_interval: 5s
  enable_system_monitor: false
  enable_realtime_log: true
```

#### Step 3: Run Stress Test with Monitoring

```bash
./mail-stress-test -stress -config config/default.yaml
```

### Option 2: System-Level Monitoring

Monitor system resources (CPU, RAM, connections) without Prometheus.

#### Configuration

```yaml
monitoring:
  enabled: true
  prometheus_url: ""  # Leave empty to disable Prometheus
  scrape_interval: 5s
  enable_system_monitor: true
  target_host: ""  # Empty for local, or "user@host" for remote SSH
  is_docker: false
  container_id: ""
  enable_realtime_log: true
```

#### For Docker Container

```yaml
monitoring:
  enabled: true
  enable_system_monitor: true
  is_docker: true
  container_id: "fiber-app"  # Your container name or ID
  scrape_interval: 5s
```

### Option 3: Both Prometheus + System Monitoring

Get comprehensive metrics from both sources:

```yaml
monitoring:
  enabled: true
  prometheus_url: "http://localhost:3000/metrics"
  scrape_interval: 5s
  enable_system_monitor: true
  target_host: ""
  is_docker: false
  enable_realtime_log: true
```

## Monitoring Output

### Real-time Console Output

During stress test, you'll see:

```
🔍 Starting monitoring...
✅ Prometheus monitoring started
✅ System monitoring started

📊 Prometheus: CPU=45.2%, Mem=512.3MB, Requests=1523
💻 System: CPU=42.8%, Mem=68.5%, Connections=245

📊 Prometheus: CPU=52.1%, Mem=548.7MB, Requests=3247
💻 System: CPU=49.3%, Mem=71.2%, Connections=312
...

🛑 Stopping monitoring...
```

### Monitoring Report

After test completion:

```
====================================================================================================
📊 MONITORING SUMMARY
====================================================================================================

⏱️  Test Duration: 5m0s
📅 Start: 2025-10-15 14:30:00
📅 End:   2025-10-15 14:35:00

🔍 Prometheus Metrics:
   --------------------------------------------------------------------------------
   HTTP Requests:      15000 total (50.00 req/s)
   Error Rate:         0.23%
   Avg CPU:            47.50%
   Avg Memory:         530.00 MB
   Peak Goroutines:    1250
   Avg Connections:    278

   Response Times (End of Test):
   P50: 12.34ms | P95: 45.67ms | P99: 89.01ms

💻 System Metrics:
   --------------------------------------------------------------------------------
   CPU Usage:          Avg: 45.30% | Peak: 78.20%
   Memory Usage:       Avg: 2048.50MB (69.50%) | Peak: 2304.00MB
   TCP Connections:    Avg: 265 | Peak: 425
   Load Average (1m):  2.45

💡 Performance Insights:
   --------------------------------------------------------------------------------
   ⚠️  High CPU usage: 78.20%
   📡 Peak connections: 425 - ensure connection pooling

====================================================================================================
```

### JSON Report

Saved to `./reports/monitoring_20251015_143000.json`:

```json
{
  "test_info": {
    "start_time": "2025-10-15T14:30:00Z",
    "end_time": "2025-10-15T14:35:00Z",
    "duration": "5m0s"
  },
  "prometheus_available": true,
  "prometheus_diff": {
    "http_requests_increase": 15000,
    "http_requests_per_second": 50.0,
    "http_error_rate_percent": 0.23,
    "avg_cpu_usage_percent": 47.5,
    "avg_memory_usage_mb": 530.0,
    "peak_goroutines": 1250,
    "avg_active_connections": 278
  },
  "system_available": true,
  "system_summary": {
    "avg_cpu_usage_percent": 45.3,
    "peak_cpu_usage_percent": 78.2,
    "avg_memory_usage_mb": 2048.5,
    "peak_memory_usage_mb": 2304.0,
    "avg_memory_usage_percent": 69.5,
    "avg_tcp_connections": 265,
    "peak_tcp_connections": 425,
    "avg_load_average_1min": 2.45
  },
  "insights": [
    "⚠️  High CPU usage: 78.20%",
    "📡 Peak connections: 425 - ensure connection pooling"
  ]
}
```

## Performance Insights

The monitoring system automatically detects:

| Issue | Threshold | Recommendation |
|-------|-----------|----------------|
| High Error Rate | > 5% | Check application logs, review error handling |
| High CPU Usage | > 80% | Scale horizontally, optimize hot paths |
| High Memory | > 85% | Check for memory leaks, adjust GC settings |
| Many Connections | > 1000 | Use connection pooling, check keep-alive |
| High Latency P99 | > 1s | Optimize slow queries, add caching |

## Remote Monitoring via SSH

Monitor Fiber app running on remote server:

```yaml
monitoring:
  enabled: true
  enable_system_monitor: true
  target_host: "user@remote-server.com"
  scrape_interval: 10s
```

**Requirements:**
- SSH key-based authentication configured
- Remote server has `top`, `free`, `netstat` commands

## Troubleshooting

### Prometheus endpoint not accessible

```
⚠️  Warning: Failed to scrape initial Prometheus metrics: Get "http://localhost:3000/metrics": connection refused
```

**Solution:**
1. Ensure Fiber app is running: `curl http://localhost:3000/metrics`
2. Check firewall rules
3. Verify `prometheus_url` in config

### System monitoring fails

```
⚠️  Failed to collect system metrics: failed to execute command
```

**Solution:**
- macOS: Requires `top` and `vm_stat` commands
- Linux: Requires `top`, `free`, `netstat` commands
- Run with `sudo` if permission denied

### Docker monitoring not working

```
⚠️  Failed to collect system metrics: docker stats failed
```

**Solution:**
1. Verify container is running: `docker ps`
2. Check container ID/name matches config
3. Ensure Docker socket accessible

## Example: Full Monitoring Setup

### 1. Fiber App with Monitoring

```go
// main.go
package main

import (
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/adaptor/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	app := fiber.New()
	
	// Prometheus middleware
	app.Use(PrometheusMiddleware())
	
	// Metrics endpoint
	app.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))
	
	// API routes
	app.Post("/api/mails", createMail)
	app.Get("/api/mails", listMails)
	app.Get("/api/mails/search", searchMails)
	
	app.Listen(":3000")
}
```

### 2. Config with Full Monitoring

```yaml
# config/production.yaml
mongodb:
  uri: "mongodb://localhost:27017"
  database: "mail_stress_test"
  timeout: 30

stress_test:
  num_users: 1000
  num_mails_per_user: 10000
  concurrent_workers: 200
  request_rate: 500
  duration: 10m
  use_api: true
  api_endpoint: "http://localhost:3000"

monitoring:
  enabled: true
  prometheus_url: "http://localhost:3000/metrics"
  scrape_interval: 5s
  enable_system_monitor: true
  enable_realtime_log: true

report:
  output_dir: "./reports"
  generate_chart: true
  json_report: true
```

### 3. Run Test

```bash
# Start Fiber app
cd /path/to/fiber-app
go run main.go

# In another terminal, run stress test
cd /path/to/stress-test
./mail-stress-test -stress -use-api -config config/production.yaml
```

## Best Practices

1. **Baseline First**: Run test without load to establish baseline metrics
2. **Gradual Load**: Increase concurrent workers gradually to find breaking point
3. **Long Duration**: Run for at least 5-10 minutes for accurate metrics
4. **Multiple Runs**: Run tests multiple times to ensure consistency
5. **Monitor Trends**: Track metrics over time to catch regressions
6. **Set Alerts**: Use Prometheus alerts for production monitoring

## Next Steps

- **Grafana Dashboard**: Import pre-built dashboard for visualization
- **Custom Metrics**: Add application-specific metrics
- **Distributed Tracing**: Add Jaeger/Zipkin for request tracing
- **Log Aggregation**: Integrate with ELK/Loki for log analysis

## Support

For issues or questions:
- Check documentation in `/monitoring` package
- Review example configurations in `/examples`
- Open issue on GitHub repository

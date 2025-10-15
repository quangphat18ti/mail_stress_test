package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// PrometheusClient scrapes metrics from Prometheus-compatible endpoints
type PrometheusClient struct {
	metricsURL string
	httpClient *http.Client
}

// PrometheusMetrics stores snapshot of key metrics
type PrometheusMetrics struct {
	Timestamp time.Time `json:"timestamp"`

	// HTTP Metrics
	HTTPRequestsTotal      float64 `json:"http_requests_total"`
	HTTPRequestDurationP50 float64 `json:"http_request_duration_p50_ms"`
	HTTPRequestDurationP95 float64 `json:"http_request_duration_p95_ms"`
	HTTPRequestDurationP99 float64 `json:"http_request_duration_p99_ms"`
	HTTPErrorsTotal        float64 `json:"http_errors_total"`
	HTTPActiveConnections  float64 `json:"http_active_connections"`

	// System Metrics
	CPUUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryUsageMB      float64 `json:"memory_usage_mb"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	GoroutinesCount    float64 `json:"goroutines_count"`

	// Database Metrics (if exposed)
	DBConnectionsActive float64 `json:"db_connections_active"`
	DBConnectionsIdle   float64 `json:"db_connections_idle"`
	DBQueriesTotal      float64 `json:"db_queries_total"`
	DBQueryDurationP99  float64 `json:"db_query_duration_p99_ms"`

	// Custom App Metrics
	CustomMetrics map[string]float64 `json:"custom_metrics,omitempty"`
}

// MetricsDiff represents the change between two snapshots
type MetricsDiff struct {
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
	Duration  string    `json:"duration"`

	HTTPRequestsIncrease  float64 `json:"http_requests_increase"`
	HTTPRequestsPerSecond float64 `json:"http_requests_per_second"`
	HTTPErrorRatePercent  float64 `json:"http_error_rate_percent"`
	AvgCPUUsagePercent    float64 `json:"avg_cpu_usage_percent"`
	AvgMemoryUsageMB      float64 `json:"avg_memory_usage_mb"`
	PeakGoroutines        float64 `json:"peak_goroutines"`
	AvgActiveConnections  float64 `json:"avg_active_connections"`

	StartMetrics *PrometheusMetrics `json:"start_metrics"`
	EndMetrics   *PrometheusMetrics `json:"end_metrics"`
}

func NewPrometheusClient(metricsURL string) *PrometheusClient {
	return &PrometheusClient{
		metricsURL: metricsURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// ScrapeMetrics fetches current metrics snapshot
func (pc *PrometheusClient) ScrapeMetrics(ctx context.Context) (*PrometheusMetrics, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", pc.metricsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := pc.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape metrics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metrics endpoint returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	metrics := &PrometheusMetrics{
		Timestamp:     time.Now(),
		CustomMetrics: make(map[string]float64),
	}

	// Parse Prometheus text format
	if err := pc.parsePrometheusFormat(string(body), metrics); err != nil {
		return nil, fmt.Errorf("failed to parse metrics: %w", err)
	}

	return metrics, nil
}

// parsePrometheusFormat parses Prometheus exposition format
func (pc *PrometheusClient) parsePrometheusFormat(body string, metrics *PrometheusMetrics) error {
	lines := strings.Split(body, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse metric line: metric_name{labels} value
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		metricName := parts[0]
		var value float64
		fmt.Sscanf(parts[len(parts)-1], "%f", &value)

		// Map to our metrics structure
		switch {
		// HTTP Metrics
		case strings.Contains(metricName, "http_requests_total"):
			metrics.HTTPRequestsTotal = value
		case strings.Contains(metricName, "http_request_duration_seconds") && strings.Contains(metricName, "quantile=\"0.5\""):
			metrics.HTTPRequestDurationP50 = value * 1000 // Convert to ms
		case strings.Contains(metricName, "http_request_duration_seconds") && strings.Contains(metricName, "quantile=\"0.95\""):
			metrics.HTTPRequestDurationP95 = value * 1000
		case strings.Contains(metricName, "http_request_duration_seconds") && strings.Contains(metricName, "quantile=\"0.99\""):
			metrics.HTTPRequestDurationP99 = value * 1000
		case strings.Contains(metricName, "http_errors_total"):
			metrics.HTTPErrorsTotal = value
		case strings.Contains(metricName, "http_connections_active") || strings.Contains(metricName, "fiber_connections_active"):
			metrics.HTTPActiveConnections = value

		// System Metrics
		case strings.Contains(metricName, "process_cpu_seconds_total"):
			metrics.CPUUsagePercent = value // This needs calculation based on time diff
		case strings.Contains(metricName, "process_resident_memory_bytes"):
			metrics.MemoryUsageMB = value / 1024 / 1024
		case strings.Contains(metricName, "go_goroutines"):
			metrics.GoroutinesCount = value

		// Database Metrics
		case strings.Contains(metricName, "db_connections_active"):
			metrics.DBConnectionsActive = value
		case strings.Contains(metricName, "db_connections_idle"):
			metrics.DBConnectionsIdle = value
		case strings.Contains(metricName, "db_queries_total"):
			metrics.DBQueriesTotal = value
		case strings.Contains(metricName, "db_query_duration") && strings.Contains(metricName, "quantile=\"0.99\""):
			metrics.DBQueryDurationP99 = value * 1000

		// Store other metrics in custom map
		default:
			if !strings.HasPrefix(metricName, "#") {
				metrics.CustomMetrics[metricName] = value
			}
		}
	}

	return nil
}

// CalculateDiff computes the difference between two metric snapshots
func (pc *PrometheusClient) CalculateDiff(start, end *PrometheusMetrics) *MetricsDiff {
	duration := end.Timestamp.Sub(start.Timestamp)

	diff := &MetricsDiff{
		StartTime:    start.Timestamp,
		EndTime:      end.Timestamp,
		Duration:     duration.String(),
		StartMetrics: start,
		EndMetrics:   end,
	}

	// HTTP Metrics
	diff.HTTPRequestsIncrease = end.HTTPRequestsTotal - start.HTTPRequestsTotal
	if duration.Seconds() > 0 {
		diff.HTTPRequestsPerSecond = diff.HTTPRequestsIncrease / duration.Seconds()
	}

	errorIncrease := end.HTTPErrorsTotal - start.HTTPErrorsTotal
	if diff.HTTPRequestsIncrease > 0 {
		diff.HTTPErrorRatePercent = (errorIncrease / diff.HTTPRequestsIncrease) * 100
	}

	// System Metrics (averages)
	diff.AvgCPUUsagePercent = (start.CPUUsagePercent + end.CPUUsagePercent) / 2
	diff.AvgMemoryUsageMB = (start.MemoryUsageMB + end.MemoryUsageMB) / 2
	diff.PeakGoroutines = max(start.GoroutinesCount, end.GoroutinesCount)
	diff.AvgActiveConnections = (start.HTTPActiveConnections + end.HTTPActiveConnections) / 2

	return diff
}

// MonitorDuringTest continuously monitors metrics during test execution
func (pc *PrometheusClient) MonitorDuringTest(ctx context.Context, interval time.Duration) ([]*PrometheusMetrics, error) {
	var snapshots []*PrometheusMetrics
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return snapshots, nil
		case <-ticker.C:
			metrics, err := pc.ScrapeMetrics(ctx)
			if err != nil {
				fmt.Printf("Warning: failed to scrape metrics: %v\n", err)
				continue
			}
			snapshots = append(snapshots, metrics)
		}
	}
}

// ExportToJSON exports metrics to JSON file
func (pc *PrometheusClient) ExportToJSON(metrics interface{}, filename string) error {
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return err
	}

	// Write to file
	return os.WriteFile(filename, data, 0644)
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

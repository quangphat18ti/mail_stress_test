package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MonitoringManager orchestrates all monitoring activities during stress test
type MonitoringManager struct {
	prometheusClient *PrometheusClient
	systemMonitor    *SystemMonitor
	config           MonitoringManagerConfig

	// Collected data
	prometheusSnapshots []*PrometheusMetrics
	systemSnapshots     []*SystemMetrics
	startTime           time.Time
	endTime             time.Time
}

// MonitoringManagerConfig configures the monitoring manager
type MonitoringManagerConfig struct {
	// Prometheus settings
	EnablePrometheus bool
	PrometheusURL    string // e.g., "http://localhost:9090/metrics"

	// System monitoring settings
	EnableSystemMonitor bool
	SystemConfig        MonitoringConfig

	// Collection settings
	ScrapeInterval    time.Duration
	OutputDir         string
	EnableRealtimeLog bool
}

// MonitoringReport contains complete monitoring results
type MonitoringReport struct {
	TestInfo struct {
		StartTime time.Time `json:"start_time"`
		EndTime   time.Time `json:"end_time"`
		Duration  string    `json:"duration"`
	} `json:"test_info"`

	// Prometheus metrics
	PrometheusAvailable bool                 `json:"prometheus_available"`
	PrometheusDiff      *MetricsDiff         `json:"prometheus_diff,omitempty"`
	PrometheusSnapshots []*PrometheusMetrics `json:"prometheus_snapshots,omitempty"`

	// System metrics
	SystemAvailable bool             `json:"system_available"`
	SystemSummary   *SystemSummary   `json:"system_summary,omitempty"`
	SystemSnapshots []*SystemMetrics `json:"system_snapshots,omitempty"`

	// Performance insights
	Insights []string `json:"insights"`
}

// SystemSummary provides aggregated system metrics
type SystemSummary struct {
	AvgCPUUsagePercent    float64 `json:"avg_cpu_usage_percent"`
	PeakCPUUsagePercent   float64 `json:"peak_cpu_usage_percent"`
	AvgMemoryUsageMB      float64 `json:"avg_memory_usage_mb"`
	PeakMemoryUsageMB     float64 `json:"peak_memory_usage_mb"`
	AvgMemoryUsagePercent float64 `json:"avg_memory_usage_percent"`
	AvgTCPConnections     float64 `json:"avg_tcp_connections"`
	PeakTCPConnections    int     `json:"peak_tcp_connections"`
	AvgLoadAverage1Min    float64 `json:"avg_load_average_1min"`
}

func NewMonitoringManager(config MonitoringManagerConfig) *MonitoringManager {
	mm := &MonitoringManager{
		config:              config,
		prometheusSnapshots: make([]*PrometheusMetrics, 0),
		systemSnapshots:     make([]*SystemMetrics, 0),
	}

	if config.EnablePrometheus {
		mm.prometheusClient = NewPrometheusClient(config.PrometheusURL)
	}

	if config.EnableSystemMonitor {
		mm.systemMonitor = NewSystemMonitor(config.SystemConfig)
	}

	// Create output directory
	if config.OutputDir != "" {
		os.MkdirAll(config.OutputDir, 0755)
	}

	return mm
}

// StartMonitoring begins collecting metrics
func (mm *MonitoringManager) StartMonitoring(ctx context.Context) error {
	mm.startTime = time.Now()

	fmt.Println("\n🔍 Starting monitoring...")

	// Take initial snapshots
	if mm.prometheusClient != nil {
		metrics, err := mm.prometheusClient.ScrapeMetrics(ctx)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to scrape initial Prometheus metrics: %v\n", err)
		} else {
			mm.prometheusSnapshots = append(mm.prometheusSnapshots, metrics)
			fmt.Println("✅ Prometheus monitoring started")
		}
	}

	if mm.systemMonitor != nil {
		metrics, err := mm.systemMonitor.CollectMetrics(ctx)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to collect initial system metrics: %v\n", err)
		} else {
			mm.systemSnapshots = append(mm.systemSnapshots, metrics)
			fmt.Println("✅ System monitoring started")
		}
	}

	// Start periodic collection in background
	go mm.periodicCollection(ctx)

	return nil
}

// periodicCollection collects metrics at regular intervals
func (mm *MonitoringManager) periodicCollection(ctx context.Context) {
	ticker := time.NewTicker(mm.config.ScrapeInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Collect Prometheus metrics
			if mm.prometheusClient != nil {
				metrics, err := mm.prometheusClient.ScrapeMetrics(ctx)
				if err != nil {
					if mm.config.EnableRealtimeLog {
						fmt.Printf("⚠️  Failed to scrape Prometheus metrics: %v\n", err)
					}
				} else {
					mm.prometheusSnapshots = append(mm.prometheusSnapshots, metrics)
					if mm.config.EnableRealtimeLog {
						fmt.Printf("📊 Prometheus: CPU=%.1f%%, Mem=%.1fMB, Requests=%.0f\n",
							metrics.CPUUsagePercent, metrics.MemoryUsageMB, metrics.HTTPRequestsTotal)
					}
				}
			}

			// Collect system metrics
			if mm.systemMonitor != nil {
				metrics, err := mm.systemMonitor.CollectMetrics(ctx)
				if err != nil {
					if mm.config.EnableRealtimeLog {
						fmt.Printf("⚠️  Failed to collect system metrics: %v\n", err)
					}
				} else {
					mm.systemSnapshots = append(mm.systemSnapshots, metrics)
					if mm.config.EnableRealtimeLog {
						fmt.Printf("💻 System: CPU=%.1f%%, Mem=%.1f%%, Connections=%d\n",
							metrics.CPUUsagePercent, metrics.MemoryUsagePercent, metrics.TCPEstablished)
					}
				}
			}
		}
	}
}

// StopMonitoring stops collecting metrics and generates report
func (mm *MonitoringManager) StopMonitoring(ctx context.Context) (*MonitoringReport, error) {
	mm.endTime = time.Now()

	fmt.Println("\n🛑 Stopping monitoring...")

	// Take final snapshots
	if mm.prometheusClient != nil {
		metrics, err := mm.prometheusClient.ScrapeMetrics(ctx)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to scrape final Prometheus metrics: %v\n", err)
		} else {
			mm.prometheusSnapshots = append(mm.prometheusSnapshots, metrics)
		}
	}

	if mm.systemMonitor != nil {
		metrics, err := mm.systemMonitor.CollectMetrics(ctx)
		if err != nil {
			fmt.Printf("⚠️  Warning: Failed to collect final system metrics: %v\n", err)
		} else {
			mm.systemSnapshots = append(mm.systemSnapshots, metrics)
		}
	}

	// Generate report
	report := mm.generateReport()

	// Save to file
	if mm.config.OutputDir != "" {
		if err := mm.saveReport(report); err != nil {
			return report, fmt.Errorf("failed to save report: %w", err)
		}
	}

	return report, nil
}

// generateReport creates monitoring report from collected data
func (mm *MonitoringManager) generateReport() *MonitoringReport {
	report := &MonitoringReport{
		Insights: make([]string, 0),
	}

	report.TestInfo.StartTime = mm.startTime
	report.TestInfo.EndTime = mm.endTime
	report.TestInfo.Duration = mm.endTime.Sub(mm.startTime).String()

	// Process Prometheus data
	if len(mm.prometheusSnapshots) >= 2 {
		report.PrometheusAvailable = true
		start := mm.prometheusSnapshots[0]
		end := mm.prometheusSnapshots[len(mm.prometheusSnapshots)-1]
		report.PrometheusDiff = mm.prometheusClient.CalculateDiff(start, end)
		report.PrometheusSnapshots = mm.prometheusSnapshots

		// Add insights
		if report.PrometheusDiff.HTTPErrorRatePercent > 5 {
			report.Insights = append(report.Insights,
				fmt.Sprintf("⚠️  High error rate detected: %.2f%%", report.PrometheusDiff.HTTPErrorRatePercent))
		}
		if report.PrometheusDiff.AvgCPUUsagePercent > 80 {
			report.Insights = append(report.Insights,
				fmt.Sprintf("⚠️  High CPU usage: %.2f%%", report.PrometheusDiff.AvgCPUUsagePercent))
		}
		if report.PrometheusDiff.AvgMemoryUsageMB > 1024 {
			report.Insights = append(report.Insights,
				fmt.Sprintf("⚠️  High memory usage: %.2fMB", report.PrometheusDiff.AvgMemoryUsageMB))
		}
	}

	// Process system data
	if len(mm.systemSnapshots) >= 2 {
		report.SystemAvailable = true
		report.SystemSummary = mm.calculateSystemSummary()
		report.SystemSnapshots = mm.systemSnapshots

		// Add system insights
		if report.SystemSummary.PeakCPUUsagePercent > 90 {
			report.Insights = append(report.Insights,
				fmt.Sprintf("🔥 CPU peaked at %.2f%% - consider scaling", report.SystemSummary.PeakCPUUsagePercent))
		}
		if report.SystemSummary.AvgMemoryUsagePercent > 85 {
			report.Insights = append(report.Insights,
				fmt.Sprintf("🔥 Memory usage high: %.2f%% - risk of OOM", report.SystemSummary.AvgMemoryUsagePercent))
		}
		if report.SystemSummary.PeakTCPConnections > 1000 {
			report.Insights = append(report.Insights,
				fmt.Sprintf("📡 Peak connections: %d - ensure connection pooling", report.SystemSummary.PeakTCPConnections))
		}
	}

	return report
}

// calculateSystemSummary computes aggregate metrics from system snapshots
func (mm *MonitoringManager) calculateSystemSummary() *SystemSummary {
	if len(mm.systemSnapshots) == 0 {
		return nil
	}

	summary := &SystemSummary{}
	count := float64(len(mm.systemSnapshots))

	for _, snapshot := range mm.systemSnapshots {
		summary.AvgCPUUsagePercent += snapshot.CPUUsagePercent
		summary.AvgMemoryUsageMB += snapshot.UsedMemoryMB
		summary.AvgMemoryUsagePercent += snapshot.MemoryUsagePercent
		summary.AvgTCPConnections += float64(snapshot.TCPEstablished)
		summary.AvgLoadAverage1Min += snapshot.LoadAverage1Min

		if snapshot.CPUUsagePercent > summary.PeakCPUUsagePercent {
			summary.PeakCPUUsagePercent = snapshot.CPUUsagePercent
		}
		if snapshot.UsedMemoryMB > summary.PeakMemoryUsageMB {
			summary.PeakMemoryUsageMB = snapshot.UsedMemoryMB
		}
		if snapshot.TCPEstablished > summary.PeakTCPConnections {
			summary.PeakTCPConnections = snapshot.TCPEstablished
		}
	}

	summary.AvgCPUUsagePercent /= count
	summary.AvgMemoryUsageMB /= count
	summary.AvgMemoryUsagePercent /= count
	summary.AvgTCPConnections /= count
	summary.AvgLoadAverage1Min /= count

	return summary
}

// saveReport saves monitoring report to JSON file
func (mm *MonitoringManager) saveReport(report *MonitoringReport) error {
	timestamp := time.Now().Format("20060102_150405")
	filename := filepath.Join(mm.config.OutputDir, fmt.Sprintf("monitoring_%s.json", timestamp))

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return err
	}

	fmt.Printf("\n📊 Monitoring report saved: %s\n", filename)
	return nil
}

// PrintSummary prints a human-readable summary of monitoring results
func (mm *MonitoringManager) PrintSummary(report *MonitoringReport) {
	fmt.Println("\n" + strings.Repeat("=", 100))
	fmt.Println("📊 MONITORING SUMMARY")
	fmt.Println(strings.Repeat("=", 100))

	fmt.Printf("\n⏱️  Test Duration: %s\n", report.TestInfo.Duration)
	fmt.Printf("📅 Start: %s\n", report.TestInfo.StartTime.Format("2006-01-02 15:04:05"))
	fmt.Printf("📅 End:   %s\n", report.TestInfo.EndTime.Format("2006-01-02 15:04:05"))

	// Prometheus summary
	if report.PrometheusAvailable && report.PrometheusDiff != nil {
		fmt.Println("\n🔍 Prometheus Metrics:")
		fmt.Println("   " + strings.Repeat("-", 80))
		diff := report.PrometheusDiff
		fmt.Printf("   HTTP Requests:      %.0f total (%.2f req/s)\n",
			diff.HTTPRequestsIncrease, diff.HTTPRequestsPerSecond)
		fmt.Printf("   Error Rate:         %.2f%%\n", diff.HTTPErrorRatePercent)
		fmt.Printf("   Avg CPU:            %.2f%%\n", diff.AvgCPUUsagePercent)
		fmt.Printf("   Avg Memory:         %.2f MB\n", diff.AvgMemoryUsageMB)
		fmt.Printf("   Peak Goroutines:    %.0f\n", diff.PeakGoroutines)
		fmt.Printf("   Avg Connections:    %.0f\n", diff.AvgActiveConnections)

		if diff.EndMetrics != nil {
			fmt.Printf("\n   Response Times (End of Test):\n")
			fmt.Printf("   P50: %.2fms | P95: %.2fms | P99: %.2fms\n",
				diff.EndMetrics.HTTPRequestDurationP50,
				diff.EndMetrics.HTTPRequestDurationP95,
				diff.EndMetrics.HTTPRequestDurationP99)
		}
	}

	// System summary
	if report.SystemAvailable && report.SystemSummary != nil {
		fmt.Println("\n💻 System Metrics:")
		fmt.Println("   " + strings.Repeat("-", 80))
		summary := report.SystemSummary
		fmt.Printf("   CPU Usage:          Avg: %.2f%% | Peak: %.2f%%\n",
			summary.AvgCPUUsagePercent, summary.PeakCPUUsagePercent)
		fmt.Printf("   Memory Usage:       Avg: %.2fMB (%.2f%%) | Peak: %.2fMB\n",
			summary.AvgMemoryUsageMB, summary.AvgMemoryUsagePercent, summary.PeakMemoryUsageMB)
		fmt.Printf("   TCP Connections:    Avg: %.0f | Peak: %d\n",
			summary.AvgTCPConnections, summary.PeakTCPConnections)
		fmt.Printf("   Load Average (1m):  %.2f\n", summary.AvgLoadAverage1Min)
	}

	// Insights
	if len(report.Insights) > 0 {
		fmt.Println("\n💡 Performance Insights:")
		fmt.Println("   " + strings.Repeat("-", 80))
		for _, insight := range report.Insights {
			fmt.Printf("   %s\n", insight)
		}
	} else {
		fmt.Println("\n✅ No performance issues detected!")
	}

	fmt.Println("\n" + strings.Repeat("=", 100))
}

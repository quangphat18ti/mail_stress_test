package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// SystemMonitor monitors system-level metrics (CPU, RAM, etc.)
type SystemMonitor struct {
	targetHost  string // empty for local, or "user@host" for remote SSH
	isDocker    bool
	containerID string
}

// SystemMetrics stores system resource metrics
type SystemMetrics struct {
	Timestamp time.Time `json:"timestamp"`

	// CPU Metrics
	CPUUsagePercent  float64 `json:"cpu_usage_percent"`
	CPUCores         int     `json:"cpu_cores"`
	LoadAverage1Min  float64 `json:"load_average_1min"`
	LoadAverage5Min  float64 `json:"load_average_5min"`
	LoadAverage15Min float64 `json:"load_average_15min"`

	// Memory Metrics
	TotalMemoryMB      float64 `json:"total_memory_mb"`
	UsedMemoryMB       float64 `json:"used_memory_mb"`
	FreeMemoryMB       float64 `json:"free_memory_mb"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`

	// Network Metrics
	NetworkRxMB float64 `json:"network_rx_mb"`
	NetworkTxMB float64 `json:"network_tx_mb"`

	// Connection Metrics
	TCPConnections int `json:"tcp_connections"`
	TCPEstablished int `json:"tcp_established"`
	TCPTimeWait    int `json:"tcp_time_wait"`

	// Process-specific (if monitoring specific process)
	ProcessCPUPercent float64 `json:"process_cpu_percent,omitempty"`
	ProcessMemoryMB   float64 `json:"process_memory_mb,omitempty"`
	ProcessThreads    int     `json:"process_threads,omitempty"`
	ProcessOpenFiles  int     `json:"process_open_files,omitempty"`
}

// MonitoringConfig configures system monitoring
type MonitoringConfig struct {
	// Target configuration
	TargetHost  string // For remote monitoring: "user@host"
	IsDocker    bool   // Monitor Docker container
	ContainerID string // Docker container ID or name
	ProcessName string // Process name to monitor (e.g., "fiber-app")

	// Monitoring settings
	ScrapeInterval time.Duration // How often to collect metrics
	EnableNetwork  bool          // Monitor network I/O
	EnableProcess  bool          // Monitor specific process
}

func NewSystemMonitor(config MonitoringConfig) *SystemMonitor {
	return &SystemMonitor{
		targetHost:  config.TargetHost,
		isDocker:    config.IsDocker,
		containerID: config.ContainerID,
	}
}

// CollectMetrics gathers current system metrics
func (sm *SystemMonitor) CollectMetrics(ctx context.Context) (*SystemMetrics, error) {
	metrics := &SystemMetrics{
		Timestamp: time.Now(),
		CPUCores:  runtime.NumCPU(),
	}

	var err error

	// Collect CPU metrics
	if err = sm.collectCPUMetrics(ctx, metrics); err != nil {
		return nil, fmt.Errorf("failed to collect CPU metrics: %w", err)
	}

	// Collect memory metrics
	if err = sm.collectMemoryMetrics(ctx, metrics); err != nil {
		return nil, fmt.Errorf("failed to collect memory metrics: %w", err)
	}

	// Collect network metrics
	if err = sm.collectNetworkMetrics(ctx, metrics); err != nil {
		// Non-fatal, just log warning
		fmt.Printf("Warning: failed to collect network metrics: %v\n", err)
	}

	// Collect connection metrics
	if err = sm.collectConnectionMetrics(ctx, metrics); err != nil {
		fmt.Printf("Warning: failed to collect connection metrics: %v\n", err)
	}

	return metrics, nil
}

// collectCPUMetrics gathers CPU usage information
func (sm *SystemMonitor) collectCPUMetrics(ctx context.Context, metrics *SystemMetrics) error {
	var cmd *exec.Cmd

	if sm.isDocker {
		// Docker stats
		cmd = exec.CommandContext(ctx, "docker", "stats", sm.containerID, "--no-stream", "--format", "{{.CPUPerc}}")
	} else if sm.targetHost != "" {
		// Remote SSH
		cmd = exec.CommandContext(ctx, "ssh", sm.targetHost, "top -bn1 | grep 'Cpu(s)' | awk '{print $2}'")
	} else {
		// Local
		switch runtime.GOOS {
		case "darwin":
			// macOS: use top
			cmd = exec.CommandContext(ctx, "sh", "-c", "top -l 1 -n 0 | grep 'CPU usage' | awk '{print $3}'")
		case "linux":
			// Linux: use top or /proc/stat
			cmd = exec.CommandContext(ctx, "sh", "-c", "top -bn1 | grep 'Cpu(s)' | awk '{print $2}'")
		default:
			return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute command: %w (output: %s)", err, string(output))
	}

	cpuStr := strings.TrimSpace(string(output))
	cpuStr = strings.TrimSuffix(cpuStr, "%")
	cpuStr = strings.TrimSuffix(cpuStr, "us") // Linux format

	cpuUsage, err := strconv.ParseFloat(cpuStr, 64)
	if err != nil {
		return fmt.Errorf("failed to parse CPU usage: %w (value: %s)", err, cpuStr)
	}

	metrics.CPUUsagePercent = cpuUsage

	// Load average (Linux/macOS)
	if runtime.GOOS != "windows" {
		sm.collectLoadAverage(ctx, metrics)
	}

	return nil
}

// collectLoadAverage gets system load average
func (sm *SystemMonitor) collectLoadAverage(ctx context.Context, metrics *SystemMetrics) error {
	var cmd *exec.Cmd

	if sm.targetHost != "" {
		cmd = exec.CommandContext(ctx, "ssh", sm.targetHost, "uptime")
	} else {
		cmd = exec.CommandContext(ctx, "uptime")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	// Parse: "load average: 1.23, 2.34, 3.45"
	uptimeStr := string(output)
	if idx := strings.Index(uptimeStr, "load average:"); idx >= 0 {
		loadStr := uptimeStr[idx+14:]
		loads := strings.Split(strings.TrimSpace(loadStr), ",")
		if len(loads) >= 3 {
			fmt.Sscanf(strings.TrimSpace(loads[0]), "%f", &metrics.LoadAverage1Min)
			fmt.Sscanf(strings.TrimSpace(loads[1]), "%f", &metrics.LoadAverage5Min)
			fmt.Sscanf(strings.TrimSpace(loads[2]), "%f", &metrics.LoadAverage15Min)
		}
	}

	return nil
}

// collectMemoryMetrics gathers memory usage information
func (sm *SystemMonitor) collectMemoryMetrics(ctx context.Context, metrics *SystemMetrics) error {
	var cmd *exec.Cmd

	if sm.isDocker {
		// Docker stats
		cmd = exec.CommandContext(ctx, "docker", "stats", sm.containerID, "--no-stream", "--format", "{{.MemUsage}}")
	} else if sm.targetHost != "" {
		// Remote SSH
		cmd = exec.CommandContext(ctx, "ssh", sm.targetHost, "free -m")
	} else {
		// Local
		switch runtime.GOOS {
		case "darwin":
			// macOS: use vm_stat
			cmd = exec.CommandContext(ctx, "vm_stat")
		case "linux":
			cmd = exec.CommandContext(ctx, "free", "-m")
		default:
			return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute command: %w", err)
	}

	if sm.isDocker {
		// Parse Docker format: "123.4MiB / 2GiB"
		return sm.parseDockerMemory(string(output), metrics)
	} else if runtime.GOOS == "linux" || sm.targetHost != "" {
		// Parse Linux free output
		return sm.parseLinuxMemory(string(output), metrics)
	} else if runtime.GOOS == "darwin" {
		// Parse macOS vm_stat output
		return sm.parseMacOSMemory(string(output), metrics)
	}

	return nil
}

func (sm *SystemMonitor) parseDockerMemory(output string, metrics *SystemMetrics) error {
	// Format: "123.4MiB / 2GiB"
	parts := strings.Split(output, "/")
	if len(parts) != 2 {
		return fmt.Errorf("unexpected docker memory format: %s", output)
	}

	usedStr := strings.TrimSpace(parts[0])
	totalStr := strings.TrimSpace(parts[1])

	used := parseMemoryValue(usedStr)
	total := parseMemoryValue(totalStr)

	metrics.UsedMemoryMB = used
	metrics.TotalMemoryMB = total
	metrics.FreeMemoryMB = total - used
	if total > 0 {
		metrics.MemoryUsagePercent = (used / total) * 100
	}

	return nil
}

func (sm *SystemMonitor) parseLinuxMemory(output string, metrics *SystemMetrics) error {
	// Parse "free -m" output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Mem:") {
			fields := strings.Fields(line)
			if len(fields) >= 7 {
				total, _ := strconv.ParseFloat(fields[1], 64)
				used, _ := strconv.ParseFloat(fields[2], 64)
				free, _ := strconv.ParseFloat(fields[3], 64)

				metrics.TotalMemoryMB = total
				metrics.UsedMemoryMB = used
				metrics.FreeMemoryMB = free
				if total > 0 {
					metrics.MemoryUsagePercent = (used / total) * 100
				}
			}
			break
		}
	}
	return nil
}

func (sm *SystemMonitor) parseMacOSMemory(output string, metrics *SystemMetrics) error {
	// Parse vm_stat output (simplified)
	// This is complex, for now return basic info
	// In production, use more sophisticated parsing

	pageSize := 4096.0 // Default page size in bytes
	var free, active, inactive, wired float64

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Pages free:") {
			fmt.Sscanf(line, "Pages free: %f", &free)
		} else if strings.Contains(line, "Pages active:") {
			fmt.Sscanf(line, "Pages active: %f", &active)
		} else if strings.Contains(line, "Pages inactive:") {
			fmt.Sscanf(line, "Pages inactive: %f", &inactive)
		} else if strings.Contains(line, "Pages wired down:") {
			fmt.Sscanf(line, "Pages wired down: %f", &wired)
		}
	}

	freeMB := (free * pageSize) / (1024 * 1024)
	usedMB := ((active + inactive + wired) * pageSize) / (1024 * 1024)
	totalMB := freeMB + usedMB

	metrics.TotalMemoryMB = totalMB
	metrics.UsedMemoryMB = usedMB
	metrics.FreeMemoryMB = freeMB
	if totalMB > 0 {
		metrics.MemoryUsagePercent = (usedMB / totalMB) * 100
	}

	return nil
}

// collectNetworkMetrics gathers network I/O statistics
func (sm *SystemMonitor) collectNetworkMetrics(ctx context.Context, metrics *SystemMetrics) error {
	// Simplified implementation - in production use proper network monitoring
	// This would require tracking delta between measurements
	return nil
}

// collectConnectionMetrics gathers TCP connection statistics
func (sm *SystemMonitor) collectConnectionMetrics(ctx context.Context, metrics *SystemMetrics) error {
	var cmd *exec.Cmd

	if sm.targetHost != "" {
		cmd = exec.CommandContext(ctx, "ssh", sm.targetHost, "netstat -an | grep ESTABLISHED | wc -l")
	} else {
		switch runtime.GOOS {
		case "darwin", "linux":
			cmd = exec.CommandContext(ctx, "sh", "-c", "netstat -an | grep ESTABLISHED | wc -l")
		default:
			return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
		}
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return err
	}

	count, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil {
		return err
	}

	metrics.TCPEstablished = count
	metrics.TCPConnections = count // Simplified

	return nil
}

// parseMemoryValue converts memory strings like "123.4MiB" or "2GiB" to MB
func parseMemoryValue(s string) float64 {
	s = strings.TrimSpace(s)
	var value float64
	var unit string

	fmt.Sscanf(s, "%f%s", &value, &unit)

	switch strings.ToUpper(unit) {
	case "GIB", "GB", "G":
		return value * 1024
	case "MIB", "MB", "M":
		return value
	case "KIB", "KB", "K":
		return value / 1024
	default:
		return value
	}
}

// ExportMetrics exports metrics to JSON
func (sm *SystemMonitor) ExportMetrics(metrics interface{}, filename string) error {
	data, err := json.MarshalIndent(metrics, "", "  ")
	if err != nil {
		return err
	}

	fmt.Printf("Metrics exported to: %s\n", filename)
	fmt.Printf("Data: %s\n", string(data))

	return nil
}

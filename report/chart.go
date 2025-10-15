package report

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mail-stress-test/benchmark"
)

type ChartGenerator struct {
	outputDir string
}

func NewChartGenerator(outputDir string) *ChartGenerator {
	return &ChartGenerator{outputDir: outputDir}
}

func (cg *ChartGenerator) GenerateCharts(stressResult *benchmark.StressTestResult, searchResults map[string]*benchmark.SearchBenchmarkResult) error {
	// Generate HTML with Chart.js
	if err := cg.generateHTMLChart(stressResult, searchResults); err != nil {
		return err
	}

	return nil
}

func (cg *ChartGenerator) generateHTMLChart(stressResult *benchmark.StressTestResult, searchResults map[string]*benchmark.SearchBenchmarkResult) error {
	filename := filepath.Join(cg.outputDir, fmt.Sprintf("charts_%s.html", time.Now().Format("20060102_150405")))

	html := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>Mail System Benchmark Report</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <style>
        body {
            font-family: Arial, sans-serif;
            max-width: 1200px;
            margin: 0 auto;
            padding: 20px;
            background-color: #f5f5f5;
        }
        .chart-container {
            background: white;
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        h1, h2 {
            color: #333;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 15px;
            margin: 20px 0;
        }
        .stat-card {
            background: white;
            padding: 15px;
            border-radius: 8px;
            box-shadow: 0 2px 4px rgba(0,0,0,0.1);
        }
        .stat-label {
            color: #666;
            font-size: 14px;
        }
        .stat-value {
            font-size: 24px;
            font-weight: bold;
            color: #333;
            margin-top: 5px;
        }
    </style>
</head>
<body>
    <h1>📊 Mail System Stress Test & Benchmark Report</h1>
    <p>Generated: ` + time.Now().Format("2006-01-02 15:04:05") + `</p>
    
    <h2>Summary Statistics</h2>
    <div class="stats-grid">
        <div class="stat-card">
            <div class="stat-label">Total Requests</div>
            <div class="stat-value">` + fmt.Sprintf("%d", stressResult.TotalRequests) + `</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Success Rate</div>
            <div class="stat-value">` + fmt.Sprintf("%.2f%%", 100-stressResult.ErrorRate) + `</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Avg Response Time</div>
            <div class="stat-value">` + stressResult.AvgResponseTime.String() + `</div>
        </div>
        <div class="stat-card">
            <div class="stat-label">Requests/Second</div>
            <div class="stat-value">` + fmt.Sprintf("%.2f", stressResult.RequestsPerSecond) + `</div>
        </div>
    </div>
    
    <div class="chart-container">
        <h2>Operation Performance</h2>
        <canvas id="operationChart"></canvas>
    </div>
    
    <div class="chart-container">
        <h2>Search Method Comparison</h2>
        <canvas id="searchChart"></canvas>
    </div>
    
    <div class="chart-container">
        <h2>Response Time Distribution</h2>
        <canvas id="responseTimeChart"></canvas>
    </div>
    
    <script>
        // Operation Performance Chart
        const operationCtx = document.getElementById('operationChart').getContext('2d');
        new Chart(operationCtx, {
            type: 'bar',
            data: {
                labels: [`
	// Add operation labels
	for op := range stressResult.OperationStats {
		html += "'" + op + "', "
	}
	html += `],
                datasets: [{
                    label: 'Average Duration (ms)',
                    data: [`
	for _, stats := range stressResult.OperationStats {
		html += fmt.Sprintf("%d, ", stats.AvgDuration.Milliseconds())
	}
	html += `],
                    backgroundColor: 'rgba(54, 162, 235, 0.8)'
                }, {
                    label: 'Error Count',
                    data: [`
	for _, stats := range stressResult.OperationStats {
		html += fmt.Sprintf("%d, ", stats.Errors)
	}
	html += `],
                    backgroundColor: 'rgba(255, 99, 132, 0.8)'
                }]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true
                    }
                }
            }
        });
        
        // Search Method Comparison Chart
        const searchCtx = document.getElementById('searchChart').getContext('2d');
        new Chart(searchCtx, {
            type: 'bar',
            data: {
                labels: [`
	for method := range searchResults {
		html += "'" + method + "', "
	}
	html += `],
                datasets: [{
                    label: 'Average Duration (ms)',
                    data: [`
	for _, result := range searchResults {
		html += fmt.Sprintf("%d, ", result.AvgDuration.Milliseconds())
	}
	html += `],
                    backgroundColor: [
                        'rgba(255, 99, 132, 0.8)',
                        'rgba(54, 162, 235, 0.8)',
                        'rgba(255, 206, 86, 0.8)'
                    ]
                }]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true
                    }
                },
                plugins: {
                    title: {
                        display: true,
                        text: 'Search Performance Comparison'
                    }
                }
            }
        });
        
        // Response Time Distribution Chart
        const responseCtx = document.getElementById('responseTimeChart').getContext('2d');
        new Chart(responseCtx, {
            type: 'line',
            data: {
                labels: ['Min', 'Average', 'Max'],
                datasets: [{
                    label: 'Response Time (ms)',
                    data: [` + fmt.Sprintf("%d, %d, %d",
		stressResult.MinResponseTime.Milliseconds(),
		stressResult.AvgResponseTime.Milliseconds(),
		stressResult.MaxResponseTime.Milliseconds()) + `],
                    borderColor: 'rgba(75, 192, 192, 1)',
                    backgroundColor: 'rgba(75, 192, 192, 0.2)',
                    fill: true
                }]
            },
            options: {
                responsive: true,
                scales: {
                    y: {
                        beginAtZero: true
                    }
                }
            }
        });
    </script>
</body>
</html>`

	return os.WriteFile(filename, []byte(html), 0644)
}

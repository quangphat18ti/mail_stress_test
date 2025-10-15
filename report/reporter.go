package report

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mail-stress-test/benchmark"
)

type Report struct {
	Timestamp        time.Time                                   `json:"timestamp"`
	StressTestResult *benchmark.StressTestResult                 `json:"stress_test_result"`
	SearchBenchmark  map[string]*benchmark.SearchBenchmarkResult `json:"search_benchmark"`
}

type Reporter struct {
	outputDir string
}

func NewReporter(outputDir string) *Reporter {
	os.MkdirAll(outputDir, 0755)
	return &Reporter{outputDir: outputDir}
}

func (r *Reporter) GenerateReport(stressResult *benchmark.StressTestResult, searchResults map[string]*benchmark.SearchBenchmarkResult) error {
	report := &Report{
		Timestamp:        time.Now(),
		StressTestResult: stressResult,
		SearchBenchmark:  searchResults,
	}

	// Generate JSON report
	if err := r.generateJSONReport(report); err != nil {
		return err
	}

	// Generate text summary
	if err := r.generateTextSummary(report); err != nil {
		return err
	}

	return nil
}

func (r *Reporter) generateJSONReport(report *Report) error {
	filename := filepath.Join(r.outputDir, fmt.Sprintf("report_%s.json", time.Now().Format("20060102_150405")))

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

func (r *Reporter) generateTextSummary(report *Report) error {
	filename := filepath.Join(r.outputDir, fmt.Sprintf("summary_%s.txt", time.Now().Format("20060102_150405")))

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "=== Mail System Stress Test Report ===\n")
	fmt.Fprintf(f, "Generated: %s\n\n", report.Timestamp.Format(time.RFC3339))

	// Stress Test Results
	st := report.StressTestResult
	if st != nil {
		fmt.Fprintf(f, "--- Stress Test Results ---\n")
		fmt.Fprintf(f, "Total Requests: %d\n", st.TotalRequests)
		fmt.Fprintf(f, "Success Requests: %d\n", st.SuccessRequests)
		fmt.Fprintf(f, "Failed Requests: %d\n", st.FailedRequests)
		fmt.Fprintf(f, "Error Rate: %.2f%%\n", st.ErrorRate)
		fmt.Fprintf(f, "Total Duration: %s\n", st.TotalDuration)
		fmt.Fprintf(f, "Avg Response Time: %s\n", st.AvgResponseTime)
		fmt.Fprintf(f, "Min Response Time: %s\n", st.MinResponseTime)
		fmt.Fprintf(f, "Max Response Time: %s\n", st.MaxResponseTime)
		fmt.Fprintf(f, "Requests/Second: %.2f\n\n", st.RequestsPerSecond)

		fmt.Fprintf(f, "--- Operation Statistics ---\n")
		for op, stats := range st.OperationStats {
			fmt.Fprintf(f, "\n%s:\n", op)
			fmt.Fprintf(f, "  Count: %d\n", stats.Count)
			fmt.Fprintf(f, "  Avg Duration: %s\n", stats.AvgDuration)
			fmt.Fprintf(f, "  Min Duration: %s\n", stats.MinDuration)
			fmt.Fprintf(f, "  Max Duration: %s\n", stats.MaxDuration)
			fmt.Fprintf(f, "  Errors: %d\n", stats.Errors)
		}
	}

	// Search Benchmark Results
	if report.SearchBenchmark != nil {
		fmt.Fprintf(f, "\n--- Search Benchmark Results ---\n")
		for method, result := range report.SearchBenchmark {
			fmt.Fprintf(f, "\n%s:\n", method)
			fmt.Fprintf(f, "  Total Queries: %d\n", result.TotalQueries)
			fmt.Fprintf(f, "  Success: %d\n", result.SuccessQueries)
			fmt.Fprintf(f, "  Failed: %d\n", result.FailedQueries)
			fmt.Fprintf(f, "  Avg Duration: %s\n", result.AvgDuration)
			fmt.Fprintf(f, "  Min Duration: %s\n", result.MinDuration)
			fmt.Fprintf(f, "  Max Duration: %s\n", result.MaxDuration)
		}
	}

	return nil
}

package benchmark

import (
	"context"
	"fmt"
	"time"

	"mail-stress-test/config"
	"mail-stress-test/database"
	"mail-stress-test/generator"
	"mail-stress-test/search"
)

// SearchBenchmarkResult holds the results of a search strategy benchmark
type SearchBenchmarkResult struct {
	StrategyName   string        `json:"strategy_name"`
	Description    string        `json:"description"`
	SetupDuration  time.Duration `json:"setup_duration"`
	AvgDuration    time.Duration `json:"avg_duration"`
	MinDuration    time.Duration `json:"min_duration"`
	MaxDuration    time.Duration `json:"max_duration"`
	P50Duration    time.Duration `json:"p50_duration"`
	P95Duration    time.Duration `json:"p95_duration"`
	P99Duration    time.Duration `json:"p99_duration"`
	TotalQueries   int           `json:"total_queries"`
	SuccessQueries int           `json:"success_queries"`
	FailedQueries  int           `json:"failed_queries"`
	TotalResults   int           `json:"total_results"`
	AvgResults     float64       `json:"avg_results"`
}

// SearchBenchmark benchmarks different search strategies
type SearchBenchmark struct {
	config     *config.Config
	db         *database.MongoDB
	generator  *generator.DataGenerator
	strategies []search.SearchStrategy
}

// NewSearchBenchmark creates a new search benchmark
func NewSearchBenchmark(cfg *config.Config, db *database.MongoDB, gen *generator.DataGenerator) *SearchBenchmark {
	return &SearchBenchmark{
		config:    cfg,
		db:        db,
		generator: gen,
		strategies: []search.SearchStrategy{
			search.NewTextSearchStrategy(),
			search.NewRegexSearchStrategy(),
			search.NewAggregationSearchStrategy(),
			search.NewIndexOptimizedStrategy(),
		},
	}
}

// Run executes the benchmark for all strategies
func (sb *SearchBenchmark) Run(ctx context.Context) (map[string]*SearchBenchmarkResult, error) {
	results := make(map[string]*SearchBenchmarkResult)

	fmt.Println("\n=== Search Strategy Benchmark ===")
	fmt.Printf("Testing %d strategies with %d iterations each\n\n",
		len(sb.strategies), sb.config.Benchmark.Iterations)

	for _, strategy := range sb.strategies {
		fmt.Printf("Testing strategy: %s\n", strategy.GetName())
		fmt.Printf("  Description: %s\n", strategy.GetDescription())

		result, err := sb.benchmarkStrategy(ctx, strategy)
		if err != nil {
			fmt.Printf("  ❌ Failed: %v\n\n", err)
			continue
		}

		results[strategy.GetName()] = result

		// Print results
		fmt.Printf("  ✅ Setup: %s\n", result.SetupDuration)
		fmt.Printf("  📊 Avg: %s, Min: %s, Max: %s\n",
			result.AvgDuration, result.MinDuration, result.MaxDuration)
		fmt.Printf("  📈 P50: %s, P95: %s, P99: %s\n",
			result.P50Duration, result.P95Duration, result.P99Duration)
		fmt.Printf("  ✓ Success: %d/%d (%.1f%%)\n",
			result.SuccessQueries, result.TotalQueries,
			float64(result.SuccessQueries)/float64(result.TotalQueries)*100)
		fmt.Printf("  📧 Avg Results: %.1f mails per query\n\n", result.AvgResults)
	}

	return results, nil
}

// benchmarkStrategy benchmarks a single search strategy
func (sb *SearchBenchmark) benchmarkStrategy(ctx context.Context, strategy search.SearchStrategy) (*SearchBenchmarkResult, error) {
	result := &SearchBenchmarkResult{
		StrategyName: strategy.GetName(),
		Description:  strategy.GetDescription(),
		MinDuration:  time.Hour,
	}

	// Setup database for this strategy
	setupStart := time.Now()
	if err := strategy.SetupDatabase(ctx, sb.db); err != nil {
		return nil, fmt.Errorf("setup failed: %w", err)
	}
	result.SetupDuration = time.Since(setupStart)

	// Wait a bit for indexes to be ready
	time.Sleep(100 * time.Millisecond)

	// Collect durations for percentile calculation
	durations := make([]time.Duration, 0, sb.config.Benchmark.Iterations)

	// Run benchmark iterations
	for i := 0; i < sb.config.Benchmark.Iterations; i++ {
		req := sb.generator.GenerateSearchMailsRequest()

		start := time.Now()
		mails, err := strategy.SearchMails(ctx, sb.db, req)
		duration := time.Since(start)

		result.TotalQueries++

		if err != nil {
			result.FailedQueries++
			continue
		}

		result.SuccessQueries++
		result.TotalResults += len(mails)
		durations = append(durations, duration)

		// Update min/max
		if duration < result.MinDuration {
			result.MinDuration = duration
		}
		if duration > result.MaxDuration {
			result.MaxDuration = duration
		}
	}

	// Calculate average
	if result.SuccessQueries > 0 {
		var totalDuration time.Duration
		for _, d := range durations {
			totalDuration += d
		}
		result.AvgDuration = totalDuration / time.Duration(result.SuccessQueries)
		result.AvgResults = float64(result.TotalResults) / float64(result.SuccessQueries)
	}

	// Calculate percentiles
	if len(durations) > 0 {
		result.P50Duration = calculatePercentile(durations, 50)
		result.P95Duration = calculatePercentile(durations, 95)
		result.P99Duration = calculatePercentile(durations, 99)
	}

	return result, nil
}

// calculatePercentile calculates the nth percentile of durations
func calculatePercentile(durations []time.Duration, percentile int) time.Duration {
	if len(durations) == 0 {
		return 0
	}

	// Simple bubble sort for small datasets
	sorted := make([]time.Duration, len(durations))
	copy(sorted, durations)

	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	index := (len(sorted) * percentile) / 100
	if index >= len(sorted) {
		index = len(sorted) - 1
	}

	return sorted[index]
}

// GenerateComparisonReport generates a textual comparison of all strategies
func (sb *SearchBenchmark) GenerateComparisonReport(results map[string]*SearchBenchmarkResult) string {
	report := "\n=== Search Strategy Comparison Report ===\n\n"

	// Find best performers
	var fastestAvg, fastestP99, mostReliable string
	var minAvg, minP99 time.Duration = time.Hour, time.Hour
	var maxSuccess float64 = 0

	for name, result := range results {
		if result.SuccessQueries > 0 {
			successRate := float64(result.SuccessQueries) / float64(result.TotalQueries)

			if result.AvgDuration < minAvg {
				minAvg = result.AvgDuration
				fastestAvg = name
			}
			if result.P99Duration < minP99 {
				minP99 = result.P99Duration
				fastestP99 = name
			}
			if successRate > maxSuccess {
				maxSuccess = successRate
				mostReliable = name
			}
		}
	}

	report += fmt.Sprintf("🏆 Fastest Average: %s (%s)\n", fastestAvg, minAvg)
	report += fmt.Sprintf("🏆 Fastest P99: %s (%s)\n", fastestP99, minP99)
	report += fmt.Sprintf("🏆 Most Reliable: %s (%.1f%% success)\n\n", mostReliable, maxSuccess*100)

	report += "Recommendations:\n"
	report += fmt.Sprintf("  • For best average performance: Use '%s'\n", fastestAvg)
	report += fmt.Sprintf("  • For consistent latency: Use '%s'\n", fastestP99)
	report += fmt.Sprintf("  • For reliability: Use '%s'\n", mostReliable)

	return report
}

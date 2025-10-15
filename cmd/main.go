package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"mail-stress-test/benchmark"
	"mail-stress-test/config"
	"mail-stress-test/database"
	"mail-stress-test/generator"
	"mail-stress-test/report"
)

func main() {
	configPath := flag.String("config", "", "Path to config file")
	seedData := flag.Bool("seed", false, "Seed initial data")
	runStress := flag.Bool("stress", true, "Run stress test")
	runBenchmark := flag.Bool("benchmark", true, "Run search benchmark")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Connect to MongoDB
	db, err := database.NewMongoDB(cfg.MongoDB.URI, cfg.MongoDB.Database, cfg.MongoDB.Timeout)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down gracefully...")
		cancel()
	}()

	// Create indexes
	fmt.Println("Creating database indexes...")
	if err := db.CreateIndexes(ctx); err != nil {
		log.Fatalf("Failed to create indexes: %v", err)
	}

	// Seed data if requested
	if *seedData {
		fmt.Println("Seeding test data...")
		dataGen := generator.NewDataGenerator(db)
		if err := dataGen.SeedData(ctx, cfg.StressTest.NumUsers, cfg.StressTest.NumMailsPerUser); err != nil {
			log.Fatalf("Failed to seed data: %v", err)
		}
		fmt.Println("Data seeding completed!")
	}

	var stressResult *benchmark.StressTestResult
	var searchResults map[string]*benchmark.SearchBenchmarkResult

	// Run stress test
	if *runStress {
		fmt.Println("\n=== Running Stress Test ===")
		stressTest := benchmark.NewStressTest(cfg, db)
		stressResult, err = stressTest.Run(ctx)
		if err != nil {
			log.Fatalf("Stress test failed: %v", err)
		}

		fmt.Printf("\nStress Test Results:\n")
		fmt.Printf("  Total Requests: %d\n", stressResult.TotalRequests)
		if stressResult.TotalRequests > 0 {
			fmt.Printf("  Success: %d (%.2f%%)\n", stressResult.SuccessRequests,
				float64(stressResult.SuccessRequests)/float64(stressResult.TotalRequests)*100)
		} else {
			fmt.Printf("  Success: %d\n", stressResult.SuccessRequests)
		}
		fmt.Printf("  Failed: %d (%.2f%%)\n", stressResult.FailedRequests, stressResult.ErrorRate)
		fmt.Printf("  Avg Response Time: %s\n", stressResult.AvgResponseTime)
		fmt.Printf("  Requests/Second: %.2f\n", stressResult.RequestsPerSecond)
	}

	// Run search benchmark
	if *runBenchmark {
		fmt.Println("\n=== Running Search Benchmark ===")
		searchBench := benchmark.NewSearchBenchmark(cfg, db)
		searchResults, err = searchBench.Run(ctx)
		if err != nil {
			log.Fatalf("Search benchmark failed: %v", err)
		}

		fmt.Printf("\nSearch Benchmark Results:\n")
		for method, result := range searchResults {
			fmt.Printf("  %s:\n", method)
			fmt.Printf("    Avg Duration: %s\n", result.AvgDuration)
			if result.TotalQueries > 0 {
				fmt.Printf("    Success Rate: %.2f%%\n",
					float64(result.SuccessQueries)/float64(result.TotalQueries)*100)
			} else {
				fmt.Printf("    Success Rate: n/a\n")
			}
		}
	}

	// Generate reports
	if stressResult != nil || searchResults != nil {
		fmt.Println("\n=== Generating Reports ===")
		reporter := report.NewReporter(cfg.Report.OutputDir)

		if err := reporter.GenerateReport(stressResult, searchResults); err != nil {
			log.Fatalf("Failed to generate report: %v", err)
		}

		if cfg.Report.GenerateChart {
			chartGen := report.NewChartGenerator(cfg.Report.OutputDir)
			if err := chartGen.GenerateCharts(stressResult, searchResults); err != nil {
				log.Fatalf("Failed to generate charts: %v", err)
			}
		}

		fmt.Printf("Reports generated in: %s\n", cfg.Report.OutputDir)
	}

	fmt.Println("\n✅ Benchmark completed successfully!")
}

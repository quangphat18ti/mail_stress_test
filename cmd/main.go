package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"mail-stress-test/benchmark"
	"mail-stress-test/config"
	"mail-stress-test/database"
	"mail-stress-test/generator"
	"mail-stress-test/handler"
	"mail-stress-test/monitoring"
	"mail-stress-test/report"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func main() {
	configPath := flag.String("config", "", "Path to config file")
	seedData := flag.Bool("seed", false, "Seed initial data")
	runStress := flag.Bool("stress", true, "Run stress test")
	runBenchmark := flag.Bool("benchmark", true, "Run search benchmark")
	useAPI := flag.Bool("use-api", false, "Use API handler instead of direct DB")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override use_api from flag if provided
	if *useAPI {
		cfg.StressTest.UseAPI = true
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

	// Prepare user IDs for data generator
	userIDs := make([]string, cfg.StressTest.NumUsers)
	for i := 0; i < cfg.StressTest.NumUsers; i++ {
		userIDs[i] = primitive.NewObjectID().Hex()
	}

	// Create data generator
	dataGen := generator.NewDataGenerator(userIDs)

	// Create mail handler based on configuration
	var mailHandler handler.MailHandler
	if cfg.StressTest.UseAPI {
		fmt.Println("Using API Handler (endpoint: " + cfg.StressTest.APIEndpoint + ")")
		mailHandler = handler.NewAPIHandler(cfg.StressTest.APIEndpoint)
	} else {
		fmt.Println("Using Direct DB Handler")
		mailHandler = handler.NewDBHandler(db)
	}

	// Seed data if requested
	if *seedData {
		fmt.Println("\n=== Seeding Test Data ===")
		fmt.Printf("Creating mails for %d users...\n", cfg.StressTest.NumUsers)

		// Seed some initial mails
		for i := 0; i < cfg.StressTest.NumMailsPerUser; i++ {
			req := dataGen.GenerateCreateMailRequest("")
			if err := mailHandler.CreateMail(ctx, req); err != nil {
				log.Printf("Warning: Failed to seed mail %d: %v", i, err)
				continue
			}

			if i%100 == 0 && i > 0 {
				fmt.Printf("  Created %d/%d mails\n", i, cfg.StressTest.NumMailsPerUser)
			}
		}
		fmt.Println("Data seeding completed!")
	}

	var stressResult *benchmark.StressTestResult
	var searchResults map[string]*benchmark.SearchBenchmarkResult
	var monitoringReport *monitoring.MonitoringReport

	// Setup monitoring if enabled
	var monitoringMgr *monitoring.MonitoringManager
	if cfg.Monitoring.Enabled {
		fmt.Println("\n=== Setting up Monitoring ===")
		monitoringConfig := monitoring.MonitoringManagerConfig{
			EnablePrometheus:    cfg.Monitoring.PrometheusURL != "",
			PrometheusURL:       cfg.Monitoring.PrometheusURL,
			EnableSystemMonitor: cfg.Monitoring.EnableSystemMonitor,
			SystemConfig: monitoring.MonitoringConfig{
				TargetHost:     cfg.Monitoring.TargetHost,
				IsDocker:       cfg.Monitoring.IsDocker,
				ContainerID:    cfg.Monitoring.ContainerID,
				ScrapeInterval: cfg.Monitoring.ScrapeInterval,
			},
			ScrapeInterval:    cfg.Monitoring.ScrapeInterval,
			OutputDir:         cfg.Report.OutputDir,
			EnableRealtimeLog: cfg.Monitoring.EnableRealtimeLog,
		}
		monitoringMgr = monitoring.NewMonitoringManager(monitoringConfig)

		if err := monitoringMgr.StartMonitoring(ctx); err != nil {
			log.Printf("Warning: Failed to start monitoring: %v", err)
		}

		// Give monitoring a moment to start
		time.Sleep(2 * time.Second)
	}

	// Run stress test
	if *runStress {
		fmt.Println("\n=== Running Stress Test ===")
		stressTest := benchmark.NewStressTest(cfg, dataGen, mailHandler)
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

		// Print operation breakdown
		fmt.Println("\n  Operation Breakdown:")
		for op, stats := range stressResult.OperationStats {
			fmt.Printf("    %s: Count=%d, Avg=%s, Errors=%d\n",
				op, stats.Count, stats.AvgDuration, stats.Errors)
		}
	}

	// Run search benchmark
	if *runBenchmark {
		searchBench := benchmark.NewSearchBenchmark(cfg, db, dataGen)
		searchResults, err = searchBench.Run(ctx)
		if err != nil {
			log.Fatalf("Search benchmark failed: %v", err)
		}

		// Print comparison report
		comparisonReport := searchBench.GenerateComparisonReport(searchResults)
		fmt.Println(comparisonReport)
	}

	// Stop monitoring and get report
	if monitoringMgr != nil {
		fmt.Println("\n=== Collecting Monitoring Results ===")
		monitoringReport, err = monitoringMgr.StopMonitoring(ctx)
		if err != nil {
			log.Printf("Warning: Failed to stop monitoring: %v", err)
		} else {
			monitoringMgr.PrintSummary(monitoringReport)
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

	if monitoringReport != nil {
		fmt.Println("\n💡 Tip: Check monitoring report for detailed performance insights!")
	}
}

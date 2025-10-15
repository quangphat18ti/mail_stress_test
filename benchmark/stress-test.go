package benchmark

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"mail-stress-test/config"
	"mail-stress-test/generator"
	"mail-stress-test/handler"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StressTestResult struct {
	TotalRequests     int64                      `json:"total_requests"`
	SuccessRequests   int64                      `json:"success_requests"`
	FailedRequests    int64                      `json:"failed_requests"`
	TotalDuration     time.Duration              `json:"total_duration"`
	AvgResponseTime   time.Duration              `json:"avg_response_time"`
	MinResponseTime   time.Duration              `json:"min_response_time"`
	MaxResponseTime   time.Duration              `json:"max_response_time"`
	RequestsPerSecond float64                    `json:"requests_per_second"`
	ErrorRate         float64                    `json:"error_rate"`
	OperationStats    map[string]*OperationStats `json:"operation_stats"`
}

type OperationStats struct {
	Count       int64         `json:"count"`
	AvgDuration time.Duration `json:"avg_duration"`
	MinDuration time.Duration `json:"min_duration"`
	MaxDuration time.Duration `json:"max_duration"`
	Errors      int64         `json:"errors"`
}

type StressTest struct {
	config    *config.Config
	generator *generator.DataGenerator
	handler   handler.MailHandler
}

// NewStressTest creates a new stress test with the given dependencies
func NewStressTest(cfg *config.Config, gen *generator.DataGenerator, handler handler.MailHandler) *StressTest {
	return &StressTest{
		config:    cfg,
		generator: gen,
		handler:   handler,
	}
}

func (st *StressTest) Run(ctx context.Context) (*StressTestResult, error) {
	result := &StressTestResult{
		MinResponseTime: time.Hour,
		OperationStats: map[string]*OperationStats{
			"create": {MinDuration: time.Hour},
			"list":   {MinDuration: time.Hour},
			"search": {MinDuration: time.Hour},
		},
	}

	var totalDuration int64
	var wg sync.WaitGroup

	startTime := time.Now()
	endTime := startTime.Add(st.config.StressTest.Duration)

	// Rate limiter
	rateLimiter := time.NewTicker(time.Second / time.Duration(st.config.StressTest.RequestRate))
	defer rateLimiter.Stop()

	// Worker pool
	for i := 0; i < st.config.StressTest.ConcurrentWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			st.worker(ctx, endTime, rateLimiter, result, &totalDuration)
		}()
	}

	wg.Wait()

	// Calculate final stats
	result.TotalDuration = time.Since(startTime)
	if result.TotalRequests > 0 {
		result.AvgResponseTime = time.Duration(totalDuration / result.TotalRequests)
		result.RequestsPerSecond = float64(result.TotalRequests) / result.TotalDuration.Seconds()
		result.ErrorRate = float64(result.FailedRequests) / float64(result.TotalRequests) * 100
	}

	// Calculate operation stats
	for _, stats := range result.OperationStats {
		if stats.Count > 0 {
			stats.AvgDuration = time.Duration(int64(stats.AvgDuration) / stats.Count)
		}
	}

	return result, nil
}

func (st *StressTest) worker(ctx context.Context, endTime time.Time, rateLimiter *time.Ticker, result *StressTestResult, totalDuration *int64) {
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return
		case <-rateLimiter.C:
			operation := st.selectOperation()
			start := time.Now()

			err := st.executeOperation(ctx, operation)
			duration := time.Since(start)

			atomic.AddInt64(totalDuration, int64(duration))
			atomic.AddInt64(&result.TotalRequests, 1)

			if err != nil {
				atomic.AddInt64(&result.FailedRequests, 1)
				st.updateOperationStats(result, operation, duration, true)
			} else {
				atomic.AddInt64(&result.SuccessRequests, 1)
				st.updateOperationStats(result, operation, duration, false)
			}

			// Update min/max
			if duration < result.MinResponseTime {
				result.MinResponseTime = duration
			}
			if duration > result.MaxResponseTime {
				result.MaxResponseTime = duration
			}
		}
	}
}

func (st *StressTest) selectOperation() string {
	weights := st.config.StressTest.Operations
	total := weights.CreateMailWeight + weights.ListMailWeight + weights.SearchWeight
	r := rand.Intn(total)

	if r < weights.CreateMailWeight {
		return "create"
	} else if r < weights.CreateMailWeight+weights.ListMailWeight {
		return "list"
	}
	return "search"
}

func (st *StressTest) executeOperation(ctx context.Context, operation string) error {
	switch operation {
	case "create":
		return st.createMail(ctx)
	case "list":
		return st.listMails(ctx)
	case "search":
		return st.searchMails(ctx)
	default:
		return fmt.Errorf("unknown operation: %s", operation)
	}
}

func (st *StressTest) createMail(ctx context.Context) error {
	// Generate mail request with optional reply
	var replyToID string
	if rand.Float32() < 0.3 { // 30% chance of being a reply
		replyToID = primitive.NewObjectID().Hex() // In real scenario, you'd pick from existing mails
	}

	req := st.generator.GenerateCreateMailRequest(replyToID)
	return st.handler.CreateMail(ctx, req)
}

func (st *StressTest) listMails(ctx context.Context) error {
	req := st.generator.GenerateListMailsRequest()
	_, err := st.handler.ListMails(ctx, req)
	return err
}

func (st *StressTest) searchMails(ctx context.Context) error {
	req := st.generator.GenerateSearchMailsRequest()
	_, err := st.handler.SearchMails(ctx, req)
	return err
}

func (st *StressTest) updateOperationStats(result *StressTestResult, operation string, duration time.Duration, isError bool) {
	stats := result.OperationStats[operation]

	atomic.AddInt64(&stats.Count, 1)
	atomic.AddInt64((*int64)(&stats.AvgDuration), int64(duration))

	if isError {
		atomic.AddInt64(&stats.Errors, 1)
	}

	if duration < stats.MinDuration {
		stats.MinDuration = duration
	}
	if duration > stats.MaxDuration {
		stats.MaxDuration = duration
	}
}

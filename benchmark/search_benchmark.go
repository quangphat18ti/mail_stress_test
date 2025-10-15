package benchmark

import (
	"context"
	"time"

	"mail-stress-test/config"
	"mail-stress-test/database"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type SearchBenchmarkResult struct {
	Method         string        `json:"method"`
	AvgDuration    time.Duration `json:"avg_duration"`
	MinDuration    time.Duration `json:"min_duration"`
	MaxDuration    time.Duration `json:"max_duration"`
	TotalQueries   int           `json:"total_queries"`
	SuccessQueries int           `json:"success_queries"`
	FailedQueries  int           `json:"failed_queries"`
}

type SearchBenchmark struct {
	config *config.Config
	db     *database.MongoDB
}

func NewSearchBenchmark(cfg *config.Config, db *database.MongoDB) *SearchBenchmark {
	return &SearchBenchmark{
		config: cfg,
		db:     db,
	}
}

func (sb *SearchBenchmark) Run(ctx context.Context) (map[string]*SearchBenchmarkResult, error) {
	results := make(map[string]*SearchBenchmarkResult)

	for _, method := range sb.config.Benchmark.SearchMethods {
		result := &SearchBenchmarkResult{
			Method:      method,
			MinDuration: time.Hour,
		}

		for i := 0; i < sb.config.Benchmark.Iterations; i++ {
			duration, err := sb.executeSearch(ctx, method)

			result.TotalQueries++
			if err != nil {
				result.FailedQueries++
				continue
			}

			result.SuccessQueries++
			result.AvgDuration += duration

			if duration < result.MinDuration {
				result.MinDuration = duration
			}
			if duration > result.MaxDuration {
				result.MaxDuration = duration
			}
		}

		if result.SuccessQueries > 0 {
			result.AvgDuration = result.AvgDuration / time.Duration(result.SuccessQueries)
		}

		results[method] = result
	}

	return results, nil
}

func (sb *SearchBenchmark) executeSearch(ctx context.Context, method string) (time.Duration, error) {
	collection := sb.db.Database.Collection("mails")
	searchTerm := "Meeting"

	start := time.Now()

	var cursor *mongo.Cursor
	var err error

	switch method {
	case "text_search":
		cursor, err = collection.Find(ctx, bson.M{
			"$text": bson.M{"$search": searchTerm},
		})
	case "regex":
		cursor, err = collection.Find(ctx, bson.M{
			"$or": []bson.M{
				{"subject": bson.M{"$regex": searchTerm, "$options": "i"}},
				{"content": bson.M{"$regex": searchTerm, "$options": "i"}},
			},
		})
	case "aggregation":
		pipeline := []bson.M{
			{"$match": bson.M{
				"$or": []bson.M{
					{"subject": bson.M{"$regex": searchTerm, "$options": "i"}},
					{"content": bson.M{"$regex": searchTerm, "$options": "i"}},
				},
			}},
			{"$limit": 100},
		}
		cursor, err = collection.Aggregate(ctx, pipeline)
	default:
		// Treat unknown method as no-op
		return 0, nil
	}

	duration := time.Since(start)

	if err != nil {
		return duration, err
	}
	if cursor != nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			// consume
		}
		if err := cursor.Err(); err != nil {
			return duration, err
		}
	}

	return duration, nil
}

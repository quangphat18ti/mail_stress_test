# Mail System Stress Test & Benchmark Tool

## Tổng quan

Hệ thống stress test và benchmark cho mail system với Golang và MongoDB. Hỗ trợ đánh giá hiệu năng, so sánh các phương pháp filter/search, và xuất report chi tiết.

## Cấu trúc Project

```
mail-stress-test/
├── cmd/
│   └── main.go                 # Entry point
├── config/
│   ├── config.go              # Configuration loader
│   └── default.yaml           # Default config
├── models/
│   └── mail.go                # Data models
├── database/
│   └── mongo.go               # MongoDB connection
├── generator/
│   ├── data_generator.go      # Tạo dữ liệu test
│   └── api_client.go          # API client (optional)
├── benchmark/
│   ├── stress_test.go         # Stress test runner
│   ├── search_benchmark.go    # Search benchmark
│   └── metrics.go             # Metrics collector
├── report/
│   ├── reporter.go            # Report generator
│   └── chart.go               # Chart generator
├── go.mod
├── go.sum
└── README.md
```

## 1. Configuration (config/config.go)

```go
package config

import (
    "os"
    "time"
    "gopkg.in/yaml.v3"
)

type Config struct {
    MongoDB    MongoDBConfig    `yaml:"mongodb"`
    StressTest StressTestConfig `yaml:"stress_test"`
    Benchmark  BenchmarkConfig  `yaml:"benchmark"`
    Report     ReportConfig     `yaml:"report"`
}

type MongoDBConfig struct {
    URI      string `yaml:"uri"`
    Database string `yaml:"database"`
    Timeout  int    `yaml:"timeout"` // seconds
}

type StressTestConfig struct {
    NumUsers            int           `yaml:"num_users"`
    NumMailsPerUser     int           `yaml:"num_mails_per_user"`
    ConcurrentWorkers   int           `yaml:"concurrent_workers"`
    RequestRate         int           `yaml:"request_rate"` // requests per second
    Duration            time.Duration `yaml:"duration"`     // test duration
    UseAPI              bool          `yaml:"use_api"`
    APIEndpoint         string        `yaml:"api_endpoint"`
    Operations          Operations    `yaml:"operations"`
}

type Operations struct {
    CreateMailWeight int `yaml:"create_mail_weight"` // 0-100
    ListMailWeight   int `yaml:"list_mail_weight"`   // 0-100
    SearchWeight     int `yaml:"search_weight"`      // 0-100
}

type BenchmarkConfig struct {
    SearchMethods []string `yaml:"search_methods"` // ["text_search", "regex", "aggregation"]
    SampleSize    int      `yaml:"sample_size"`
    Iterations    int      `yaml:"iterations"`
}

type ReportConfig struct {
    OutputDir     string `yaml:"output_dir"`
    GenerateChart bool   `yaml:"generate_chart"`
    JSONReport    bool   `yaml:"json_report"`
}

func LoadConfig(path string) (*Config, error) {
    // Load from ENV first
    config := &Config{}
    
    if path == "" {
        path = os.Getenv("CONFIG_PATH")
        if path == "" {
            path = "config/default.yaml"
        }
    }
    
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    
    if err := yaml.Unmarshal(data, config); err != nil {
        return nil, err
    }
    
    // Override with ENV variables
    config.overrideFromEnv()
    
    return config, nil
}

func (c *Config) overrideFromEnv() {
    if uri := os.Getenv("MONGO_URI"); uri != "" {
        c.MongoDB.URI = uri
    }
    if db := os.Getenv("MONGO_DATABASE"); db != "" {
        c.MongoDB.Database = db
    }
}

func DefaultConfig() *Config {
    return &Config{
        MongoDB: MongoDBConfig{
            URI:      "mongodb://localhost:27017",
            Database: "mail_stress_test",
            Timeout:  10,
        },
        StressTest: StressTestConfig{
            NumUsers:          100,
            NumMailsPerUser:   1000,
            ConcurrentWorkers: 50,
            RequestRate:       100,
            Duration:          5 * time.Minute,
            UseAPI:            false,
            APIEndpoint:       "http://localhost:8080",
            Operations: Operations{
                CreateMailWeight: 30,
                ListMailWeight:   50,
                SearchWeight:     20,
            },
        },
        Benchmark: BenchmarkConfig{
            SearchMethods: []string{"text_search", "regex", "aggregation"},
            SampleSize:    1000,
            Iterations:    100,
        },
        Report: ReportConfig{
            OutputDir:     "./reports",
            GenerateChart: true,
            JSONReport:    true,
        },
    }
}
```

## 2. Default Config File (config/default.yaml)

```yaml
mongodb:
  uri: "mongodb://localhost:27017"
  database: "mail_stress_test"
  timeout: 10

stress_test:
  num_users: 100
  num_mails_per_user: 1000
  concurrent_workers: 50
  request_rate: 100
  duration: 5m
  use_api: false
  api_endpoint: "http://localhost:8080"
  operations:
    create_mail_weight: 30
    list_mail_weight: 50
    search_weight: 20

benchmark:
  search_methods:
    - "text_search"
    - "regex"
    - "aggregation"
  sample_size: 1000
  iterations: 100

report:
  output_dir: "./reports"
  generate_chart: true
  json_report: true
```

## 3. Models (models/mail.go)

```go
package models

import (
    "time"
    "go.mongodb.org/mongo-driver/bson/primitive"
)

type Mail struct {
    ID        primitive.ObjectID `bson:"_id,omitempty"`
    UserID    string             `bson:"userId"`
    ThreadID  string             `bson:"threadId"`
    Subject   string             `bson:"subject"`
    Content   string             `bson:"content"`
    CreatedAt time.Time          `bson:"createdAt"`
}

type Thread struct {
    ID         primitive.ObjectID `bson:"_id,omitempty"`
    ThreadID   string             `bson:"thread_id"`
    Mails      []ThreadMail       `bson:"mails"`
    TotalMails int                `bson:"total_mails"`
    UserID     primitive.ObjectID `bson:"user_id"`
}

type ThreadMail struct {
    From    string   `bson:"from"`
    MsgID   string   `bson:"msg_id"`
    Subject string   `bson:"subject"`
    Content string   `bson:"content"`
    Cc      []string `bson:"cc"`
    To      []string `bson:"to"`
    Bcc     []string `bson:"bcc"`
    Type    int      `bson:"type"`
}
```

## 4. Database Connection (database/mongo.go)

```go
package database

import (
    "context"
    "time"
    
    "go.mongodb.org/mongo-driver/mongo"
    "go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDB struct {
    Client   *mongo.Client
    Database *mongo.Database
}

func NewMongoDB(uri, dbName string, timeout int) (*MongoDB, error) {
    ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
    defer cancel()
    
    client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
    if err != nil {
        return nil, err
    }
    
    if err := client.Ping(ctx, nil); err != nil {
        return nil, err
    }
    
    return &MongoDB{
        Client:   client,
        Database: client.Database(dbName),
    }, nil
}

func (m *MongoDB) Close() error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    return m.Client.Disconnect(ctx)
}

func (m *MongoDB) CreateIndexes(ctx context.Context) error {
    // Mail collection indexes
    mailCollection := m.Database.Collection("mails")
    _, err := mailCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
        {Keys: map[string]interface{}{"userId": 1}},
        {Keys: map[string]interface{}{"threadId": 1}},
        {Keys: map[string]interface{}{"createdAt": -1}},
        {Keys: map[string]interface{}{"subject": "text", "content": "text"}},
    })
    if err != nil {
        return err
    }
    
    // Thread collection indexes
    threadCollection := m.Database.Collection("threads")
    _, err = threadCollection.Indexes().CreateMany(ctx, []mongo.IndexModel{
        {Keys: map[string]interface{}{"user_id": 1, "thread_id": 1}},
        {Keys: map[string]interface{}{"user_id": 1}},
    })
    
    return err
}
```

## 5. Data Generator (generator/data_generator.go)

```go
package generator

import (
    "context"
    "fmt"
    "math/rand"
    "time"
    
    "go.mongodb.org/mongo-driver/bson/primitive"
    "mail-stress-test/models"
    "mail-stress-test/database"
)

type DataGenerator struct {
    db *database.MongoDB
}

func NewDataGenerator(db *database.MongoDB) *DataGenerator {
    return &DataGenerator{db: db}
}

var subjects = []string{
    "Meeting Update", "Project Status", "Quick Question",
    "Follow Up", "Important Notice", "Weekly Report",
    "Team Sync", "Budget Review", "Action Required",
}

var contentTemplates = []string{
    "Hi team, I wanted to follow up on our discussion about %s. Please review and provide feedback.",
    "This is regarding the %s project. We need to discuss the next steps.",
    "Can you please take a look at %s? Your input would be valuable.",
    "Update on %s: We've made significant progress this week.",
    "Reminder about %s. Please complete by end of day.",
}

func (g *DataGenerator) GenerateMail(userID, threadID string) *models.Mail {
    return &models.Mail{
        ID:        primitive.NewObjectID(),
        UserID:    userID,
        ThreadID:  threadID,
        Subject:   subjects[rand.Intn(len(subjects))],
        Content:   fmt.Sprintf(contentTemplates[rand.Intn(len(contentTemplates))], subjects[rand.Intn(len(subjects))]),
        CreatedAt: time.Now().Add(-time.Duration(rand.Intn(365*24)) * time.Hour),
    }
}

func (g *DataGenerator) CreateMailWithThread(ctx context.Context, senderID string, recipients []string) error {
    threadID := primitive.NewObjectID().Hex()
    
    // Create mail for sender
    senderMail := g.GenerateMail(senderID, threadID)
    
    mailCollection := g.db.Database.Collection("mails")
    threadCollection := g.db.Database.Collection("threads")
    
    // Insert sender's mail
    if _, err := mailCollection.InsertOne(ctx, senderMail); err != nil {
        return err
    }
    
    // Create thread mail metadata
    threadMail := models.ThreadMail{
        From:    senderID,
        MsgID:   senderMail.ID.Hex(),
        Subject: senderMail.Subject,
        Content: senderMail.Content,
        To:      recipients,
        Type:    1, // sent
    }
    
    // Update sender's thread
    userIDObj, _ := primitive.ObjectIDFromHex(senderID)
    g.updateThread(ctx, threadCollection, userIDObj, threadID, threadMail)
    
    // Create mails for recipients
    for _, recipientID := range recipients {
        recipientMail := &models.Mail{
            ID:        primitive.NewObjectID(),
            UserID:    recipientID,
            ThreadID:  threadID,
            Subject:   senderMail.Subject,
            Content:   senderMail.Content,
            CreatedAt: senderMail.CreatedAt,
        }
        
        if _, err := mailCollection.InsertOne(ctx, recipientMail); err != nil {
            return err
        }
        
        // Update recipient's thread
        recipientThreadMail := threadMail
        recipientThreadMail.Type = 0 // received
        
        userIDObj, _ := primitive.ObjectIDFromHex(recipientID)
        g.updateThread(ctx, threadCollection, userIDObj, threadID, recipientThreadMail)
    }
    
    return nil
}

func (g *DataGenerator) updateThread(ctx context.Context, collection *mongo.Collection, userID primitive.ObjectID, threadID string, threadMail models.ThreadMail) error {
    filter := map[string]interface{}{
        "user_id":   userID,
        "thread_id": threadID,
    }
    
    update := map[string]interface{}{
        "$push": map[string]interface{}{
            "mails": threadMail,
        },
        "$inc": map[string]interface{}{
            "total_mails": 1,
        },
        "$setOnInsert": map[string]interface{}{
            "user_id":   userID,
            "thread_id": threadID,
        },
    }
    
    opts := options.Update().SetUpsert(true)
    _, err := collection.UpdateOne(ctx, filter, update, opts)
    return err
}

func (g *DataGenerator) SeedData(ctx context.Context, numUsers, mailsPerUser int) error {
    userIDs := make([]string, numUsers)
    for i := 0; i < numUsers; i++ {
        userIDs[i] = primitive.NewObjectID().Hex()
    }
    
    for i := 0; i < mailsPerUser; i++ {
        senderIdx := rand.Intn(numUsers)
        numRecipients := rand.Intn(5) + 1
        
        recipients := make([]string, 0, numRecipients)
        for j := 0; j < numRecipients; j++ {
            recipientIdx := rand.Intn(numUsers)
            if recipientIdx != senderIdx {
                recipients = append(recipients, userIDs[recipientIdx])
            }
        }
        
        if len(recipients) > 0 {
            if err := g.CreateMailWithThread(ctx, userIDs[senderIdx], recipients); err != nil {
                return err
            }
        }
        
        if i%100 == 0 {
            fmt.Printf("Created %d/%d mails\n", i, mailsPerUser)
        }
    }
    
    return nil
}
```

## 6. Stress Test Runner (benchmark/stress_test.go)

```go
package benchmark

import (
    "context"
    "fmt"
    "math/rand"
    "sync"
    "sync/atomic"
    "time"
    
    "mail-stress-test/config"
    "mail-stress-test/database"
    "mail-stress-test/generator"
    "go.mongodb.org/mongo-driver/bson"
)

type StressTestResult struct {
    TotalRequests     int64         `json:"total_requests"`
    SuccessRequests   int64         `json:"success_requests"`
    FailedRequests    int64         `json:"failed_requests"`
    TotalDuration     time.Duration `json:"total_duration"`
    AvgResponseTime   time.Duration `json:"avg_response_time"`
    MinResponseTime   time.Duration `json:"min_response_time"`
    MaxResponseTime   time.Duration `json:"max_response_time"`
    RequestsPerSecond float64       `json:"requests_per_second"`
    ErrorRate         float64       `json:"error_rate"`
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
    db        *database.MongoDB
    generator *generator.DataGenerator
    userIDs   []string
}

func NewStressTest(cfg *config.Config, db *database.MongoDB) *StressTest {
    return &StressTest{
        config:    cfg,
        db:        db,
        generator: generator.NewDataGenerator(db),
    }
}

func (st *StressTest) Run(ctx context.Context) (*StressTestResult, error) {
    // Prepare user IDs
    st.prepareUsers(ctx)
    
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
    senderID := st.userIDs[rand.Intn(len(st.userIDs))]
    numRecipients := rand.Intn(3) + 1
    
    recipients := make([]string, 0, numRecipients)
    for i := 0; i < numRecipients; i++ {
        recipients = append(recipients, st.userIDs[rand.Intn(len(st.userIDs))])
    }
    
    return st.generator.CreateMailWithThread(ctx, senderID, recipients)
}

func (st *StressTest) listMails(ctx context.Context) error {
    userID := st.userIDs[rand.Intn(len(st.userIDs))]
    collection := st.db.Database.Collection("mails")
    
    cursor, err := collection.Find(ctx, bson.M{"userId": userID})
    if err != nil {
        return err
    }
    defer cursor.Close(ctx)
    
    // Consume results
    for cursor.Next(ctx) {
        // Just iterate
    }
    
    return cursor.Err()
}

func (st *StressTest) searchMails(ctx context.Context) error {
    userID := st.userIDs[rand.Intn(len(st.userIDs))]
    searchTerm := generator.subjects[rand.Intn(len(generator.subjects))]
    
    collection := st.db.Database.Collection("mails")
    
    filter := bson.M{
        "userId": userID,
        "$or": []bson.M{
            {"subject": bson.M{"$regex": searchTerm, "$options": "i"}},
            {"content": bson.M{"$regex": searchTerm, "$options": "i"}},
        },
    }
    
    cursor, err := collection.Find(ctx, filter)
    if err != nil {
        return err
    }
    defer cursor.Close(ctx)
    
    for cursor.Next(ctx) {
        // Just iterate
    }
    
    return cursor.Err()
}

func (st *StressTest) prepareUsers(ctx context.Context) {
    st.userIDs = make([]string, st.config.StressTest.NumUsers)
    for i := 0; i < st.config.StressTest.NumUsers; i++ {
        st.userIDs[i] = fmt.Sprintf("user_%d", i)
    }
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
```

## 7. Search Benchmark (benchmark/search_benchmark.go)

```go
package benchmark

import (
    "context"
    "time"
    
    "go.mongodb.org/mongo-driver/bson"
    "mail-stress-test/config"
    "mail-stress-test/database"
)

type SearchBenchmarkResult struct {
    Method          string        `json:"method"`
    AvgDuration     time.Duration `json:"avg_duration"`
    MinDuration     time.Duration `json:"min_duration"`
    MaxDuration     time.Duration `json:"max_duration"`
    TotalQueries    int           `json:"total_queries"`
    SuccessQueries  int           `json:"success_queries"`
    FailedQueries   int           `json:"failed_queries"`
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
    
    var cursor interface{}
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
    }
    
    duration := time.Since(start)
    
    if err != nil {
        return duration, err
    }
    
    // Close cursor if needed
    // Implementation depends on cursor type
    
    return duration, nil
}
```

## 8. Report Generator (report/reporter.go)

```go
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
    Timestamp          time.Time                              `json:"timestamp"`
    StressTestResult   *benchmark.StressTestResult            `json:"stress_test_result"`
    SearchBenchmark    map[string]*benchmark.SearchBenchmarkResult `json:"search_benchmark"`
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
    
    // Search Benchmark Results
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
    
    return nil
}
```

## 9. Chart Generator (report/chart.go)

```go
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
        html += `'` + op + `', `
    }
    
    html += `],
                datasets: [{
                    label: 'Average Duration (ms)',
                    data: [`
    
    // Add operation data
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
        html += `'` + method + `', `
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
```

## 10. Main Entry Point (cmd/main.go)

```go
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
        fmt.Printf("  Success: %d (%.2f%%)\n", stressResult.SuccessRequests, 
            float64(stressResult.SuccessRequests)/float64(stressResult.TotalRequests)*100)
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
            fmt.Printf("    Success Rate: %.2f%%\n", 
                float64(result.SuccessQueries)/float64(result.TotalQueries)*100)
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
```

## 11. API Client (generator/api_client.go)

```go
package generator

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type APIClient struct {
    baseURL    string
    httpClient *http.Client
}

func NewAPIClient(baseURL string) *APIClient {
    return &APIClient{
        baseURL: baseURL,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

type CreateMailRequest struct {
    SenderID   string   `json:"sender_id"`
    Recipients []string `json:"recipients"`
    Subject    string   `json:"subject"`
    Content    string   `json:"content"`
}

type ListMailsRequest struct {
    UserID string `json:"user_id"`
    Limit  int    `json:"limit"`
    Offset int    `json:"offset"`
}

type SearchMailsRequest struct {
    UserID     string `json:"user_id"`
    SearchTerm string `json:"search_term"`
}

func (c *APIClient) CreateMail(ctx context.Context, req *CreateMailRequest) error {
    body, err := json.Marshal(req)
    if err != nil {
        return err
    }
    
    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/mails", bytes.NewBuffer(body))
    if err != nil {
        return err
    }
    
    httpReq.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
        return fmt.Errorf("API error: status code %d", resp.StatusCode)
    }
    
    return nil
}

func (c *APIClient) ListMails(ctx context.Context, req *ListMailsRequest) error {
    body, err := json.Marshal(req)
    if err != nil {
        return err
    }
    
    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/mails/list", bytes.NewBuffer(body))
    if err != nil {
        return err
    }
    
    httpReq.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("API error: status code %d", resp.StatusCode)
    }
    
    return nil
}

func (c *APIClient) SearchMails(ctx context.Context, req *SearchMailsRequest) error {
    body, err := json.Marshal(req)
    if err != nil {
        return err
    }
    
    httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/mails/search", bytes.NewBuffer(body))
    if err != nil {
        return err
    }
    
    httpReq.Header.Set("Content-Type", "application/json")
    
    resp, err := c.httpClient.Do(httpReq)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("API error: status code %d", resp.StatusCode)
    }
    
    return nil
}
```

## 12. Go Modules (go.mod)

```go
module mail-stress-test

go 1.21

require (
    go.mongodb.org/mongo-driver v1.13.1
    gopkg.in/yaml.v3 v3.0.1
)

require (
    github.com/golang/snappy v0.0.4 // indirect
    github.com/klauspost/compress v1.17.4 // indirect
    github.com/montanaflynn/stats v0.7.1 // indirect
    github.com/xdg-go/pbkdf2 v1.0.0 // indirect
    github.com/xdg-go/scram v1.1.2 // indirect
    github.com/xdg-go/stringprep v1.0.4 // indirect
    github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a // indirect
    golang.org/x/crypto v0.17.0 // indirect
    golang.org/x/sync v0.5.0 // indirect
    golang.org/x/text v0.14.0 // indirect
)
```

## 13. Installation & Usage

### Prerequisites
```bash
# Install Go 1.21+ (for local development)
# Install MongoDB 5.0+ (for local development)
# Install Docker & Docker Compose (for containerized setup)
```

### Setup
```bash
# Clone hoặc tạo project
mkdir mail-stress-test
cd mail-stress-test

# Initialize Go module
go mod init mail-stress-test

# Download dependencies
go mod download

# Build
go build -o mail-stress-test ./cmd/main.go
```

### NEW: Quick start with run.sh

Script trợ giúp `run.sh` giúp tự động build và chạy các tác vụ phổ biến. Trên macOS/Linux, cấp quyền chạy và sử dụng:

```bash
chmod +x ./run.sh

# 1) Cài deps và chuẩn bị thư mục báo cáo
./run.sh setup

# 2) Build binary
./run.sh build

# 3) Seed dữ liệu ban đầu (đọc config/default.yaml)
./run.sh seed

# 4) Chạy stress test (không chạy benchmark)
./run.sh stress

# 5) Chạy benchmark tìm kiếm (không chạy stress test)
./run.sh bench

# 6) Chạy full (stress + benchmark) – giống mặc định của chương trình
./run.sh all

# 7) Mở chart HTML gần nhất
./run.sh open-report

# Tuỳ chọn: chỉ định config khác
./run.sh all -c ./config/default.yaml

# Chạy với Docker Compose (không cần cài Go/MongoDB local)
./run.sh --docker all

# Kết hợp options
./run.sh --docker seed -c ./config/default.yaml

# Truyền thêm tham số trực tiếp cho chương trình Go (sau --)
./run.sh stress -c ./config/default.yaml -- -seed=false
```

Ghi chú:
- Biến môi trường hỗ trợ override: `MONGO_URI`, `MONGO_DATABASE`, `CONFIG_PATH`.
- `run.sh` tự động build nếu chưa có binary. Đường dẫn có khoảng trắng vẫn hoạt động.

### Usage Examples

#### 1. Seed initial data
```bash
./mail-stress-test -seed -config=config/default.yaml
# hoặc dùng run.sh
./run.sh seed -c ./config/default.yaml
# hoặc dùng Docker
./run.sh --docker seed
```

#### 2. Run stress test only
```bash
./mail-stress-test -stress -benchmark=false
# hoặc dùng run.sh
./run.sh stress
# hoặc dùng Docker
./run.sh --docker stress
```

#### 3. Run benchmark only
```bash
./mail-stress-test -stress=false -benchmark
# hoặc dùng run.sh
./run.sh bench
# hoặc dùng Docker
./run.sh --docker bench
```

#### 4. Run full test suite
```bash
./mail-stress-test -config=config/default.yaml
# hoặc dùng run.sh
./run.sh all -c ./config/default.yaml
# hoặc dùng Docker
./run.sh --docker all
```

#### 5. Custom configuration via ENV
```bash
export MONGO_URI="mongodb://localhost:27017"
export MONGO_DATABASE="my_mail_test"
./mail-stress-test
# hoặc dùng run.sh (ENV vẫn có hiệu lực)
./run.sh all
# hoặc dùng Docker (ENV override trong docker-compose.yml)
./run.sh --docker all
```

### NEW: Docker Setup & Usage

Sử dụng Docker để chạy ứng dụng và MongoDB mà không cần cài đặt Go hoặc MongoDB locally.

#### Prerequisites
```bash
# Install Docker & Docker Compose
```

#### Quick Start with Docker Compose
```bash
# 1) Build và start services (MongoDB + app)
docker-compose up --build

# 2) Hoặc chạy background
docker-compose up -d --build

# 3) Xem logs
docker-compose logs -f app

# 4) Stop services
docker-compose down

# 5) Clean up (remove volumes)
docker-compose down -v
```

#### Sử dụng run.sh với Docker
```bash
# Tất cả commands trên đều hỗ trợ --docker flag
./run.sh --docker setup    # Không cần (handled by Dockerfile)
./run.sh --docker build    # Không cần (use docker-compose build)
./run.sh --docker seed
./run.sh --docker stress
./run.sh --docker bench
./run.sh --docker all
./run.sh --docker open-report  # Reports vẫn ở ./reports
```

#### Custom Configuration
- Chỉnh sửa `config/default.yaml` hoặc mount config khác.
- Override environment variables trong `docker-compose.yml` hoặc qua command line.

#### Run Specific Commands
```bash
# Seed data
docker-compose run --rm app ./mail-stress-test -seed -config=config/default.yaml

# Run stress test
docker-compose run --rm app ./mail-stress-test -stress -benchmark=false

# Run benchmark
docker-compose run --rm app ./mail-stress-test -stress=false -benchmark

# Full test
docker-compose run --rm app ./mail-stress-test -config=config/default.yaml
```

#### Notes
- Reports được lưu trong `./reports` (mounted volume).
- MongoDB data persisted trong Docker volume `mongo_data`.
- App container depends on MongoDB, sẽ tự động wait.

### Configuration Examples

#### High Load Test
```yaml
stress_test:
  num_users: 1000
  num_mails_per_user: 10000
  concurrent_workers: 200
  request_rate: 500
  duration: 10m
```

#### Quick Test
```yaml
stress_test:
  num_users: 10
  num_mails_per_user: 100
  concurrent_workers: 10
  request_rate: 50
  duration: 1m
```

## 14. Output Examples

### JSON Report Structure
```json
{
  "timestamp": "2025-10-14T10:30:00Z",
  "stress_test_result": {
    "total_requests": 50000,
    "success_requests": 49500,
    "failed_requests": 500,
    "total_duration": "5m0s",
    "avg_response_time": "45ms",
    "min_response_time": "10ms",
    "max_response_time": "500ms",
    "requests_per_second": 166.67,
    "error_rate": 1.0,
    "operation_stats": {
      "create": {
        "count": 15000,
        "avg_duration": "50ms",
        "min_duration": "20ms",
        "max_duration": "500ms",
        "errors": 150
      },
      "list": {
        "count": 25000,
        "avg_duration": "40ms",
        "min_duration": "10ms",
        "max_duration": "300ms",
        "errors": 250
      },
      "search": {
        "count": 10000,
        "avg_duration": "55ms",
        "min_duration": "15ms",
        "max_duration": "450ms",
        "errors": 100
      }
    }
  },
  "search_benchmark": {
    "text_search": {
      "method": "text_search",
      "avg_duration": "25ms",
      "min_duration": "10ms",
      "max_duration": "100ms",
      "total_queries": 100,
      "success_queries": 100,
      "failed_queries": 0
    },
    "regex": {
      "method": "regex",
      "avg_duration": "85ms",
      "min_duration": "40ms",
      "max_duration": "200ms",
      "total_queries": 100,
      "success_queries": 100,
      "failed_queries": 0
    },
    "aggregation": {
      "method": "aggregation",
      "avg_duration": "65ms",
      "min_duration": "30ms",
      "max_duration": "150ms",
      "total_queries": 100,
      "success_queries": 100,
      "failed_queries": 0
    }
  }
}
```

## 15. Integration Guide

### Tích hợp với API hiện có

Modify `benchmark/stress_test.go` để sử dụng API:

```go
func (st *StressTest) executeOperation(ctx context.Context, operation string) error {
    if st.config.StressTest.UseAPI {
        return st.executeViaAPI(ctx, operation)
    }
    // ... existing direct DB code
}

func (st *StressTest) executeViaAPI(ctx context.Context, operation string) error {
    client := generator.NewAPIClient(st.config.StressTest.APIEndpoint)
    
    switch operation {
    case "create":
        req := &generator.CreateMailRequest{
            SenderID:   st.userIDs[rand.Intn(len(st.userIDs))],
            Recipients: []string{st.userIDs[rand.Intn(len(st.userIDs))]},
            Subject:    "Test Subject",
            Content:    "Test Content",
        }
        return client.CreateMail(ctx, req)
    // ... other operations
    }
    return nil
}
```

### Custom Search Methods

Thêm search method mới trong `benchmark/search_benchmark.go`:

```go
case "custom_method":
    // Implement your custom search logic
    cursor, err = collection.Find(ctx, bson.M{
        // Your custom filter
    })
```

## 16. Performance Tips

### MongoDB Optimization
```javascript
// Recommended indexes
db.mails.createIndex({ "userId": 1, "createdAt": -1 })
db.mails.createIndex({ "threadId": 1 })
db.mails.createIndex({ "subject": "text", "content": "text" })
db.threads.createIndex({ "user_id": 1, "thread_id": 1 }, { unique: true })
```

### Best Practices
1. Start với small dataset để validate
2. Tăng dần load để tìm bottleneck
3. Monitor MongoDB metrics (CPU, Memory, Disk I/O)
4. Sử dụng connection pooling
5. Tune MongoDB cache size

## 17. Troubleshooting

### Common Issues

1. **Too many connections**
   - Giảm `concurrent_workers`
   - Tăng MongoDB connection pool

2. **High error rate**
   - Check MongoDB logs
   - Giảm `request_rate`
   - Verify indexes

3. **Slow response time**
   - Check query patterns
   - Review indexes
   - Consider sharding

## Kết luận

Tool này cung cấp framework hoàn chỉnh để:
- ✅ Stress test mail system với various operations
- ✅ Benchmark multiple search strategies
- ✅ Generate comprehensive reports & visualizations
- ✅ Easy configuration & extensibility
- ✅ Support both direct DB & API testing

Bạn có thể customize theo nhu cầu cụ thể của hệ thống!
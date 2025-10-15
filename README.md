# Mail System Stress Test & Benchmark Tool


## Tổng quan

Hệ thống stress test và benchmark cho mail system với Golang và MongoDB. Hỗ trợ đánh giá hiệu năng, so sánh các phương pháp search, và xuất report chi tiết với Strategy Pattern.

## Tính năng chính

- ✅ **Stress Testing**: Tạo tải với concurrent workers, rate limiting
- ✅ **Search Benchmark**: So sánh 4 strategies (Text Search, Regex, Aggregation, Index Optimized)
- ✅ **Handler Pattern**: DBHandler (direct DB) và APIHandler (REST API)
- ✅ **Threading**: Email threading với ReplyTo field
- ✅ **Reports**: JSON reports và HTML charts với Chart.js
- ✅ **Docker Support**: Docker Compose với MongoDB

## Cấu trúc Project

```
mail-stress-test/
├── cmd/main.go                    # Entry point
├── config/
│   ├── config.go                  # Configuration loader
│   └── default.yaml               # Default config
├── models/mail.go                 # Mail, MailRequest structs
├── database/mongo.go              # MongoDB connection & indexes
├── handler/
│   ├── mail_handler.go            # MailHandler interface
│   ├── db_handler.go              # Direct DB implementation
│   └── api_handler.go             # REST API client
├── generator/request_generator.go # Generate test requests
├── benchmark/
│   ├── stress_test.go             # Load testing
│   └── search_benchmark.go        # Search performance testing
├── search/
│   ├── strategy.go                # SearchStrategy interface
│   ├── text_search.go             # Text index strategy
│   ├── regex_search.go            # Regex pattern matching
│   ├── aggregation_search.go     # Pipeline with scoring
│   └── index_optimized.go         # Compound indexes + collation
├── report/
│   ├── reporter.go                # Report generator
│   └── chart.go                   # HTML chart generator
├── Dockerfile
├── docker-compose.yml
└── run.sh                         # Helper script
```

## Architecture

### Handler Pattern

Hệ thống sử dụng `MailHandler` interface với 2 implementations:

- **DBHandler**: Thao tác trực tiếp với MongoDB, xử lý threading với ReplyTo
- **APIHandler**: Gọi REST API endpoints (Fiber-based backend)

Interface definition (xem `handler/mail_handler.go`):
```go
type MailHandler interface {
    CreateMail(ctx context.Context, req models.MailRequest) (*models.Mail, error)
    ListMails(ctx context.Context, req models.ListMailsRequest) ([]models.Mail, error)
    SearchMails(ctx context.Context, req models.SearchMailsRequest) ([]models.Mail, error)
}
```

### Search Strategy Pattern

4 strategies để so sánh hiệu năng tìm kiếm (xem `search/` folder):

#### 1. Text Search Strategy (`text_search.go`)
- **Phương pháp**: MongoDB Text Index với `$text` operator
- **Ưu điểm**: Nhanh cho full-text search, hỗ trợ stemming
- **Nhược điểm**: Không phân biệt hoa/thường, không hỗ trợ regex
- **Use case**: Search queries đơn giản, tài liệu lớn

#### 2. Regex Search Strategy (`regex_search.go`)
- **Phương pháp**: `$regex` operator với case-insensitive option
- **Ưu điểm**: Linh hoạt, hỗ trợ pattern matching
- **Nhược điểm**: Chậm hơn, không dùng index hiệu quả
- **Use case**: Pattern matching phức tạp, dataset nhỏ

#### 3. Aggregation Search Strategy (`aggregation_search.go`)
- **Phương pháp**: Aggregation Pipeline với relevance scoring
- **Ưu điểm**: Hỗ trợ scoring phức tạp, sort theo relevance
- **Nhược điểm**: Phức tạp, có thể chậm với dataset lớn
- **Use case**: Cần ranking kết quả theo độ liên quan

#### 4. Index Optimized Strategy (`index_optimized.go`)
- **Phương pháp**: Compound indexes với MongoDB Collation
- **Ưu điểm**: Rất nhanh, case-insensitive, tận dụng index
- **Nhược điểm**: Phụ thuộc collation configuration
- **Use case**: Production với yêu cầu performance cao

### Threading Model

Mail threading sử dụng `ReplyTo` field trong `models/mail.go`:

- **New thread**: `ReplyTo` = empty
- **Reply mail**: `ReplyTo` = parent mail ID, kế thừa To/Cc/Subject với prefix "Re:"

## Configuration

File `config/default.yaml`

```yaml
mongodb:
  uri: "mongodb://localhost:27017"
  database: "mail_stress_test"
  timeout: 30

stress_test:
  num_users: 100
  num_mails_per_user: 1000
  concurrent_workers: 50
  request_rate: 100
  duration: 5m
  use_api: false
  api_endpoint: "http://localhost:3000"
  operations:
    create_mail_weight: 50
    list_mail_weight: 30
    search_weight: 20

benchmark:
  search_methods: ["text_search", "regex", "aggregation", "index_optimized"]
  sample_size: 1000
  iterations: 100

report:
  output_dir: "./reports"
  generate_chart: true
  json_report: true
```

### Configuration Options

- **MongoDB**: Connection URI, database name, timeout
- **Stress Test**: Number of users/mails, concurrent workers, request rate, operation weights
- **Benchmark**: Search methods to compare, sample size, iterations
- **Report**: Output directory, enable charts/JSON

## Installation & Usage

### Sử dụng Helper Script (Khuyên dùng)

```bash
# 1. Setup dependencies & build
./run.sh setup

# 2. Seed test data (10,000 mails mặc định)
./run.sh seed

# 3. Run stress test
./run.sh stress

# 4. Run search benchmark
./run.sh bench

# 5. View HTML report
./run.sh open-report

# 6. Chạy tất cả (setup + seed + stress + bench)
./run.sh all

# 7. Clean up
./run.sh clean
```

### Sử dụng Docker

```bash
# Chạy với Docker Compose
./run.sh --docker setup
./run.sh --docker seed
./run.sh --docker stress
./run.sh --docker bench

# Hoặc dùng docker-compose trực tiếp
docker-compose up --build

# Xem logs
docker-compose logs -f app

# Stop & cleanup
docker-compose down -v
```

### Sử dụng trực tiếp (Manual)

```bash
# 1. Install dependencies
go mod download

# 2. Build
go build -o mail-stress-test ./cmd/main.go

# 3. Seed data
./mail-stress-test -seed -config config/default.yaml

# 4. Run stress test với DB handler
./mail-stress-test -stress -config config/default.yaml

# 5. Run stress test với API handler
./mail-stress-test -stress -use-api -config config/default.yaml

# 6. Run search benchmark
./mail-stress-test -benchmark -config config/default.yaml
```

## Command Line Flags

```
-config string     Path to config file (default: "config/default.yaml")
-seed             Seed test data vào database
-stress           Run stress test
-benchmark        Run search benchmark
-use-api          Sử dụng API handler thay vì DB handler
```

## Search Benchmark Metrics

Benchmark tính toán các metrics sau cho từng strategy:

| Metric | Mô tả |
|--------|-------|
| **Setup Duration** | Thời gian tạo indexes |
| **Average Duration** | Trung bình thời gian query |
| **Min/Max Duration** | Thời gian nhanh nhất/chậm nhất |
| **P50 (Median)** | 50% queries nhanh hơn giá trị này |
| **P95** | 95% queries nhanh hơn giá trị này |
| **P99** | 99% queries nhanh hơn giá trị này (tail latency) |
| **Success Rate** | Tỷ lệ queries thành công |

### Sample Output

```
====================================================================================================
SEARCH STRATEGY COMPARISON REPORT
====================================================================================================

Strategy: text_search
  Setup Duration: 1.234s
  Avg Duration: 15.23ms | Min: 8.45ms | Max: 156.78ms
  P50: 12.34ms | P95: 45.67ms | P99: 89.01ms
  Success: 998/1000 (99.80%)

Strategy: index_optimized
  Setup Duration: 2.567s
  Avg Duration: 8.91ms | Min: 3.21ms | Max: 78.45ms
  P50: 7.89ms | P95: 23.45ms | P99: 34.56ms
  Success: 1000/1000 (100.00%)

🏆 FASTEST AVERAGE: index_optimized (8.91ms)
🏆 FASTEST P99: index_optimized (34.56ms)
🏆 MOST RELIABLE: index_optimized (100.00% success)

Recommendations:
- For best average performance: Use index_optimized
- For consistent latency (P99): Use index_optimized
- For highest reliability: Use index_optimized
```

## Output Files

Reports được lưu trong `./reports/`:

```
reports/
├── report_2025-10-15_14-30-00.json    # JSON report với metrics
└── chart_2025-10-15_14-30-00.html     # HTML chart với Chart.js
```

### JSON Report Structure

```json
{
  "timestamp": "2025-10-15T14:30:00Z",
  "stress_test_result": {
    "total_requests": 50000,
    "successful_requests": 49850,
    "failed_requests": 150,
    "total_duration": "5m0s",
    "requests_per_second": 166.17,
    "avg_latency": "45.23ms",
    "min_latency": "5.12ms",
    "max_latency": "2.345s",
    "operation_stats": {...}
  },
  "search_benchmark": {
    "text_search": {...},
    "regex": {...},
    "aggregation": {...},
    "index_optimized": {...}
  }
}
```

## Performance Tips

### MongoDB Optimization

1. **Indexes**: Tạo compound indexes phù hợp với query patterns
   ```javascript
   db.mails.createIndex({ userId: 1, createdAt: -1 })
   db.mails.createIndex({ userId: 1, subject: 1 })
   ```

2. **Collation**: Sử dụng collation cho case-insensitive search
   ```javascript
   db.mails.createIndex(
     { userId: 1, subject: 1 },
     { collation: { locale: "en", strength: 2 } }
   )
   ```

3. **Connection Pool**: Tăng connection pool size trong production
   ```yaml
   mongodb:
     uri: "mongodb://localhost:27017/?maxPoolSize=100"
   ```

### Stress Test Tuning

1. **Workers**: Tăng `concurrent_workers` cho throughput cao hơn
2. **Rate Limiting**: Điều chỉnh `request_rate` tránh quá tải
3. **Operation Mix**: Thay đổi weights phù hợp với use case thực tế

### Search Strategy Selection

| Strategy | Best For | Avoid When |
|----------|----------|------------|
| **Text Search** | Full-text search, nhiều document | Cần exact matching |
| **Regex** | Pattern matching phức tạp | Dataset lớn, cần tốc độ |
| **Aggregation** | Cần relevance scoring | Yêu cầu latency thấp |
| **Index Optimized** | Production, performance cao | Schema thay đổi thường xuyên |

## Troubleshooting

### MongoDB Connection Issues

```bash
# Kiểm tra MongoDB đang chạy
mongosh --eval "db.adminCommand('ping')"

# Kiểm tra connection từ app
./mail-stress-test -seed  # Nếu fail = connection issue
```

### Memory Issues

```bash
# Giảm concurrent workers
stress_test:
  concurrent_workers: 10  # Từ 50 xuống 10

# Giảm sample size
benchmark:
  sample_size: 100  # Từ 1000 xuống 100
```

### API Handler Timeout

```yaml
mongodb:
  timeout: 60  # Tăng timeout lên 60s

stress_test:
  api_endpoint: "http://localhost:3000"
  request_rate: 50  # Giảm rate xuống
```

## Integration với Backend API

Nếu sử dụng API Handler, backend API cần implement các endpoints:

### Required Endpoints

```
POST   /api/mails              # Create mail
GET    /api/mails              # List mails (với userId, page, limit params)
GET    /api/mails/search       # Search mails (với userId, query params)
```

### Request/Response Format

Xem `models/mail.go` cho chi tiết struct definitions:
- `MailRequest`: Request để tạo mail
- `ListMailsRequest`: Request để list mails
- `SearchMailsRequest`: Request để search
- `Mail`: Mail object structure

## Dependencies

```
go 1.21
require (
    go.mongodb.org/mongo-driver v1.13.1
    gopkg.in/yaml.v3 v3.0.1
)
```

## License

MIT License

## Kết luận

Tool này cung cấp framework hoàn chỉnh để:
- ✅ Test performance của mail system
- ✅ So sánh các search strategies với metrics chi tiết (P50/P95/P99)
- ✅ Generate reports với visualizations
- ✅ Dễ dàng integrate với backend API hoặc database trực tiếp
- ✅ Docker support cho deployment đơn giản
- ✅ Handler Pattern cho flexibility
- ✅ Strategy Pattern cho pluggable search methods

Để bắt đầu nhanh nhất: `./run.sh all` 🚀

---

## Code Implementation Details

Để xem chi tiết implementation của từng module, tham khảo các files:
- `config/config.go` - Configuration management
- `models/mail.go` - Data structures
- `database/mongo.go` - MongoDB connection & indexes  
- `handler/` - MailHandler interface & implementations
- `generator/request_generator.go` - Test data generation
- `search/` - Search strategy implementations
- `benchmark/` - Stress test & search benchmark
- `report/` - Report generation
- `cmd/main.go` - Entry point

---

**Project repository**: `mail_stress_test` by quangphat18ti
**Current branch**: `main`

**Last updated**: October 15, 2025

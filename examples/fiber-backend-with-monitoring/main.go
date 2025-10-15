package main

import (
	"context"
	"log"
	"time"

	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Prometheus metrics
var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "path"},
	)

	httpErrorsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_errors_total",
			Help: "Total number of HTTP errors",
		},
		[]string{"method", "path", "status"},
	)

	activeConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "fiber_connections_active",
			Help: "Number of active connections",
		},
	)

	dbQueriesTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "db_queries_total",
			Help: "Total number of database queries",
		},
	)

	dbQueryDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "db_query_duration_seconds",
			Help:    "Database query duration in seconds",
			Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
	)

	dbConnectionsActive = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "db_connections_active",
			Help: "Active database connections",
		},
	)
)

// Models
type Mail struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserId    string             `bson:"userId" json:"userId"`
	From      string             `bson:"from" json:"from"`
	To        []string           `bson:"to" json:"to"`
	Cc        []string           `bson:"cc,omitempty" json:"cc,omitempty"`
	Bcc       []string           `bson:"bcc,omitempty" json:"bcc,omitempty"`
	Subject   string             `bson:"subject" json:"subject"`
	Content   string             `bson:"content" json:"content"`
	Type      string             `bson:"type" json:"type"`
	ReplyTo   string             `bson:"replyTo,omitempty" json:"replyTo,omitempty"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
}

type CreateMailRequest struct {
	UserId  string   `json:"userId"`
	From    string   `json:"from"`
	To      []string `json:"to"`
	Cc      []string `json:"cc,omitempty"`
	Bcc     []string `json:"bcc,omitempty"`
	Subject string   `json:"subject"`
	Content string   `json:"content"`
	Type    string   `json:"type,omitempty"`
	ReplyTo string   `json:"replyTo,omitempty"`
}

// App state
type App struct {
	db *mongo.Database
}

func main() {
	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://localhost:27017"))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	// Ping MongoDB
	if err := client.Ping(ctx, nil); err != nil {
		log.Fatalf("Failed to ping MongoDB: %v", err)
	}

	db := client.Database("mail_stress_test")
	app := &App{db: db}

	// Create Fiber app
	fiberApp := fiber.New(fiber.Config{
		Prefork:       false,
		CaseSensitive: true,
		StrictRouting: true,
		ServerHeader:  "Fiber",
		AppName:       "Mail Stress Test Backend",
	})

	// Middleware
	fiberApp.Use(recover.New())
	fiberApp.Use(cors.New())
	fiberApp.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	// Prometheus middleware
	fiberApp.Use(PrometheusMiddleware())

	// Routes
	fiberApp.Get("/health", healthHandler)
	fiberApp.Get("/metrics", adaptor.HTTPHandler(promhttp.Handler()))

	// API routes
	api := fiberApp.Group("/api")
	api.Post("/mails", app.createMailHandler)
	api.Get("/mails", app.listMailsHandler)
	api.Get("/mails/search", app.searchMailsHandler)

	// Start server
	log.Println("🚀 Server starting on :3000")
	log.Println("📊 Metrics available at http://localhost:3000/metrics")
	log.Println("💚 Health check at http://localhost:3000/health")

	if err := fiberApp.Listen(":3000"); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// PrometheusMiddleware tracks HTTP metrics
func PrometheusMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Track active connections
		activeConnections.Inc()
		defer activeConnections.Dec()

		// Continue with request
		err := c.Next()

		// Record metrics
		duration := time.Since(start).Seconds()
		status := c.Response().StatusCode()
		method := c.Method()
		path := c.Path()

		// Skip metrics endpoint from metrics
		if path != "/metrics" {
			httpRequestsTotal.WithLabelValues(method, path, string(rune(status))).Inc()
			httpRequestDuration.WithLabelValues(method, path).Observe(duration)

			if status >= 400 {
				httpErrorsTotal.WithLabelValues(method, path, string(rune(status))).Inc()
			}
		}

		return err
	}
}

// Handlers
func healthHandler(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "ok",
		"timestamp": time.Now(),
		"uptime":    time.Since(time.Now()).String(),
	})
}

func (app *App) createMailHandler(c *fiber.Ctx) error {
	var req CreateMailRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Validate required fields
	if req.UserId == "" || req.From == "" || len(req.To) == 0 || req.Subject == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Missing required fields"})
	}

	// Create mail document
	now := time.Now()
	mail := Mail{
		ID:        primitive.NewObjectID(),
		UserId:    req.UserId,
		From:      req.From,
		To:        req.To,
		Cc:        req.Cc,
		Bcc:       req.Bcc,
		Subject:   req.Subject,
		Content:   req.Content,
		Type:      req.Type,
		ReplyTo:   req.ReplyTo,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if mail.Type == "" {
		mail.Type = "sent"
	}

	// Insert to database with metrics
	start := time.Now()
	collection := app.db.Collection("mails")

	_, err := collection.InsertOne(c.Context(), mail)

	// Track DB metrics
	dbQueriesTotal.Inc()
	dbQueryDuration.Observe(time.Since(start).Seconds())

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create mail"})
	}

	return c.Status(201).JSON(mail)
}

func (app *App) listMailsHandler(c *fiber.Ctx) error {
	userId := c.Query("userId")
	if userId == "" {
		return c.Status(400).JSON(fiber.Map{"error": "userId is required"})
	}

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	skip := (page - 1) * limit

	// Query database with metrics
	start := time.Now()
	collection := app.db.Collection("mails")

	filter := bson.M{"userId": userId}
	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := collection.Find(c.Context(), filter, opts)

	// Track DB metrics
	dbQueriesTotal.Inc()
	dbQueryDuration.Observe(time.Since(start).Seconds())

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch mails"})
	}
	defer cursor.Close(c.Context())

	var mails []Mail
	if err := cursor.All(c.Context(), &mails); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode mails"})
	}

	return c.JSON(fiber.Map{
		"data":  mails,
		"page":  page,
		"limit": limit,
		"total": len(mails),
	})
}

func (app *App) searchMailsHandler(c *fiber.Ctx) error {
	userId := c.Query("userId")
	query := c.Query("query")

	if userId == "" || query == "" {
		return c.Status(400).JSON(fiber.Map{"error": "userId and query are required"})
	}

	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 20)
	skip := (page - 1) * limit

	// Search database with metrics
	start := time.Now()
	collection := app.db.Collection("mails")

	filter := bson.M{
		"userId": userId,
		"$or": []bson.M{
			{"subject": bson.M{"$regex": query, "$options": "i"}},
			{"content": bson.M{"$regex": query, "$options": "i"}},
		},
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "createdAt", Value: -1}}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := collection.Find(c.Context(), filter, opts)

	// Track DB metrics
	dbQueriesTotal.Inc()
	dbQueryDuration.Observe(time.Since(start).Seconds())

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to search mails"})
	}
	defer cursor.Close(c.Context())

	var mails []Mail
	if err := cursor.All(c.Context(), &mails); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to decode mails"})
	}

	return c.JSON(fiber.Map{
		"data":  mails,
		"query": query,
		"page":  page,
		"limit": limit,
		"total": len(mails),
	})
}

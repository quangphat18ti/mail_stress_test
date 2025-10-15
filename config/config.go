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
	Monitoring MonitoringConfig `yaml:"monitoring"`
}

type MongoDBConfig struct {
	URI      string `yaml:"uri"`
	Database string `yaml:"database"`
	Timeout  int    `yaml:"timeout"` // seconds
}

type StressTestConfig struct {
	NumUsers          int           `yaml:"num_users"`
	NumMailsPerUser   int           `yaml:"num_mails_per_user"`
	ConcurrentWorkers int           `yaml:"concurrent_workers"`
	RequestRate       int           `yaml:"request_rate"` // requests per second
	Duration          time.Duration `yaml:"duration"`     // test duration
	UseAPI            bool          `yaml:"use_api"`
	APIEndpoint       string        `yaml:"api_endpoint"`
	Operations        Operations    `yaml:"operations"`
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

type MonitoringConfig struct {
	Enabled             bool          `yaml:"enabled"`
	PrometheusURL       string        `yaml:"prometheus_url"`  // e.g., "http://localhost:9090/metrics"
	ScrapeInterval      time.Duration `yaml:"scrape_interval"` // e.g., 5s
	EnableSystemMonitor bool          `yaml:"enable_system_monitor"`
	TargetHost          string        `yaml:"target_host"` // For remote monitoring: "user@host"
	IsDocker            bool          `yaml:"is_docker"`
	ContainerID         string        `yaml:"container_id"`
	EnableRealtimeLog   bool          `yaml:"enable_realtime_log"`
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

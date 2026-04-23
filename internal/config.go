package internal

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var configLogger = NewLogger().GetLogger()

type ObservabilityConfig struct {
	ServiceName             string            `json:"service_name"`
	ServiceVersion          string            `json:"service_version"`
	Environment             string            `json:"environment"`
	Namespace               string            `json:"namespace"`
	MetricsPath             string            `json:"metrics_path"`
	HealthPath              string            `json:"health_path"`
	ReadyPath               string            `json:"ready_path"`
	InfoPath                string            `json:"info_path"`
	TraceSampleRatio        float64           `json:"trace_sample_ratio"`
	TraceExporter           string            `json:"trace_exporter"`
	TraceOTLPEndpoint       string            `json:"trace_otlp_endpoint"`
	TraceOTLPInsecure       bool              `json:"trace_otlp_insecure"`
	TraceOTLPHeaders        map[string]string `json:"trace_otlp_headers"`
	TraceOTLPTimeoutSeconds int               `json:"trace_otlp_timeout_seconds"`
}

type LoggingConfig struct {
	Level       string `json:"level"`
	FileEnabled bool   `json:"file_enabled"`
	FileDir     string `json:"file_dir"`
}

type AdminConfig struct {
	SecretKey string `json:"secret_key"`
}

type Config struct {
	Prod       bool        `json:"prod"`
	AppName    string      `json:"app_name"`
	AuthDomain string      `json:"auth_domain"`
	Languages  []string    `json:"languages"`
	Domains    []string    `json:"domains"`
	Admin      AdminConfig `json:"admin"`
	Server     struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`
	Database struct {
		Host     string `json:"host"`
		User     string `json:"user"`
		Password string `json:"password"`
		DbName   string `json:"dbname"`
		Port     int    `json:"port"`
		Charset  string `json:"charset"`
	} `json:"database"`
	Observability ObservabilityConfig `json:"observability"`
	Logging       LoggingConfig       `json:"logging"`
}

func LoadConfiguration(file string) Config {
	var config Config

	configFile, err := os.Open(file)
	if err != nil {
		configLogger.Fatal("problem loading config file", zap.Error(err), zap.String("file", file))
	}
	defer configFile.Close()

	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		configLogger.Fatal("problem parsing config file", zap.Error(err), zap.String("file", file))
	}

	return config
}

func (c Config) ServerAddress() string {
	host := strings.TrimSpace(c.Server.Host)
	if host == "" {
		host = "0.0.0.0"
	}

	port := c.Server.Port
	if port <= 0 {
		port = 8080
	}

	return fmt.Sprintf("%s:%d", host, port)
}

func (cfg ObservabilityConfig) WithDefaults(appVersion, appName string, prod bool) ObservabilityConfig {
	if strings.TrimSpace(cfg.ServiceName) == "" {
		cfg.ServiceName = strings.TrimSpace(appName)
		if cfg.ServiceName == "" {
			cfg.ServiceName = "procyon"
		}
	}
	if strings.TrimSpace(cfg.ServiceVersion) == "" {
		cfg.ServiceVersion = strings.TrimSpace(appVersion)
		if cfg.ServiceVersion == "" {
			cfg.ServiceVersion = "dev"
		}
	}
	if strings.TrimSpace(cfg.Environment) == "" {
		if prod {
			cfg.Environment = "prod"
		} else {
			cfg.Environment = "dev"
		}
	}
	if strings.TrimSpace(cfg.Namespace) == "" {
		cfg.Namespace = "procyon"
	}
	if strings.TrimSpace(cfg.MetricsPath) == "" {
		cfg.MetricsPath = "/metrics"
	}
	if strings.TrimSpace(cfg.HealthPath) == "" {
		cfg.HealthPath = "/healthz"
	}
	if strings.TrimSpace(cfg.ReadyPath) == "" {
		cfg.ReadyPath = "/readyz"
	}
	if strings.TrimSpace(cfg.InfoPath) == "" {
		cfg.InfoPath = "/info"
	}
	if cfg.TraceSampleRatio <= 0 || cfg.TraceSampleRatio > 1 {
		cfg.TraceSampleRatio = 1
	}
	if strings.TrimSpace(cfg.TraceExporter) == "" {
		cfg.TraceExporter = "log"
	}
	if cfg.TraceOTLPTimeoutSeconds <= 0 {
		cfg.TraceOTLPTimeoutSeconds = 10
	}

	return cfg
}

func (cfg LoggingConfig) WithDefaults() LoggingConfig {
	if strings.TrimSpace(cfg.Level) == "" {
		cfg.Level = "info"
	}
	if strings.TrimSpace(cfg.FileDir) == "" {
		cfg.FileDir = "log"
	}

	return cfg
}

func NewDatabaseConnection(cfg Config) *gorm.DB {
	loc := url.QueryEscape("UTC")
	tz := url.QueryEscape("'+00:00'")
	charset := strings.TrimSpace(cfg.Database.Charset)
	if charset == "" {
		charset = "utf8mb4"
	}

	connStr := fmt.Sprintf(
		"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=%s&time_zone=%s",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.DbName,
		charset,
		loc,
		tz,
	)

	db, err := gorm.Open(mysql.Open(connStr), &gorm.Config{PrepareStmt: true})
	if err != nil {
		configLogger.Fatal("unable to connect to database", zap.Error(err))
	}

	return db
}

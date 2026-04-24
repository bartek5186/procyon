package internal

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
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

type AuthConfig struct {
	Enabled  *bool  `json:"enabled"`
	Provider string `json:"provider"`
	Domain   string `json:"domain"`
}

type RBACConfig struct {
	Enabled *bool `json:"enabled"`
}

type AdminConfig struct {
	Enabled   *bool  `json:"enabled"`
	SecretKey string `json:"secret_key"`
}

type DatabaseConfig struct {
	Driver                     string `json:"driver"`
	Host                       string `json:"host"`
	User                       string `json:"user"`
	Password                   string `json:"password"`
	DbName                     string `json:"dbname"`
	Port                       int    `json:"port"`
	Charset                    string `json:"charset"`
	SSLMode                    string `json:"sslmode"`
	TimeZone                   string `json:"timezone"`
	MaxOpenConns               int    `json:"max_open_conns"`
	MaxIdleConns               int    `json:"max_idle_conns"`
	ConnMaxLifetimeSeconds     int    `json:"conn_max_lifetime_seconds"`
	ConnMaxIdleTimeSeconds     int    `json:"conn_max_idle_time_seconds"`
	MigrationsDir              string `json:"migrations_dir"`
	MigrationsTable            string `json:"migrations_table"`
	DisableVersionedMigrations bool   `json:"disable_versioned_migrations"`
}

type Config struct {
	Prod       bool        `json:"prod"`
	AppName    string      `json:"app_name"`
	AuthDomain string      `json:"auth_domain"`
	Languages  []string    `json:"languages"`
	Domains    []string    `json:"domains"`
	Auth       AuthConfig  `json:"auth"`
	RBAC       RBACConfig  `json:"rbac"`
	Admin      AdminConfig `json:"admin"`
	Server     struct {
		Host string `json:"host"`
		Port int    `json:"port"`
	} `json:"server"`
	Database      DatabaseConfig      `json:"database"`
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

	config.ApplyEnv()
	if err := config.Validate(); err != nil {
		configLogger.Fatal("invalid configuration", zap.Error(err), zap.String("file", file))
	}

	return config
}

func (c *Config) ApplyEnv() {
	setStringFromEnv(&c.AppName, "APP_NAME")
	setStringFromEnv(&c.AuthDomain, "AUTH_DOMAIN")
	setBoolFromEnv(&c.Prod, "APP_PROD")

	setStringFromEnv(&c.Auth.Provider, "AUTH_PROVIDER")
	setStringFromEnv(&c.Auth.Domain, "AUTH_DOMAIN")
	setOptionalBoolFromEnv(&c.Auth.Enabled, "AUTH_ENABLED")
	setOptionalBoolFromEnv(&c.RBAC.Enabled, "RBAC_ENABLED")
	setOptionalBoolFromEnv(&c.Admin.Enabled, "ADMIN_ENABLED")
	setStringFromEnv(&c.Admin.SecretKey, "ADMIN_SECRET_KEY")

	setStringFromEnv(&c.Server.Host, "SERVER_HOST")
	setIntFromEnv(&c.Server.Port, "SERVER_PORT")

	setStringFromEnv(&c.Database.Driver, "DB_DRIVER")
	setStringFromEnv(&c.Database.Host, "DB_HOST")
	setStringFromEnv(&c.Database.User, "DB_USER")
	setStringFromEnv(&c.Database.Password, "DB_PASSWORD")
	setStringFromEnv(&c.Database.DbName, "DB_NAME")
	setIntFromEnv(&c.Database.Port, "DB_PORT")
	setStringFromEnv(&c.Database.Charset, "DB_CHARSET")
	setStringFromEnv(&c.Database.SSLMode, "DB_SSLMODE")
	setStringFromEnv(&c.Database.TimeZone, "DB_TIMEZONE")
	setIntFromEnv(&c.Database.MaxOpenConns, "DB_MAX_OPEN_CONNS")
	setIntFromEnv(&c.Database.MaxIdleConns, "DB_MAX_IDLE_CONNS")
	setIntFromEnv(&c.Database.ConnMaxLifetimeSeconds, "DB_CONN_MAX_LIFETIME_SECONDS")
	setIntFromEnv(&c.Database.ConnMaxIdleTimeSeconds, "DB_CONN_MAX_IDLE_TIME_SECONDS")
	setStringFromEnv(&c.Database.MigrationsDir, "DB_MIGRATIONS_DIR")
	setStringFromEnv(&c.Database.MigrationsTable, "DB_MIGRATIONS_TABLE")
	setBoolFromEnv(&c.Database.DisableVersionedMigrations, "DB_DISABLE_VERSIONED_MIGRATIONS")

	setStringFromEnv(&c.Observability.ServiceName, "OTEL_SERVICE_NAME")
	setStringFromEnv(&c.Observability.ServiceVersion, "SERVICE_VERSION")
	setStringFromEnv(&c.Observability.Environment, "APP_ENV")
	setStringFromEnv(&c.Observability.Namespace, "METRICS_NAMESPACE")
	setStringFromEnv(&c.Observability.TraceExporter, "TRACE_EXPORTER")
	setStringFromEnv(&c.Observability.TraceOTLPEndpoint, "TRACE_OTLP_ENDPOINT")
	setBoolFromEnv(&c.Observability.TraceOTLPInsecure, "TRACE_OTLP_INSECURE")
	setIntFromEnv(&c.Observability.TraceOTLPTimeoutSeconds, "TRACE_OTLP_TIMEOUT_SECONDS")

	setStringFromEnv(&c.Logging.Level, "LOG_LEVEL")
	setBoolFromEnv(&c.Logging.FileEnabled, "LOG_FILE_ENABLED")
	setStringFromEnv(&c.Logging.FileDir, "LOG_FILE_DIR")
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.AppName) == "" {
		return fmt.Errorf("app_name is required")
	}
	if strings.TrimSpace(c.Database.Host) == "" {
		return fmt.Errorf("database.host is required")
	}
	if strings.TrimSpace(c.Database.User) == "" {
		return fmt.Errorf("database.user is required")
	}
	if strings.TrimSpace(c.Database.DbName) == "" {
		return fmt.Errorf("database.dbname is required")
	}
	if c.Database.Port <= 0 {
		return fmt.Errorf("database.port must be positive")
	}
	if c.AuthEnabled() && c.AuthProvider() == "kratos" && strings.TrimSpace(c.AuthBaseURL()) == "" {
		return fmt.Errorf("auth.domain or auth_domain is required when auth provider is kratos")
	}
	if c.RBACEnabled() && !c.AuthEnabled() {
		return fmt.Errorf("rbac requires auth.enabled=true")
	}
	if c.AdminEnabled() && strings.TrimSpace(c.Admin.SecretKey) == "" {
		return fmt.Errorf("admin.secret_key is required when admin is enabled")
	}
	if c.Prod {
		if c.AdminEnabled() && strings.TrimSpace(c.Admin.SecretKey) == "CHANGE_ME_ADMIN_KEY" {
			return fmt.Errorf("admin.secret_key must be changed in prod")
		}
		if strings.TrimSpace(c.Database.Password) == "" {
			return fmt.Errorf("database.password is required in prod")
		}
	}

	return nil
}

func (c Config) AuthEnabled() bool {
	return optionalBool(c.Auth.Enabled, true)
}

func (c Config) AuthProvider() string {
	provider := strings.ToLower(strings.TrimSpace(c.Auth.Provider))
	if provider == "" {
		provider = "kratos"
	}
	return provider
}

func (c Config) AuthBaseURL() string {
	if domain := strings.TrimSpace(c.Auth.Domain); domain != "" {
		return domain
	}
	return strings.TrimSpace(c.AuthDomain)
}

func (c Config) RBACEnabled() bool {
	return optionalBool(c.RBAC.Enabled, c.AuthEnabled())
}

func (c Config) AdminEnabled() bool {
	return optionalBool(c.Admin.Enabled, strings.TrimSpace(c.Admin.SecretKey) != "")
}

func optionalBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
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
	driver := strings.ToLower(strings.TrimSpace(cfg.Database.Driver))
	if driver == "" {
		driver = "mysql"
	}

	var dialector gorm.Dialector
	switch driver {
	case "mysql":
		charset := strings.TrimSpace(cfg.Database.Charset)
		if charset == "" {
			charset = "utf8mb4"
		}

		timeZone := strings.TrimSpace(cfg.Database.TimeZone)
		if timeZone == "" {
			timeZone = "UTC"
		}
		timeZoneLoc := url.QueryEscape(timeZone)
		timeZoneSetting := url.QueryEscape("'+00:00'")
		if !strings.EqualFold(timeZone, "UTC") {
			timeZoneSetting = url.QueryEscape("'" + timeZone + "'")
		}

		connStr := fmt.Sprintf(
			"%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=true&loc=%s&time_zone=%s",
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.Host,
			cfg.Database.Port,
			cfg.Database.DbName,
			charset,
			timeZoneLoc,
			timeZoneSetting,
		)
		dialector = mysql.Open(connStr)
	case "postgres", "postgresql":
		sslMode := strings.TrimSpace(cfg.Database.SSLMode)
		if sslMode == "" {
			sslMode = "disable"
		}

		timeZone := strings.TrimSpace(cfg.Database.TimeZone)
		if timeZone == "" {
			timeZone = "UTC"
		}

		connStr := fmt.Sprintf(
			"host=%s user=%s password=%s dbname=%s port=%d sslmode=%s TimeZone=%s",
			cfg.Database.Host,
			cfg.Database.User,
			cfg.Database.Password,
			cfg.Database.DbName,
			cfg.Database.Port,
			sslMode,
			timeZone,
		)
		dialector = postgres.Open(connStr)
	default:
		configLogger.Fatal("unsupported database driver", zap.String("driver", driver))
	}

	db, err := gorm.Open(dialector, &gorm.Config{PrepareStmt: true})
	if err != nil {
		configLogger.Fatal("unable to connect to database", zap.Error(err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		configLogger.Fatal("unable to configure database pool", zap.Error(err))
	}
	if cfg.Database.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	}
	if cfg.Database.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	}
	if cfg.Database.ConnMaxLifetimeSeconds > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetimeSeconds) * time.Second)
	}
	if cfg.Database.ConnMaxIdleTimeSeconds > 0 {
		sqlDB.SetConnMaxIdleTime(time.Duration(cfg.Database.ConnMaxIdleTimeSeconds) * time.Second)
	}

	return db
}

func setStringFromEnv(target *string, key string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value != "" {
		*target = value
	}
}

func setIntFromEnv(target *int, key string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		configLogger.Fatal("invalid integer environment value", zap.String("key", key), zap.String("value", value), zap.Error(err))
	}
	*target = parsed
}

func setBoolFromEnv(target *bool, key string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		configLogger.Fatal("invalid boolean environment value", zap.String("key", key), zap.String("value", value), zap.Error(err))
	}
	*target = parsed
}

func setOptionalBoolFromEnv(target **bool, key string) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		configLogger.Fatal("invalid boolean environment value", zap.String("key", key), zap.String("value", value), zap.Error(err))
	}
	*target = &parsed
}

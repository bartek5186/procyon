package internal

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var configLogger = NewLogger().GetLogger()

type ObservabilityConfig struct {
	TraceExporter     string  `json:"trace_exporter"`
	MetricsExporter   string  `json:"metrics_exporter"`
	TraceOTLPEndpoint string  `json:"trace_otlp_endpoint"`
	TraceSampleRatio  float64 `json:"trace_sample_ratio"`

	// derived by WithDefaults — not in JSON
	ServiceName       string
	ServiceVersion    string
	Environment       string
	TraceOTLPInsecure bool
}

type LoggingConfig struct {
	Level       string `json:"level"`
	FileEnabled bool   `json:"file_enabled"`
	FileDir     string `json:"file_dir"`
}

type AuthConfig struct {
	Provider string `json:"provider"`
	Domain   string `json:"domain"`
}

type RBACConfig struct {
	DefaultRole      string   `json:"default_role"`
	AdminIdentityIDs []string `json:"admin_identity_ids"`
}

type AdminConfig struct {
	SecretKey string `json:"secret_key"`
}

type DatabaseConfig struct {
	Driver                 string `json:"driver"`
	Host                   string `json:"host"`
	User                   string `json:"user"`
	Password               string `json:"password"`
	DbName                 string `json:"dbname"`
	Port                   int    `json:"port"`
	Charset                string `json:"charset"`
	SSLMode                string `json:"sslmode"`
	TimeZone               string `json:"timezone"`
	MaxOpenConns           int    `json:"max_open_conns"`
	MaxIdleConns           int    `json:"max_idle_conns"`
	ConnMaxLifetimeSeconds int    `json:"conn_max_lifetime_seconds"`
	ConnMaxIdleTimeSeconds int    `json:"conn_max_idle_time_seconds"`
	AutoMigrate            *bool  `json:"auto_migrate"`
	MigrationsDir          string `json:"migrations_dir"`
	MigrationsTable        string `json:"migrations_table"`
}

type ServerConfig struct {
	Name string `json:"name"`
	Host string `json:"host"`
	Port int    `json:"port"`
}

func (s ServerConfig) address(defaultHost string, defaultPort int) string {
	host := strings.TrimSpace(s.Host)
	if host == "" {
		host = defaultHost
	}
	port := s.Port
	if port <= 0 {
		port = defaultPort
	}
	return fmt.Sprintf("%s:%d", host, port)
}

type Config struct {
	Prod          bool           `json:"prod"`
	AppName       string         `json:"app_name"`
	AuthDomain    string         `json:"auth_domain"`
	Languages     []string       `json:"languages"`
	Domains       []string       `json:"domains"`
	Auth          AuthConfig     `json:"auth"`
	RBAC          RBACConfig     `json:"rbac"`
	Admin         AdminConfig    `json:"admin"`
	Servers       []ServerConfig `json:"servers"`
	Database      DatabaseConfig      `json:"database"`
	Observability ObservabilityConfig `json:"observability"`
	Logging       LoggingConfig       `json:"logging"`
}

func (c Config) server(name string) ServerConfig {
	for _, s := range c.Servers {
		if s.Name == name {
			return s
		}
	}
	return ServerConfig{}
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

	if err := config.Validate(); err != nil {
		configLogger.Fatal("invalid configuration", zap.Error(err), zap.String("file", file))
	}

	return config
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
	if c.AuthProvider() == "kratos" && strings.TrimSpace(c.AuthBaseURL()) == "" {
		return fmt.Errorf("auth.domain or auth_domain is required when auth provider is kratos")
	}
	if strings.TrimSpace(c.Admin.SecretKey) == "" {
		return fmt.Errorf("admin.secret_key is required")
	}
	if c.Prod {
		if strings.TrimSpace(c.Admin.SecretKey) == "CHANGE_ME_ADMIN_KEY" {
			return fmt.Errorf("admin.secret_key must be changed in prod")
		}
		if strings.TrimSpace(c.Database.Password) == "" {
			return fmt.Errorf("database.password is required in prod")
		}
	}

	return nil
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

func (c Config) AutoMigrateEnabled() bool {
	return optionalBool(c.Database.AutoMigrate, true)
}

func optionalBool(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func (c Config) PublicAddress() string { return c.server("public").address("0.0.0.0", 8080) }
func (c Config) AdminAddress() string  { return c.server("admin").address("127.0.0.1", 8081) }
func (c Config) UploadAddress() string { return c.server("upload").address("0.0.0.0", 8082) }

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
	if cfg.TraceSampleRatio <= 0 || cfg.TraceSampleRatio > 1 {
		cfg.TraceSampleRatio = 1
	}
	if strings.TrimSpace(cfg.TraceExporter) == "" {
		cfg.TraceExporter = "log"
	}
	if strings.TrimSpace(cfg.MetricsExporter) == "" {
		cfg.MetricsExporter = "none"
	}
	cfg.TraceOTLPInsecure = !prod

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

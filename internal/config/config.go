package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Auth          AuthConfig          `mapstructure:"auth"`
	AWS           AWSConfig           `mapstructure:"aws"`
	Caddy         CaddyConfig         `mapstructure:"caddy"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Discovery     DiscoveryConfig     `mapstructure:"discovery"`
	Observability ObservabilityConfig `mapstructure:"observability"`
}

type ServerConfig struct {
	Port               string        `mapstructure:"port"`
	CORSAllowedOrigins []string      `mapstructure:"cors_allowed_origins"`
	GinMode            string        `mapstructure:"gin_mode"` // debug, release, test; empty = release
	ReadHeaderTimeout  time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout        time.Duration `mapstructure:"read_timeout"`
	WriteTimeout       time.Duration `mapstructure:"write_timeout"`
	IdleTimeout        time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout    time.Duration `mapstructure:"shutdown_timeout"`
	MaxBodyBytes       int64         `mapstructure:"max_body_bytes"`
	MaxApplyBodyBytes  int64         `mapstructure:"max_apply_body_bytes"`
	EnableSwagger      bool          `mapstructure:"enable_swagger"`
}

type AuthConfig struct {
	TokenTTLMinutes   int    `mapstructure:"token_ttl_minutes"`
	RefreshTTLMinutes int    `mapstructure:"refresh_ttl_minutes"`
	JWTSecret         string `mapstructure:"jwt_secret"`
	JWTSecretARN      string `mapstructure:"jwt_secret_arn"`
	UsersSecretARN    string `mapstructure:"users_secret_arn"`
	Issuer            string `mapstructure:"issuer"`
	Audience          string `mapstructure:"audience"`
}

type AWSConfig struct {
	Profile string   `mapstructure:"profile"`
	Regions []string `mapstructure:"regions"`
}

type CaddyConfig struct {
	CacheTTL time.Duration `mapstructure:"cache_ttl"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Name            string        `mapstructure:"name"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Params          string        `mapstructure:"params"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
	ConnectRetries  int           `mapstructure:"connect_retries"`
	ConnectBackoff  time.Duration `mapstructure:"connect_backoff"`
}

type DiscoveryConfig struct {
	DefaultTagKey   string `mapstructure:"default_tag_key"`
	DefaultTagValue string `mapstructure:"default_tag_value"`
}

type ObservabilityConfig struct {
	LogLevel string `mapstructure:"log_level"`
}

func Load() (*Config, error) {
	// Best-effort .env loading for local development.
	_ = godotenv.Load(".env")

	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath("./configs")
	v.AddConfigPath(".")

	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Server
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.gin_mode", "release")
	v.BindEnv("server.gin_mode", "GIN_MODE")
	v.SetDefault("server.read_header_timeout", "5s")
	v.SetDefault("server.read_timeout", "30s")
	v.SetDefault("server.write_timeout", "30s")
	v.SetDefault("server.idle_timeout", "60s")
	v.SetDefault("server.shutdown_timeout", "15s")
	v.SetDefault("server.max_body_bytes", 1048576)
	v.SetDefault("server.max_apply_body_bytes", 10485760)
	v.SetDefault("server.enable_swagger", true)
	_ = v.BindEnv("server.read_timeout", "SERVER_READ_TIMEOUT")
	_ = v.BindEnv("server.write_timeout", "SERVER_WRITE_TIMEOUT")
	_ = v.BindEnv("server.shutdown_timeout", "SERVER_SHUTDOWN_TIMEOUT")
	_ = v.BindEnv("server.max_body_bytes", "SERVER_MAX_BODY_BYTES")
	_ = v.BindEnv("server.max_apply_body_bytes", "SERVER_MAX_APPLY_BODY_BYTES")
	_ = v.BindEnv("server.cors_allowed_origins", "CORS_ALLOWED_ORIGINS")

	// Auth
	v.SetDefault("auth.token_ttl_minutes", 60)
	v.SetDefault("auth.refresh_ttl_minutes", 10080) // 7 days
	v.BindEnv("auth.jwt_secret", "JWT_SECRET")
	v.SetDefault("auth.issuer", "caddy-dashboard")
	v.SetDefault("auth.audience", "caddy-dashboard-api")
	_ = v.BindEnv("auth.issuer", "JWT_ISSUER")
	_ = v.BindEnv("auth.audience", "JWT_AUDIENCE")

	// AWS
	v.SetDefault("aws.regions", []string{"eu-south-1", "eu-central-1"})
	v.BindEnv("aws.profile", "AWS_PROFILE")
	v.BindEnv("aws.regions", "AWS_REGIONS")

	// Caddy
	v.SetDefault("caddy.cache_ttl", "2m")
	_ = v.BindEnv("caddy.cache_ttl", "CADDY_CACHE_TTL")

	// DB
	v.BindEnv("database.host", "DB_HOST")
	v.SetDefault("database.port", 3306)
	v.BindEnv("database.port", "DB_PORT")
	v.BindEnv("database.name", "DB_NAME")
	v.BindEnv("database.user", "DB_USER")
	v.BindEnv("database.password", "DB_PASSWORD")
	v.SetDefault("database.params", "charset=utf8mb4&parseTime=True&loc=Local")
	v.BindEnv("database.params", "DB_PARAMS")
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)
	v.SetDefault("database.conn_max_lifetime", "30m")
	v.SetDefault("database.conn_max_idle_time", "10m")
	v.SetDefault("database.connect_retries", 10)
	v.SetDefault("database.connect_backoff", "500ms")
	_ = v.BindEnv("database.max_open_conns", "DB_MAX_OPEN_CONNS")
	_ = v.BindEnv("database.max_idle_conns", "DB_MAX_IDLE_CONNS")
	_ = v.BindEnv("database.conn_max_lifetime", "DB_CONN_MAX_LIFETIME")
	_ = v.BindEnv("database.conn_max_idle_time", "DB_CONN_MAX_IDLE_TIME")
	_ = v.BindEnv("database.connect_retries", "DB_CONNECT_RETRIES")
	_ = v.BindEnv("database.connect_backoff", "DB_CONNECT_BACKOFF")

	// Observability
	v.SetDefault("observability.log_level", "info")
	v.BindEnv("observability.log_level", "LOG_LEVEL")

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		if strings.TrimSpace(cfg.Auth.JWTSecretARN) != "" {
			cfg.Auth.JWTSecret = strings.TrimSpace(os.Getenv("JWT_SECRET_RESOLVED"))
		}
	}
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return nil, fmt.Errorf("auth.jwt_secret is required (set JWT_SECRET or auth.jwt_secret in config)")
	}
	if len(strings.TrimSpace(cfg.Auth.JWTSecret)) < 32 || strings.EqualFold(strings.TrimSpace(cfg.Auth.JWTSecret), "change-me") {
		return nil, fmt.Errorf("auth.jwt_secret must be at least 32 chars and not a placeholder")
	}
	return cfg, nil
}

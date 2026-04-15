package config

import (
	"fmt"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	Server        ServerConfig        `mapstructure:"server"`
	Auth          AuthConfig          `mapstructure:"auth"`
	AWS           AWSConfig           `mapstructure:"aws"`
	Database      DatabaseConfig      `mapstructure:"database"`
	Discovery     DiscoveryConfig     `mapstructure:"discovery"`
	Observability ObservabilityConfig `mapstructure:"observability"`
}

type ServerConfig struct {
	Port               string   `mapstructure:"port"`
	CORSAllowedOrigins []string `mapstructure:"cors_allowed_origins"`
	GinMode            string   `mapstructure:"gin_mode"` // debug, release, test; empty = release
}

type AuthConfig struct {
	TokenTTLMinutes     int    `mapstructure:"token_ttl_minutes"`
	RefreshTTLMinutes   int    `mapstructure:"refresh_ttl_minutes"`
	JWTSecret           string `mapstructure:"jwt_secret"`
}

type AWSConfig struct {
	Profile string   `mapstructure:"profile"`
	Regions []string `mapstructure:"regions"`
}

type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Name     string `mapstructure:"name"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Params   string `mapstructure:"params"`
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

	// Auth
	v.SetDefault("auth.token_ttl_minutes", 60)
	v.SetDefault("auth.refresh_ttl_minutes", 10080) // 7 days
	v.BindEnv("auth.jwt_secret", "JWT_SECRET")

	// AWS
	v.SetDefault("aws.regions", []string{"eu-south-1", "eu-central-1"})
	v.BindEnv("aws.profile", "AWS_PROFILE")
	v.BindEnv("aws.regions", "AWS_REGIONS")

	// DB
	v.BindEnv("database.host", "DB_HOST")
	v.SetDefault("database.port", 5432)
	v.BindEnv("database.port", "DB_PORT")
	v.BindEnv("database.name", "DB_NAME")
	v.BindEnv("database.user", "DB_USER")
	v.BindEnv("database.password", "DB_PASSWORD")
	v.SetDefault("database.params", "charset=utf8mb4&parseTime=True&loc=Local")
	v.BindEnv("database.params", "DB_PARAMS")

	// Observability
	v.SetDefault("observability.log_level", "info")
	v.BindEnv("observability.log_level", "LOG_LEVEL")

	_ = v.ReadInConfig()

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if strings.TrimSpace(cfg.Auth.JWTSecret) == "" {
		return nil, fmt.Errorf("auth.jwt_secret is required (set JWT_SECRET or auth.jwt_secret in config)")
	}
	return cfg, nil
}

package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Server   ServerConfig   `mapstructure:"server"`
	Store    StoreConfig    `mapstructure:"store"`
	Database DatabaseConfig `mapstructure:"database"`
	Redis    RedisConfig    `mapstructure:"redis"`
	Auth     AuthConfig     `mapstructure:"auth"`
	OAuth    OAuthConfig    `mapstructure:"oauth"`
	Security SecurityConfig `mapstructure:"security"`
	Logging  LoggingConfig  `mapstructure:"logging"`
}

type OAuthConfig struct {
	BaseURL           string `mapstructure:"base_url"` // e.g. http://localhost:8080
	FacebookClientID  string `mapstructure:"facebook_client_id"`
	FacebookSecret    string `mapstructure:"facebook_secret"`
	InstagramClientID string `mapstructure:"instagram_client_id"` // same app as Facebook
	InstagramSecret   string `mapstructure:"instagram_secret"`
	XClientID         string `mapstructure:"x_client_id"`
	XSecret           string `mapstructure:"x_secret"`
	BlueskyEnabled    bool   `mapstructure:"bluesky_enabled"`
}

type ServerConfig struct {
	Port     string `mapstructure:"port"`
	Host     string `mapstructure:"host"`
	Domain   string `mapstructure:"domain"` // public hostname, e.g. xyzping.com or localhost
	CertPath string `mapstructure:"cert_path"`
	KeyPath  string `mapstructure:"key_path"`
	Env      string `mapstructure:"env"`
}

// StoreConfig selects the WebFinger record backend. When Database.URL is
// empty the server runs standalone from the JSON file at DataFile.
type StoreConfig struct {
	DataFile string `mapstructure:"data_file"`
}

type DatabaseConfig struct {
	URL          string `mapstructure:"url"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

type RedisConfig struct {
	URL      string `mapstructure:"url"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type AuthConfig struct {
	JWTSecret        string `mapstructure:"jwt_secret"`
	SessionSecret    string `mapstructure:"session_secret"`
	TokenExpiryHours int    `mapstructure:"token_expiry_hours"`
}

type SecurityConfig struct {
	RateLimitRPS   int      `mapstructure:"rate_limit_rps"`
	RateLimitBurst int      `mapstructure:"rate_limit_burst"`
	AllowedOrigins []string `mapstructure:"allowed_origins"`
	TrustedProxies []string `mapstructure:"trusted_proxies"`
	EnableCSP      bool     `mapstructure:"enable_csp"`
	EnableHSTS     bool     `mapstructure:"enable_hsts"`
	ForceHTTPS     bool     `mapstructure:"force_https"`
}

type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
}

// Load reads configuration from various sources (env vars, config files, defaults)
func Load() (*Config, error) {
	v := viper.New()

	// Set defaults
	setDefaults(v)

	// Environment variables
	v.SetEnvPrefix("WEBFINGER")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Config file (optional)
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("./config")

	// Read config file if it exists (ignore errors)
	v.ReadInConfig()

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &config, nil
}

func setDefaults(v *viper.Viper) {
	// Server defaults
	v.SetDefault("server.port", "8080")
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.domain", "localhost")
	v.SetDefault("server.env", "production")

	// Store defaults (standalone mode reads JRD records from this file)
	v.SetDefault("store.data_file", "data/data.json")

	// Database defaults
	v.SetDefault("database.max_open_conns", 25)
	v.SetDefault("database.max_idle_conns", 5)

	// Redis defaults
	v.SetDefault("redis.url", "redis://localhost:6379")
	v.SetDefault("redis.db", 0)

	// Auth defaults
	v.SetDefault("auth.token_expiry_hours", 24)

	// Security defaults
	v.SetDefault("security.rate_limit_rps", 10)
	v.SetDefault("security.rate_limit_burst", 20)
	v.SetDefault("security.allowed_origins", []string{"*"})
	v.SetDefault("security.enable_csp", true)
	v.SetDefault("security.enable_hsts", true)
	v.SetDefault("security.force_https", true)

	// Logging defaults
	v.SetDefault("logging.level", "info")
	v.SetDefault("logging.format", "json")
	v.SetDefault("logging.output", "stdout")
}

// IsTestEnv returns true if running in test environment
func (c *Config) IsTestEnv() bool {
	return c.Server.Env == "test" || c.Server.Env == "testing"
}

// IsDevelopmentEnv returns true if running in development environment
func (c *Config) IsDevelopmentEnv() bool {
	return c.Server.Env == "development" || c.Server.Env == "dev"
}

// IsProductionEnv returns true if running in production environment
func (c *Config) IsProductionEnv() bool {
	return c.Server.Env == "production" || c.Server.Env == "prod"
}

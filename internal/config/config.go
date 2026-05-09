package config

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server  ServerConfig  `mapstructure:"server"`
	Auth    AuthConfig    `mapstructure:"auth"`
	Storage StorageConfig `mapstructure:"storage"`
	Audit   AuditConfig   `mapstructure:"audit"`
}

type ServerConfig struct {
	Host string    `mapstructure:"host"`
	Port int       `mapstructure:"port"`
	TLS  TLSConfig `mapstructure:"tls"`
}

type TLSConfig struct {
	CertFile string `mapstructure:"cert_file"`
	KeyFile  string `mapstructure:"key_file"`
}

type AuthConfig struct {
	Type    string         `mapstructure:"type"`
	APIKeys []APIKeyConfig `mapstructure:"api_keys"`
}

type APIKeyConfig struct {
	Key          string   `mapstructure:"key"`
	ClientID     string   `mapstructure:"client_id"`
	AllowedPaths []string `mapstructure:"allowed_paths"`
}

type StorageConfig struct {
	Type     string         `mapstructure:"type"`
	SQLite   SQLiteConfig   `mapstructure:"sqlite"`
	Postgres PostgresConfig `mapstructure:"postgres"`
}

type SQLiteConfig struct {
	Path string `mapstructure:"path"`
}

type PostgresConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	SSLMode  string `mapstructure:"sslmode"`
}

type AuditConfig struct {
	Type    string             `mapstructure:"type"`
	File    FileAuditConfig    `mapstructure:"file"`
	Webhook WebhookAuditConfig `mapstructure:"webhook"`
}

type FileAuditConfig struct {
	Path       string `mapstructure:"path"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
}

type WebhookAuditConfig struct {
	URL   string `mapstructure:"url"`
	Token string `mapstructure:"token"`
}

type SecretRequest struct {
	SecretPath              string            `json:"secret_path"`
	Reason                  string            `json:"reason"`
	ClientID                string            `json:"client_id"`
	RequestedDuration       time.Duration     `json:"requested_duration"`
	NotificationPreferences []string          `json:"notification_preferences"`
	Metadata                map[string]string `json:"metadata"`
}

var secretPathRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-\/\.]+$`)

func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("server.host", "0.0.0.0")
	v.SetDefault("server.port", 8080)
	v.SetDefault("storage.type", "sqlite")
	v.SetDefault("storage.sqlite.path", "/var/lib/traverse/traverse.db")
	v.SetDefault("audit.type", "file")
	v.SetDefault("audit.file.path", "/var/log/traverse/audit.log")

	// Environment variables
	v.SetEnvPrefix("TRAVERSE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Config file
	if configPath != "" {
		v.SetConfigFile(configPath)
		if err := v.ReadInConfig(); err != nil {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func ValidateRequest(req SecretRequest) error {
	if req.SecretPath == "" {
		return fmt.Errorf("secret_path is required")
	}

	if !secretPathRegex.MatchString(req.SecretPath) {
		return fmt.Errorf("secret_path contains invalid characters")
	}

	if len(req.Reason) < 10 {
		return fmt.Errorf("reason must be at least 10 characters")
	}

	return nil
}

func (c *Config) GetAPIKeys() map[string]APIKeyConfig {
	keys := make(map[string]APIKeyConfig)
	for _, key := range c.Auth.APIKeys {
		keys[key.Key] = key
	}
	return keys
}

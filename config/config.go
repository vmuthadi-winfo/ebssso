package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Environment string         `yaml:"environment"`
	Server      ServerConfig   `yaml:"server"`
	OIDC        OIDCConfig     `yaml:"oidc"`
	Database    DatabaseConfig `yaml:"database"`
	EBS         EBSConfig      `yaml:"ebs"`
	Logging     LoggingConfig  `yaml:"logging"`
	Session     SessionConfig  `yaml:"session"`
}

// ServerConfig holds server-related settings
type ServerConfig struct {
	Port    int    `yaml:"port"`
	Host    string `yaml:"host"`
	BaseURL string `yaml:"base_url"`
}

// OIDCConfig holds OIDC provider settings
type OIDCConfig struct {
	ProviderURL  string   `yaml:"provider_url"`
	ClientID     string   `yaml:"client_id"`
	ClientSecret string   `yaml:"client_secret"`
	RedirectURL  string   `yaml:"redirect_url"`
	Scopes       []string `yaml:"scopes"`
	UserClaim    string   `yaml:"user_claim"`
}

// DatabaseConfig holds Oracle database connection settings
type DatabaseConfig struct {
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	ConnectionString string `yaml:"connection_string"`
	PoolSize         int    `yaml:"pool_size"`
	MaxOpenConns     int    `yaml:"max_open_conns"`
	MaxIdleConns     int    `yaml:"max_idle_conns"`
}

// EBSConfig holds EBS-specific settings
type EBSConfig struct {
	BaseURL      string `yaml:"base_url"`
	HomePage     string `yaml:"home_page"`
	CookieDomain string `yaml:"cookie_domain"`
	CookieSecure bool   `yaml:"cookie_secure"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	Output string `yaml:"output"`
}

// SessionConfig holds session management settings
type SessionConfig struct {
	TimeoutMinutes int    `yaml:"timeout_minutes"`
	CookieName     string `yaml:"cookie_name"`
}

// Load reads and parses the configuration file
func Load(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate required fields
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate checks if all required configuration fields are set
func (c *Config) Validate() error {
	if c.OIDC.ProviderURL == "" {
		return fmt.Errorf("oidc.provider_url is required")
	}
	if c.OIDC.ClientID == "" {
		return fmt.Errorf("oidc.client_id is required")
	}
	if c.OIDC.ClientSecret == "" {
		return fmt.Errorf("oidc.client_secret is required")
	}
	if c.OIDC.RedirectURL == "" {
		return fmt.Errorf("oidc.redirect_url is required")
	}
	if c.Database.ConnectionString == "" {
		return fmt.Errorf("database.connection_string is required")
	}
	if c.Database.Username == "" {
		return fmt.Errorf("database.username is required")
	}
	if c.EBS.BaseURL == "" {
		return fmt.Errorf("ebs.base_url is required")
	}
	return nil
}

// LoadFromEnv loads the configuration based on the EBS_ENV environment variable
func LoadFromEnv() (*Config, error) {
	env := os.Getenv("EBS_ENV")
	if env == "" {
		env = "DEV"
	}

	// Try to load config.yaml first, then fall back to environment-specific config
	configFile := "config.yaml"
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		configFile = fmt.Sprintf("config.%s.yaml", env)
	}

	return Load(configFile)
}

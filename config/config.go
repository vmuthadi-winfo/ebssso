package config

import (
	"errors"
	"fmt"
	"io/fs"
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
	// Legacy direct connection (deprecated)
	Username         string `yaml:"username"`
	Password         string `yaml:"password"`
	ConnectionString string `yaml:"connection_string"`
	
	// DBC file approach (recommended)
	DBCFile          string `yaml:"dbc_file"`
	AppsUser         string `yaml:"apps_user"`
	UseDBC           bool   `yaml:"use_dbc"` // If true, use DBC file method
	
	// Connection pool settings
	PoolSize         int    `yaml:"pool_size"`
	MaxOpenConns     int    `yaml:"max_open_conns"`
	MaxIdleConns     int    `yaml:"max_idle_conns"`
}

// EBSConfig holds EBS-specific settings
type EBSConfig struct {
	BaseURL           string   `yaml:"base_url"`
	HomePage          string   `yaml:"home_page"`
	CookieDomain      string   `yaml:"cookie_domain"`
	CookieSecure      bool     `yaml:"cookie_secure"`
	
	// Trusted node configuration
	TrustedNodeName   string   `yaml:"trusted_node_name"`
	TrustedNodeSecret string   `yaml:"trusted_node_secret"`
	
	// Proxy configuration for EBS requests
	EnableProxy       bool     `yaml:"enable_proxy"`
	AllowedPaths      []string `yaml:"allowed_paths"`
	
	// WebADI and form support
	EnableReturnURL   bool     `yaml:"enable_return_url"`
	MaxUploadSize     int64    `yaml:"max_upload_size"` // in bytes
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
	
	// Validate database configuration
	if c.Database.UseDBC {
		// DBC file method
		if c.Database.DBCFile == "" {
			return fmt.Errorf("database.dbc_file is required when use_dbc is true")
		}
		if c.Database.AppsUser == "" {
			return fmt.Errorf("database.apps_user is required when use_dbc is true")
		}
	} else {
		// Legacy direct connection method
		if c.Database.ConnectionString == "" {
			return fmt.Errorf("database.connection_string is required when use_dbc is false")
		}
		if c.Database.Username == "" {
			return fmt.Errorf("database.username is required when use_dbc is false")
		}
	}
	
	if c.EBS.BaseURL == "" {
		return fmt.Errorf("ebs.base_url is required")
	}
	
	// Set defaults
	if c.EBS.MaxUploadSize == 0 {
		c.EBS.MaxUploadSize = 100 * 1024 * 1024 // 100MB default
	}
	
	return nil
}

// LoadFromEnv loads the configuration based on the EBS_ENV environment variable.
// It first tries to load config.yaml, and if that doesn't exist, falls back to
// config.{EBS_ENV}.yaml (e.g., config.DEV.yaml, config.PROD.yaml).
func LoadFromEnv() (*Config, error) {
	env := os.Getenv("EBS_ENV")
	if env == "" {
		env = "DEV"
	}

	// Try to load config.yaml first, then fall back to environment-specific config
	configFile := "config.yaml"
	if _, err := os.Stat(configFile); errors.Is(err, fs.ErrNotExist) {
		configFile = fmt.Sprintf("config.%s.yaml", env)
	}

	return Load(configFile)
}

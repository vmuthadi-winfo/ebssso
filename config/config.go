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
	Environment  string          `yaml:"environment"`
	Server       ServerConfig    `yaml:"server"`
	OIDC         OIDCConfig      `yaml:"oidc"`
	Database     DatabaseConfig  `yaml:"database"`
	EBS          EBSConfig       `yaml:"ebs"`
	Logging      LoggingConfig   `yaml:"logging"`
	Session      SessionConfig   `yaml:"session"`
	Resilience   ResilienceConfig `yaml:"resilience"`
	Authorization AuthorizationConfig `yaml:"authorization"`
}

// ResilienceConfig holds resilience and failure handling settings
type ResilienceConfig struct {
	// Circuit breaker settings
	EnableCircuitBreaker  bool `yaml:"enable_circuit_breaker"`
	FailureThreshold      int  `yaml:"failure_threshold"`       // Number of failures before opening circuit (default: 5)
	SuccessThreshold      int  `yaml:"success_threshold"`       // Number of successes to close circuit (default: 2)
	CircuitTimeout        int  `yaml:"circuit_timeout"`         // Timeout before trying again in seconds (default: 60)
	
	// Retry settings
	EnableRetry           bool `yaml:"enable_retry"`
	MaxRetries            int  `yaml:"max_retries"`             // Max retry attempts (default: 3)
	RetryDelay            int  `yaml:"retry_delay"`             // Initial retry delay in seconds (default: 2)
	RetryBackoff          bool `yaml:"retry_backoff"`           // Use exponential backoff (default: true)
	
	// Timeout settings
	DatabaseTimeout       int  `yaml:"database_timeout"`        // DB operation timeout in seconds (default: 10)
	OIDCTimeout           int  `yaml:"oidc_timeout"`            // OIDC operation timeout in seconds (default: 30)
	EBSTimeout            int  `yaml:"ebs_timeout"`             // EBS operation timeout in seconds (default: 30)
	
	// Health check settings
	EnableHealthCheck     bool `yaml:"enable_health_check"`
	HealthCheckInterval   int  `yaml:"health_check_interval"`   // Health check interval in seconds (default: 60)
}

// AuthorizationConfig holds authorization and access control settings
type AuthorizationConfig struct {
	// Enable authorization checks
	EnableAuthorization   bool   `yaml:"enable_authorization"`
	
	// Access control mode
	Mode                  string `yaml:"mode"`                    // "permissive" or "strict" (default: "permissive")
	
	// Failed authorization action
	OnFailureAction       string `yaml:"on_failure_action"`       // "deny" or "log" (default: "deny")
	
	// Audit logging
	EnableAuditLog        bool   `yaml:"enable_audit_log"`
	AuditLogPath          string `yaml:"audit_log_path"`
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
	
	// Group-based access control
	GroupClaim         string   `yaml:"group_claim"`          // Claim containing user groups (e.g., "groups")
	AllowedGroups      []string `yaml:"allowed_groups"`       // List of allowed AD groups
	DeniedGroups       []string `yaml:"denied_groups"`        // List of denied AD groups (takes precedence)
	RequireGroupMatch  bool     `yaml:"require_group_match"`  // If true, user must be in at least one allowed group
	GroupMatchStrategy string   `yaml:"group_match_strategy"` // "any" or "all"
	
	// Connection and retry settings
	Timeout        int `yaml:"timeout"`         // Connection timeout in seconds (default: 30)
	MaxRetries     int `yaml:"max_retries"`     // Number of retries for transient failures (default: 3)
	RetryDelay     int `yaml:"retry_delay"`     // Delay between retries in seconds (default: 2)
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
	TimeoutMinutes        int    `yaml:"timeout_minutes"`
	CookieName            string `yaml:"cookie_name"`
	
	// Session synchronization with EBS
	SyncWithEBS           bool   `yaml:"sync_with_ebs"`           // Sync SSO timeout with EBS session timeout
	RefreshBeforeExpiry   int    `yaml:"refresh_before_expiry"`   // Minutes before expiry to trigger refresh (default: 5)
	EnableAutoRefresh     bool   `yaml:"enable_auto_refresh"`     // Enable automatic session refresh
	
	// Session monitoring and cleanup
	EnableSessionTracking bool   `yaml:"enable_session_tracking"` // Track active sessions
	CleanupInterval       int    `yaml:"cleanup_interval"`        // Cleanup interval in minutes (default: 10)
	MaxConcurrentSessions int    `yaml:"max_concurrent_sessions"` // Max concurrent sessions per user (0 = unlimited)
	
	// Warning and grace period
	ExpiryWarningMinutes  int    `yaml:"expiry_warning_minutes"`  // Minutes before expiry to warn user (default: 10)
	GracePeriodMinutes    int    `yaml:"grace_period_minutes"`    // Grace period after session expiry (default: 5)
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
	
	// Set OIDC defaults
	if c.OIDC.Timeout == 0 {
		c.OIDC.Timeout = 30
	}
	if c.OIDC.MaxRetries == 0 {
		c.OIDC.MaxRetries = 3
	}
	if c.OIDC.RetryDelay == 0 {
		c.OIDC.RetryDelay = 2
	}
	if c.OIDC.GroupMatchStrategy == "" {
		c.OIDC.GroupMatchStrategy = "any"
	}
	
	// Set Session defaults
	if c.Session.RefreshBeforeExpiry == 0 {
		c.Session.RefreshBeforeExpiry = 5
	}
	if c.Session.CleanupInterval == 0 {
		c.Session.CleanupInterval = 10
	}
	if c.Session.ExpiryWarningMinutes == 0 {
		c.Session.ExpiryWarningMinutes = 10
	}
	if c.Session.GracePeriodMinutes == 0 {
		c.Session.GracePeriodMinutes = 5
	}
	
	// Set Resilience defaults
	if c.Resilience.FailureThreshold == 0 {
		c.Resilience.FailureThreshold = 5
	}
	if c.Resilience.SuccessThreshold == 0 {
		c.Resilience.SuccessThreshold = 2
	}
	if c.Resilience.CircuitTimeout == 0 {
		c.Resilience.CircuitTimeout = 60
	}
	if c.Resilience.MaxRetries == 0 {
		c.Resilience.MaxRetries = 3
	}
	if c.Resilience.RetryDelay == 0 {
		c.Resilience.RetryDelay = 2
	}
	if c.Resilience.DatabaseTimeout == 0 {
		c.Resilience.DatabaseTimeout = 10
	}
	if c.Resilience.OIDCTimeout == 0 {
		c.Resilience.OIDCTimeout = 30
	}
	if c.Resilience.EBSTimeout == 0 {
		c.Resilience.EBSTimeout = 30
	}
	if c.Resilience.HealthCheckInterval == 0 {
		c.Resilience.HealthCheckInterval = 60
	}
	
	// Set Authorization defaults
	if c.Authorization.Mode == "" {
		c.Authorization.Mode = "permissive"
	}
	if c.Authorization.OnFailureAction == "" {
		c.Authorization.OnFailureAction = "deny"
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

package onepassword

// Config holds the configuration for the 1Password Connect provider
type Config struct {
	// Host is the 1Password Connect server URL (e.g., "http://localhost:8080")
	Host string

	// Token is the 1Password Connect service account token
	Token string

	// Vault is the default vault to use (optional)
	Vault string

	// Timeout is the HTTP request timeout in seconds (default: 30)
	Timeout int

	// MaxRetries is the maximum number of retry attempts for failed requests (default: 3)
	MaxRetries int
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.Host == "" {
		return ErrInvalidConfig("host is required")
	}
	if c.Token == "" {
		return ErrInvalidConfig("token is required")
	}
	if c.Timeout <= 0 {
		c.Timeout = defaultTimeout
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = defaultMaxRetries
	}
	return nil
}

// ErrInvalidConfig represents an invalid configuration error
type ErrInvalidConfig string

func (e ErrInvalidConfig) Error() string {
	return string(e)
}

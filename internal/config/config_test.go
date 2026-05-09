package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfigFromYAML(t *testing.T) {
	yamlContent := `
server:
  host: "127.0.0.1"
  port: 9090
  tls:
    cert_file: "/etc/cert.pem"
    key_file: "/etc/key.pem"

auth:
  type: "api_key"
  api_keys:
    - key: "test_key_123"
      client_id: "test-client"
      allowed_paths: ["dev/*"]

storage:
  type: "sqlite"
  sqlite:
    path: "/tmp/test.db"

audit:
  type: "file"
  file:
    path: "/tmp/audit.log"
`
	tmpFile, err := os.CreateTemp("", "config*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.WriteString(yamlContent); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}
	tmpFile.Close()

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "127.0.0.1" {
		t.Errorf("Server.Host = %v, want %v", cfg.Server.Host, "127.0.0.1")
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("Server.Port = %v, want %v", cfg.Server.Port, 9090)
	}
	if cfg.Server.TLS.CertFile != "/etc/cert.pem" {
		t.Errorf("Server.TLS.CertFile = %v, want %v", cfg.Server.TLS.CertFile, "/etc/cert.pem")
	}
	if cfg.Auth.Type != "api_key" {
		t.Errorf("Auth.Type = %v, want %v", cfg.Auth.Type, "api_key")
	}
	if len(cfg.Auth.APIKeys) != 1 {
		t.Errorf("len(Auth.APIKeys) = %v, want %v", len(cfg.Auth.APIKeys), 1)
	}
	if cfg.Storage.Type != "sqlite" {
		t.Errorf("Storage.Type = %v, want %v", cfg.Storage.Type, "sqlite")
	}
	if cfg.Audit.Type != "file" {
		t.Errorf("Audit.Type = %v, want %v", cfg.Audit.Type, "file")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() with empty path error = %v", err)
	}

	if cfg.Server.Host != "0.0.0.0" {
		t.Errorf("Server.Host default = %v, want %v", cfg.Server.Host, "0.0.0.0")
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port default = %v, want %v", cfg.Server.Port, 8080)
	}
	if cfg.Storage.Type != "sqlite" {
		t.Errorf("Storage.Type default = %v, want %v", cfg.Storage.Type, "sqlite")
	}
}

func TestLoadConfigFromEnv(t *testing.T) {
	os.Setenv("TRAVERSE_SERVER_HOST", "192.168.1.1")
	os.Setenv("TRAVERSE_SERVER_PORT", "9000")
	os.Setenv("TRAVERSE_STORAGE_TYPE", "postgres")
	defer func() {
		os.Unsetenv("TRAVERSE_SERVER_HOST")
		os.Unsetenv("TRAVERSE_SERVER_PORT")
		os.Unsetenv("TRAVERSE_STORAGE_TYPE")
	}()

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Server.Host != "192.168.1.1" {
		t.Errorf("Server.Host from env = %v, want %v", cfg.Server.Host, "192.168.1.1")
	}
	if cfg.Server.Port != 9000 {
		t.Errorf("Server.Port from env = %v, want %v", cfg.Server.Port, 9000)
	}
	if cfg.Storage.Type != "postgres" {
		t.Errorf("Storage.Type from env = %v, want %v", cfg.Storage.Type, "postgres")
	}
}

func TestRequestValidation(t *testing.T) {
	tests := []struct {
		name    string
		request SecretRequest
		wantErr bool
	}{
		{
			name: "valid request",
			request: SecretRequest{
				SecretPath:        "prod/api-keys/stripe",
				Reason:            "Deploying payment feature",
				RequestedDuration: time.Hour,
			},
			wantErr: false,
		},
		{
			name: "invalid path with space",
			request: SecretRequest{
				SecretPath:        "prod/api keys/stripe",
				Reason:            "Deploying payment feature",
				RequestedDuration: time.Hour,
			},
			wantErr: true,
		},
		{
			name: "reason too short",
			request: SecretRequest{
				SecretPath:        "prod/api-keys/stripe",
				Reason:            "short",
				RequestedDuration: time.Hour,
			},
			wantErr: true,
		},
		{
			name: "empty path",
			request: SecretRequest{
				SecretPath:        "",
				Reason:            "Deploying payment feature",
				RequestedDuration: time.Hour,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRequest(tt.request)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRequest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

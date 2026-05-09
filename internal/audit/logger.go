package audit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Logger interface {
	Log(event Event) error
	Close() error
}

type Event struct {
	Timestamp   time.Time              `json:"timestamp"`
	Type        string                 `json:"type"`
	RequestID   string                 `json:"request_id,omitempty"`
	ClientID    string                 `json:"client_id,omitempty"`
	SecretPath  string                 `json:"secret_path,omitempty"`
	Identity    string                 `json:"identity,omitempty"`
	Description string                 `json:"description,omitempty"`
	Details     map[string]interface{} `json:"details,omitempty"`
}

const (
	EventRequestCreated   = "REQUEST_CREATED"
	EventNotificationSent = "NOTIFICATION_SENT"
	EventRequestApproved  = "REQUEST_APPROVED"
	EventRequestDenied    = "REQUEST_DENIED"
	EventRequestExpired   = "REQUEST_EXPIRED"
	EventSecretAccessed   = "SECRET_ACCESSED"
	EventTokenIssued      = "TOKEN_ISSUED"
	EventTokenRevoked     = "TOKEN_REVOKED"
)

type FileLogger struct {
	file *os.File
}

func NewFileLogger(path string) (Logger, error) {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open audit log file: %w", err)
	}

	return &FileLogger{file: file}, nil
}

func (f *FileLogger) Log(event Event) error {
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	if _, err := f.file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("failed to write audit event: %w", err)
	}

	return nil
}

func (f *FileLogger) Close() error {
	return f.file.Close()
}

type WebhookLogger struct {
	url   string
	token string
}

func NewWebhookLogger(url, token string) Logger {
	return &WebhookLogger{
		url:   url,
		token: token,
	}
}

func (w *WebhookLogger) Log(event Event) error {
	// In a real implementation, this would POST to the webhook
	// For now, it's a placeholder
	return nil
}

func (w *WebhookLogger) Close() error {
	return nil
}

func NewLogger(loggerType string, config map[string]string) (Logger, error) {
	switch loggerType {
	case "file":
		path := config["path"]
		if path == "" {
			path = "/var/log/traverse/audit.log"
		}
		return NewFileLogger(path)
	case "webhook":
		url := config["url"]
		token := config["token"]
		return NewWebhookLogger(url, token), nil
	default:
		return nil, fmt.Errorf("unsupported audit logger type: %s", loggerType)
	}
}

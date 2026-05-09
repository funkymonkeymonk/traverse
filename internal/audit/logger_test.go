package audit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileLogger_LogEvent(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("NewFileLogger() error = %v", err)
	}
	defer logger.Close()

	event := Event{
		Timestamp:   time.Now(),
		Type:        "REQUEST_CREATED",
		RequestID:   "req_test_123",
		ClientID:    "agent-001",
		SecretPath:  "prod/api-keys/stripe",
		Description: "Secret request created",
	}

	if err := logger.Log(event); err != nil {
		t.Fatalf("Log() error = %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	var loggedEvent Event
	if err := json.Unmarshal(content, &loggedEvent); err != nil {
		t.Fatalf("failed to unmarshal log entry: %v", err)
	}

	if loggedEvent.Type != event.Type {
		t.Errorf("Logged event Type = %v, want %v", loggedEvent.Type, event.Type)
	}
	if loggedEvent.RequestID != event.RequestID {
		t.Errorf("Logged event RequestID = %v, want %v", loggedEvent.RequestID, event.RequestID)
	}
	if loggedEvent.ClientID != event.ClientID {
		t.Errorf("Logged event ClientID = %v, want %v", loggedEvent.ClientID, event.ClientID)
	}
}

func TestFileLogger_LogMultipleEvents(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.log")

	logger, err := NewFileLogger(logPath)
	if err != nil {
		t.Fatalf("NewFileLogger() error = %v", err)
	}
	defer logger.Close()

	events := []Event{
		{Type: "REQUEST_CREATED", RequestID: "req_1", ClientID: "agent-001"},
		{Type: "NOTIFICATION_SENT", RequestID: "req_1", ClientID: "agent-001"},
		{Type: "REQUEST_APPROVED", RequestID: "req_1", ClientID: "agent-001"},
	}

	for _, event := range events {
		event.Timestamp = time.Now()
		if err := logger.Log(event); err != nil {
			t.Fatalf("Log() error = %v", err)
		}
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	lines := 0
	for _, line := range splitLines(content) {
		if len(line) > 0 {
			lines++
			var event Event
			if err := json.Unmarshal(line, &event); err != nil {
				t.Fatalf("failed to unmarshal log entry: %v", err)
			}
		}
	}

	if lines != 3 {
		t.Errorf("Expected 3 log lines, got %d", lines)
	}
}

func TestEventTypes(t *testing.T) {
	expectedTypes := []string{
		"REQUEST_CREATED",
		"NOTIFICATION_SENT",
		"REQUEST_APPROVED",
		"REQUEST_DENIED",
		"REQUEST_EXPIRED",
		"SECRET_ACCESSED",
		"TOKEN_ISSUED",
		"TOKEN_REVOKED",
	}

	for _, eventType := range expectedTypes {
		event := Event{
			Timestamp: time.Now(),
			Type:      eventType,
		}
		if event.Type == "" {
			t.Errorf("Event type %s is empty", eventType)
		}
	}
}

func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}

package notification

import (
	"context"
	"testing"
	"time"
)

type mockNotifier struct {
	name          string
	configured    bool
	notifications []Notification
	notifyError   error
	closeCalled   bool
}

func (m *mockNotifier) Name() string {
	return m.name
}

func (m *mockNotifier) Configure(config map[string]interface{}) error {
	m.configured = true
	return nil
}

func (m *mockNotifier) Notify(ctx context.Context, notification Notification) error {
	if m.notifyError != nil {
		return m.notifyError
	}
	m.notifications = append(m.notifications, notification)
	return nil
}

func (m *mockNotifier) Close() error {
	m.closeCalled = true
	return nil
}

func TestNotifierInterface(t *testing.T) {
	t.Run("Notifier implements all required methods", func(t *testing.T) {
		notifier := &mockNotifier{name: "test-notifier"}

		if notifier.Name() != "test-notifier" {
			t.Errorf("expected name 'test-notifier', got '%s'", notifier.Name())
		}

		err := notifier.Configure(map[string]interface{}{"key": "value"})
		if err != nil {
			t.Errorf("unexpected error during configure: %v", err)
		}
		if !notifier.configured {
			t.Error("expected notifier to be configured")
		}

		ctx := context.Background()
		notification := Notification{
			ID:        "test-id",
			Type:      TypeApprovalRequest,
			Title:     "Test Notification",
			Message:   "Test message",
			Priority:  PriorityHigh,
			Recipient: "user@example.com",
			Context: NotificationContext{
				RequestID:   "req-123",
				SecretPath:  "vault/item",
				RequestedBy: "requester@example.com",
				RequestedAt: time.Now(),
			},
		}
		err = notifier.Notify(ctx, notification)
		if err != nil {
			t.Errorf("unexpected error during notify: %v", err)
		}
		if len(notifier.notifications) != 1 {
			t.Errorf("expected 1 notification, got %d", len(notifier.notifications))
		}

		err = notifier.Close()
		if err != nil {
			t.Errorf("unexpected error during close: %v", err)
		}
		if !notifier.closeCalled {
			t.Error("expected Close() to be called")
		}
	})
}

func TestNotificationTypes(t *testing.T) {
	t.Run("notification types are defined", func(t *testing.T) {
		if TypeApprovalRequest != "approval_request" {
			t.Errorf("expected TypeApprovalRequest to be 'approval_request', got '%s'", TypeApprovalRequest)
		}
		if TypeApprovalReminder != "approval_reminder" {
			t.Errorf("expected TypeApprovalReminder to be 'approval_reminder', got '%s'", TypeApprovalReminder)
		}
		if TypeApprovalResolved != "approval_resolved" {
			t.Errorf("expected TypeApprovalResolved to be 'approval_resolved', got '%s'", TypeApprovalResolved)
		}
		if TypeSystemAlert != "system_alert" {
			t.Errorf("expected TypeSystemAlert to be 'system_alert', got '%s'", TypeSystemAlert)
		}
	})
}

func TestNotificationPriorities(t *testing.T) {
	t.Run("notification priorities are ordered correctly", func(t *testing.T) {
		if PriorityLow >= PriorityMedium {
			t.Error("PriorityLow should be less than PriorityMedium")
		}
		if PriorityMedium >= PriorityHigh {
			t.Error("PriorityMedium should be less than PriorityHigh")
		}
		if PriorityHigh >= PriorityCritical {
			t.Error("PriorityHigh should be less than PriorityCritical")
		}
	})
}

func TestNotificationContext(t *testing.T) {
	t.Run("notification context contains required fields", func(t *testing.T) {
		now := time.Now()
		ctx := NotificationContext{
			RequestID:   "req-123",
			SecretPath:  "vault/item",
			RequestedBy: "requester@example.com",
			RequestedAt: now,
			ExpiresAt:   now.Add(1 * time.Hour),
			Metadata: map[string]string{
				"environment": "production",
			},
		}

		if ctx.RequestID != "req-123" {
			t.Errorf("expected request_id 'req-123', got '%s'", ctx.RequestID)
		}
		if ctx.SecretPath != "vault/item" {
			t.Errorf("expected secret_path 'vault/item', got '%s'", ctx.SecretPath)
		}
		if ctx.RequestedBy != "requester@example.com" {
			t.Errorf("expected requested_by 'requester@example.com', got '%s'", ctx.RequestedBy)
		}
		if len(ctx.Metadata) != 1 {
			t.Errorf("expected 1 metadata entry, got %d", len(ctx.Metadata))
		}
	})
}

func TestResponseCallbacks(t *testing.T) {
	t.Run("ResponseCallback contains required fields", func(t *testing.T) {
		callback := ResponseCallback{
			NotificationID: "notif-123",
			RequestID:      "req-123",
			Response:       ResponseApproved,
			RespondedBy:    "approver@example.com",
			RespondedAt:    time.Now(),
			Reason:         "Approved for production deployment",
		}

		if callback.NotificationID != "notif-123" {
			t.Errorf("expected notification_id 'notif-123', got '%s'", callback.NotificationID)
		}
		if callback.Response != ResponseApproved {
			t.Errorf("expected response '%s', got '%s'", ResponseApproved, callback.Response)
		}
		if callback.RespondedBy != "approver@example.com" {
			t.Errorf("expected responded_by 'approver@example.com', got '%s'", callback.RespondedBy)
		}
	})

	t.Run("response types are defined", func(t *testing.T) {
		if ResponseApproved != "approved" {
			t.Errorf("expected ResponseApproved to be 'approved', got '%s'", ResponseApproved)
		}
		if ResponseDenied != "denied" {
			t.Errorf("expected ResponseDenied to be 'denied', got '%s'", ResponseDenied)
		}
		if ResponseTimeout != "timeout" {
			t.Errorf("expected ResponseTimeout to be 'timeout', got '%s'", ResponseTimeout)
		}
	})
}

func TestErrors(t *testing.T) {
	t.Run("Error constants are defined", func(t *testing.T) {
		if ErrNotifierNotConfigured == nil {
			t.Error("ErrNotifierNotConfigured should not be nil")
		}
		if ErrNotificationFailed == nil {
			t.Error("ErrNotificationFailed should not be nil")
		}
		if ErrInvalidPriority == nil {
			t.Error("ErrInvalidPriority should not be nil")
		}
		if ErrNotifierNotFound == nil {
			t.Error("ErrNotifierNotFound should not be nil")
		}
	})

	t.Run("Error messages are descriptive", func(t *testing.T) {
		if ErrNotifierNotConfigured.Error() != "notifier not configured" {
			t.Errorf("unexpected error message: %s", ErrNotifierNotConfigured.Error())
		}
		if ErrNotificationFailed.Error() != "notification delivery failed" {
			t.Errorf("unexpected error message: %s", ErrNotificationFailed.Error())
		}
	})
}

func TestNotificationCreation(t *testing.T) {
	t.Run("creates notification with required fields", func(t *testing.T) {
		notification := Notification{
			ID:        "test-id",
			Type:      TypeApprovalRequest,
			Title:     "Approval Required",
			Message:   "Please approve access to secret",
			Priority:  PriorityHigh,
			Recipient: "approver@example.com",
		}

		if notification.ID != "test-id" {
			t.Errorf("expected id 'test-id', got '%s'", notification.ID)
		}
		if notification.Type != TypeApprovalRequest {
			t.Errorf("expected type '%s', got '%s'", TypeApprovalRequest, notification.Type)
		}
		if notification.Priority != PriorityHigh {
			t.Errorf("expected priority %d, got %d", PriorityHigh, notification.Priority)
		}
	})

	t.Run("creates notification with full context", func(t *testing.T) {
		now := time.Now()
		notification := Notification{
			ID:        "test-id",
			Type:      TypeApprovalRequest,
			Title:     "Approval Required",
			Message:   "Please approve access to secret",
			Priority:  PriorityHigh,
			Recipient: "approver@example.com",
			Context: NotificationContext{
				RequestID:   "req-123",
				SecretPath:  "production/api-key",
				RequestedBy: "developer@example.com",
				RequestedAt: now,
				ExpiresAt:   now.Add(30 * time.Minute),
				Metadata: map[string]string{
					"client":      "cli-tool",
					"environment": "production",
				},
			},
			Actions: []Action{
				{Type: ActionApprove, Label: "Approve", URL: "https://api.example.com/approve"},
				{Type: ActionDeny, Label: "Deny", URL: "https://api.example.com/deny"},
			},
		}

		if notification.Context.SecretPath != "production/api-key" {
			t.Errorf("expected secret_path 'production/api-key', got '%s'", notification.Context.SecretPath)
		}
		if len(notification.Actions) != 2 {
			t.Errorf("expected 2 actions, got %d", len(notification.Actions))
		}
		if notification.Actions[0].Type != ActionApprove {
			t.Errorf("expected first action type '%s', got '%s'", ActionApprove, notification.Actions[0].Type)
		}
	})
}

func TestActionTypes(t *testing.T) {
	t.Run("action types are defined", func(t *testing.T) {
		if ActionApprove != "approve" {
			t.Errorf("expected ActionApprove to be 'approve', got '%s'", ActionApprove)
		}
		if ActionDeny != "deny" {
			t.Errorf("expected ActionDeny to be 'deny', got '%s'", ActionDeny)
		}
		if ActionView != "view" {
			t.Errorf("expected ActionView to be 'view', got '%s'", ActionView)
		}
	})
}

func TestNotificationValidation(t *testing.T) {
	t.Run("Validate checks required fields", func(t *testing.T) {
		tests := []struct {
			name         string
			notification Notification
			wantErr      bool
			errMsg       string
		}{
			{
				name: "valid notification",
				notification: Notification{
					ID:        "test-id",
					Type:      TypeApprovalRequest,
					Title:     "Test",
					Message:   "Test message",
					Priority:  PriorityHigh,
					Recipient: "user@example.com",
				},
				wantErr: false,
			},
			{
				name: "missing ID",
				notification: Notification{
					Type:      TypeApprovalRequest,
					Title:     "Test",
					Message:   "Test message",
					Priority:  PriorityHigh,
					Recipient: "user@example.com",
				},
				wantErr: true,
				errMsg:  "notification ID is required",
			},
			{
				name: "missing type",
				notification: Notification{
					ID:        "test-id",
					Title:     "Test",
					Message:   "Test message",
					Priority:  PriorityHigh,
					Recipient: "user@example.com",
				},
				wantErr: true,
				errMsg:  "notification type is required",
			},
			{
				name: "missing recipient",
				notification: Notification{
					ID:       "test-id",
					Type:     TypeApprovalRequest,
					Title:    "Test",
					Message:  "Test message",
					Priority: PriorityHigh,
				},
				wantErr: true,
				errMsg:  "recipient is required",
			},
			{
				name: "invalid priority",
				notification: Notification{
					ID:        "test-id",
					Type:      TypeApprovalRequest,
					Title:     "Test",
					Message:   "Test message",
					Priority:  999,
					Recipient: "user@example.com",
				},
				wantErr: true,
				errMsg:  "invalid priority level",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.notification.Validate()
				if tt.wantErr {
					if err == nil {
						t.Errorf("expected error, got nil")
					} else if err.Error() != tt.errMsg {
						t.Errorf("expected error '%s', got '%s'", tt.errMsg, err.Error())
					}
				} else {
					if err != nil {
						t.Errorf("unexpected error: %v", err)
					}
				}
			})
		}
	})
}

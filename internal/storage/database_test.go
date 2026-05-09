package storage

import (
	"testing"
	"time"
)

func TestDatabase_CreateAndGetRequest(t *testing.T) {
	db, err := New("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	req := &SecretRequest{
		ID:                "req_test_123",
		ClientID:          "agent-001",
		SecretPath:        "prod/api-keys/stripe",
		Reason:            "Deploying payment feature",
		Status:            "pending",
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(5 * time.Minute),
		RequiredApprovals: 1,
	}

	if err := db.CreateRequest(req); err != nil {
		t.Fatalf("CreateRequest() error = %v", err)
	}

	retrieved, err := db.GetRequest(req.ID)
	if err != nil {
		t.Fatalf("GetRequest() error = %v", err)
	}

	if retrieved.ID != req.ID {
		t.Errorf("GetRequest() ID = %v, want %v", retrieved.ID, req.ID)
	}
	if retrieved.ClientID != req.ClientID {
		t.Errorf("GetRequest() ClientID = %v, want %v", retrieved.ClientID, req.ClientID)
	}
	if retrieved.SecretPath != req.SecretPath {
		t.Errorf("GetRequest() SecretPath = %v, want %v", retrieved.SecretPath, req.SecretPath)
	}
}

func TestDatabase_UpdateRequestStatus(t *testing.T) {
	db, err := New("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	req := &SecretRequest{
		ID:                "req_test_456",
		ClientID:          "agent-001",
		SecretPath:        "prod/api-keys/stripe",
		Reason:            "Deploying payment feature",
		Status:            "pending",
		CreatedAt:         time.Now(),
		ExpiresAt:         time.Now().Add(5 * time.Minute),
		RequiredApprovals: 1,
	}

	if err := db.CreateRequest(req); err != nil {
		t.Fatalf("CreateRequest() error = %v", err)
	}

	if err := db.UpdateRequestStatus(req.ID, "approved"); err != nil {
		t.Fatalf("UpdateRequestStatus() error = %v", err)
	}

	retrieved, err := db.GetRequest(req.ID)
	if err != nil {
		t.Fatalf("GetRequest() after update error = %v", err)
	}

	if retrieved.Status != "approved" {
		t.Errorf("Status after update = %v, want %v", retrieved.Status, "approved")
	}
}

func TestDatabase_ListRequests(t *testing.T) {
	db, err := New("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	for i := 0; i < 5; i++ {
		req := &SecretRequest{
			ID:                "req_" + string(rune('a'+i)),
			ClientID:          "agent-001",
			SecretPath:        "prod/api-keys/stripe",
			Reason:            "Deploying payment feature",
			Status:            "pending",
			CreatedAt:         time.Now(),
			ExpiresAt:         time.Now().Add(5 * time.Minute),
			RequiredApprovals: 1,
		}
		if err := db.CreateRequest(req); err != nil {
			t.Fatalf("CreateRequest() error = %v", err)
		}
	}

	requests, total, err := db.ListRequests(ListFilters{}, 10, 0)
	if err != nil {
		t.Fatalf("ListRequests() error = %v", err)
	}

	if total != 5 {
		t.Errorf("ListRequests() total = %v, want %v", total, 5)
	}
	if len(requests) != 5 {
		t.Errorf("ListRequests() count = %v, want %v", len(requests), 5)
	}
}

func TestDatabase_ListRequestsWithFilter(t *testing.T) {
	db, err := New("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer db.Close()

	for i := 0; i < 3; i++ {
		req := &SecretRequest{
			ID:                "req_pending_" + string(rune('a'+i)),
			ClientID:          "agent-001",
			SecretPath:        "prod/api-keys/stripe",
			Reason:            "Deploying payment feature",
			Status:            "pending",
			CreatedAt:         time.Now(),
			ExpiresAt:         time.Now().Add(5 * time.Minute),
			RequiredApprovals: 1,
		}
		if err := db.CreateRequest(req); err != nil {
			t.Fatalf("CreateRequest() error = %v", err)
		}
	}

	for i := 0; i < 2; i++ {
		req := &SecretRequest{
			ID:                "req_approved_" + string(rune('a'+i)),
			ClientID:          "agent-001",
			SecretPath:        "prod/api-keys/stripe",
			Reason:            "Deploying payment feature",
			Status:            "approved",
			CreatedAt:         time.Now(),
			ExpiresAt:         time.Now().Add(5 * time.Minute),
			RequiredApprovals: 1,
		}
		if err := db.CreateRequest(req); err != nil {
			t.Fatalf("CreateRequest() error = %v", err)
		}
	}

	requests, total, err := db.ListRequests(ListFilters{Status: "pending"}, 10, 0)
	if err != nil {
		t.Fatalf("ListRequests() error = %v", err)
	}

	if total != 3 {
		t.Errorf("ListRequests() total with pending filter = %v, want %v", total, 3)
	}
	if len(requests) != 3 {
		t.Errorf("ListRequests() count with pending filter = %v, want %v", len(requests), 3)
	}
}

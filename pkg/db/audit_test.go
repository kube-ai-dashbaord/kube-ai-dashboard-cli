package db

import (
	"os"
	"testing"
)

func TestAuditLogging(t *testing.T) {
	dbPath := "test_audit.db"
	defer os.Remove(dbPath)

	if err := Init(dbPath); err != nil {
		t.Fatalf("Failed to init DB: %v", err)
	}
	defer Close()

	entry := AuditEntry{
		User:    "Tester",
		Action:  "TEST_ACTION",
		Details: "Test Details",
	}

	if err := RecordAudit(entry); err != nil {
		t.Fatalf("Failed to record audit: %v", err)
	}

	logs, err := GetAuditLogs()
	if err != nil {
		t.Fatalf("Failed to get logs: %v", err)
	}

	if len(logs) == 0 {
		t.Errorf("Expected logs, got none")
	}

	found := false
	for _, log := range logs {
		if log["action"] == "TEST_ACTION" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Test action not found in logs")
	}
}

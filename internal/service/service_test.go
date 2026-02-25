package service

import (
	"testing"
)

func TestNew(t *testing.T) {
	mgr, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	if mgr == nil {
		t.Fatal("New() returned nil manager")
	}
}

func TestStatus_NotInstalled(t *testing.T) {
	mgr, err := New()
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	// We can't reliably test install/uninstall in CI, but we can call Status
	st, err := mgr.Status()
	if err != nil {
		t.Fatalf("Status() failed: %v", err)
	}

	// Status should return a non-empty message
	if st.Message == "" {
		t.Error("Status() returned empty message")
	}
}

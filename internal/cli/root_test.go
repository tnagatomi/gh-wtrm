package cli

import "testing"

func TestNewRootCmdUse(t *testing.T) {
	cmd := NewRootCmd()
	if cmd.Use != "wtrm" {
		t.Errorf("Use: got %q, want %q", cmd.Use, "wtrm")
	}
}

func TestNewRootCmdVersion(t *testing.T) {
	cmd := NewRootCmd()
	if cmd.Version == "" {
		t.Error("Version should not be empty")
	}
}

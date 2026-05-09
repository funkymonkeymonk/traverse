package main

import (
	"testing"
)

func TestMainFunction(t *testing.T) {
	// Main function should be tested via integration tests
	// This test verifies the package compiles correctly
	t.Log("Main package compiled successfully")
}

func TestVersion(t *testing.T) {
	// Version is set to "dev" by default, will be overridden at build time
	expectedVersion := "dev"
	if version != expectedVersion {
		t.Errorf("version = %v, want %v", version, expectedVersion)
	}
}

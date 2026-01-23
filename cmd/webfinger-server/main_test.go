package main

import (
	"github.com/0dayfall/webfinger-server/internal/config"
	"testing"
)

// TestConfigLoad tests that configuration can be loaded
func TestConfigLoad(t *testing.T) {
	_, err := config.Load()
	if err != nil {
		t.Fatalf("Failed to load configuration: %v", err)
	}
}

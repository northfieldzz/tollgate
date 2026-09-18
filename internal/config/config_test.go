package config

import (
	"os"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"/openapi.json", "/openapi.json"},
		{"openapi.json", "/openapi.json"},
		{"/docs", "/docs"},
		{"docs", "/docs"},
	}

	for _, tt := range tests {
		actual := normalizePath(tt.input)
		if actual != tt.expected {
			t.Errorf("normalizePath(%q) = %q, want %q", tt.input, actual, tt.expected)
		}
	}
}

func TestLoad_DefaultAndPath(t *testing.T) {
	os.Setenv("OPENAPI_PATH", "/openapi.json")
	os.Setenv("DOCS_PATH", "docs")
	defer func() {
		os.Unsetenv("OPENAPI_PATH")
		os.Unsetenv("DOCS_PATH")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.OpenAPIPath != "/openapi.json" {
		t.Errorf("expected /openapi.json, got %s", cfg.OpenAPIPath)
	}
	if cfg.DocsPath != "/docs" {
		t.Errorf("expected /docs, got %s", cfg.DocsPath)
	}
}

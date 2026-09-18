package usecase

import (
	"strings"
	"testing"
)

func TestGenerateRawKey(t *testing.T) {
	key, err := GenerateRawKey()
	if err != nil {
		t.Fatalf("GenerateRawKey error: %v", err)
	}

	if !strings.HasPrefix(key, "tlge-live-") {
		t.Errorf("expected prefix 'tlge-live-', got: %s", key)
	}
	// "tlge-live-" (10文字) + hex32文字 = 42文字
	if len(key) != 42 {
		t.Errorf("expected key length 42, got: %d (%s)", len(key), key)
	}

	// 2回目のキーと重複しないか検証
	key2, _ := GenerateRawKey()
	if key == key2 {
		t.Errorf("generated keys should be unique")
	}
}

func TestHashKey(t *testing.T) {
	raw := "tlge-live-0123456789abcdef0123456789abcdef"
	h1 := HashKey(raw)
	h2 := HashKey(raw)

	if h1 != h2 {
		t.Errorf("hashes should be deterministic, got %s and %s", h1, h2)
	}
	if len(h1) != 64 {
		t.Errorf("expected sha256 hex length 64, got %d", len(h1))
	}
}

func TestMatchScope(t *testing.T) {
	cases := []struct {
		required string
		allowed  []string
		expected bool
	}{
		{"ai:workflows:execute", []string{"ai:workflows:execute"}, true},
		{"ai:workflows:execute", []string{"ai:*"}, true},
		{"ai:workflows:execute", []string{"*"}, true},
		{"ai:workflows:execute", []string{"mcp:tools:execute"}, false},
		{"llm:chat:completions", []string{"llm:*", "ai:*"}, true},
		{"llm:chat:completions", []string{"mcp:*"}, false},
		{"", []string{"mcp:tools:execute"}, true}, // 要求なしは通す
	}

	for _, tc := range cases {
		got := matchScope(tc.required, tc.allowed)
		if got != tc.expected {
			t.Errorf("matchScope(%q, %v) = %v; want %v", tc.required, tc.allowed, got, tc.expected)
		}
	}
}

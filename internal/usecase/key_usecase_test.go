package usecase

import (
	"errors"
	"regexp"
	"strings"
	"testing"
)


func TestExtractKeyPrefix(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "valid long key",
			input:    "tlge-live-8f9c1234567890abcdef",
			expected: "tlge-live-8f9c",
		},
		{
			name:     "exact length key",
			input:    "tlge-live-1234",
			expected: "tlge-live-1234",
		},
		{
			name:     "too short key",
			input:    "tlge",
			expected: "tlge",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractKeyPrefix(tc.input)
			if got != tc.expected {
				t.Errorf("ExtractKeyPrefix(%q) = %q; want %q", tc.input, got, tc.expected)
			}
		})
	}
}

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

	// verify format with regex (tlge-live- followed by 32 hex chars)
	matched, _ := regexp.MatchString(`^tlge-live-[a-f0-9]{32}$`, key)
	if !matched {
		t.Errorf("key does not match expected format, got: %s", key)
	}

	// 2回目のキーと重複しないか検証
	key2, _ := GenerateRawKey()
	if key == key2 {
		t.Errorf("generated keys should be unique")
	}
}

func TestGenerateRawKey_Error(t *testing.T) {
	// mock cryptoRandRead to return an error
	originalRandRead := cryptoRandRead
	cryptoRandRead = func(b []byte) (int, error) {
		return 0, errors.New("simulated rand read error")
	}
	defer func() {
		cryptoRandRead = originalRandRead
	}()

	key, err := GenerateRawKey()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "crypto rand read error") {
		t.Errorf("expected error to contain 'crypto rand read error', got: %v", err)
	}
	if key != "" {
		t.Errorf("expected empty key on error, got: %s", key)
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

package usecase

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/northfieldzz/tollgate/internal/domain/entity"
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

	t.Run("deterministic hash with secret", func(t *testing.T) {
		t.Setenv("API_KEY_HASH_SECRET", "test-secret")
		h1 := HashKey(raw)
		h2 := HashKey(raw)

		if h1 != h2 {
			t.Errorf("hashes should be deterministic, got %s and %s", h1, h2)
		}
		if len(h1) != 64 {
			t.Errorf("expected hmac-sha256 hex length 64, got %d", len(h1))
		}

		mac := hmac.New(sha256.New, []byte("test-secret"))
		mac.Write([]byte(raw))
		expected := hex.EncodeToString(mac.Sum(nil))

		if h1 != expected {
			t.Errorf("expected hash %s, got %s", expected, h1)
		}
	})

	t.Run("different secret produces different hash", func(t *testing.T) {
		t.Setenv("API_KEY_HASH_SECRET", "secret-a")
		hashA := HashKey(raw)

		t.Setenv("API_KEY_HASH_SECRET", "secret-b")
		hashB := HashKey(raw)

		if hashA == hashB {
			t.Errorf("expected different hashes for different secrets, got %s", hashA)
		}
	})

	t.Run("fallback when secret not set", func(t *testing.T) {
		t.Setenv("API_KEY_HASH_SECRET", "")
		h := HashKey(raw)
		if len(h) != 64 {
			t.Errorf("expected hash length 64, got %d", len(h))
		}
	})
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

func TestExtractKeyPrefix(t *testing.T) {
	cases := []struct {
		name     string
		rawKey   string
		expected string
	}{
		{
			name:     "normal length key",
			rawKey:   "tlge-live-8f9c1234567890abcdef",
			expected: "tlge-live-8f9c",
		},
		{
			name:     "exact length key",
			rawKey:   "tlge-live-8f9c",
			expected: "tlge-live-8f9c",
		},
		{
			name:     "short length key",
			rawKey:   "tlge-live-",
			expected: "tlge-live-",
		},
		{
			name:     "empty string",
			rawKey:   "",
			expected: "",
		},
		{
			name:     "random string long",
			rawKey:   "12345678901234567890",
			expected: "12345678901234",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ExtractKeyPrefix(tc.rawKey)
			if got != tc.expected {
				t.Errorf("ExtractKeyPrefix(%q) = %q; want %q", tc.rawKey, got, tc.expected)
			}
		})
	}
}

func TestCreateKeyInput_Validate(t *testing.T) {
	cases := []struct {
		name      string
		input     entity.CreateKeyInput
		expectErr bool
	}{
		{
			name: "tenant only (missing service_id)",
			input: entity.CreateKeyInput{
				Name:     "Tenant Key",
				TenantID: "tenant-001",
				Scopes:   []string{"llm:*"},
			},
			expectErr: true,
		},
		{
			name: "service only",
			input: entity.CreateKeyInput{
				Name:      "Service Key",
				ServiceID: "service-orchestrator",
				Scopes:    []string{"llm:*"},
			},
			expectErr: false,
		},
		{
			name: "both tenant and service",
			input: entity.CreateKeyInput{
				Name:      "Dedicated Service Key",
				TenantID:  "tenant-001",
				ServiceID: "service-orchestrator",
				Scopes:    []string{"llm:*"},
			},
			expectErr: false,
		},
		{
			name: "both empty",
			input: entity.CreateKeyInput{
				Name:   "Invalid Key",
				Scopes: []string{"llm:*"},
			},
			expectErr: true,
		},
		{
			name: "spaces only",
			input: entity.CreateKeyInput{
				Name:      "Invalid Whitespace Key",
				TenantID:  "   ",
				ServiceID: "   ",
				Scopes:    []string{"llm:*"},
			},
			expectErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.input.Validate()
			if tc.expectErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

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

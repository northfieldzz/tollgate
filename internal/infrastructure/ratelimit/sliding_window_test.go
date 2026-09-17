package ratelimit

import (
	"testing"
	"time"
)

func TestSlidingWindowLimiter_Allow(t *testing.T) {
	limiter := NewSlidingWindowLimiter(time.Second) // 1秒ウィンドウでテスト

	id := "test-key-1"
	limit := 3

	// 1〜3回目: 許可
	for i := 1; i <= limit; i++ {
		allowed, rem, _ := limiter.Allow(id, limit)
		if !allowed {
			t.Fatalf("expected request %d to be allowed", i)
		}
		expectedRem := limit - i
		if rem != expectedRem {
			t.Errorf("expected remaining %d, got %d", expectedRem, rem)
		}
	}

	// 4回目: 超過で拒否
	allowed, rem, resetIn := limiter.Allow(id, limit)
	if allowed {
		t.Fatalf("expected request 4 to be rejected")
	}
	if rem != 0 {
		t.Errorf("expected remaining 0, got %d", rem)
	}
	if resetIn <= 0 {
		t.Errorf("expected resetIn > 0, got %v", resetIn)
	}

	// 1.1秒待機後: 再度許可
	time.Sleep(1100 * time.Millisecond)
	allowedAfter, remAfter, _ := limiter.Allow(id, limit)
	if !allowedAfter {
		t.Fatalf("expected request to be allowed after window reset")
	}
	if remAfter != limit-1 {
		t.Errorf("expected remaining %d, got %d", limit-1, remAfter)
	}
}

package ratelimit

import (
	"context"
	"testing"
)

func TestDynamoDBRateLimiter_Allow_Unlimited(t *testing.T) {
	// limitRPM <= 0 の場合は DynamoDB を呼ばず無制限を返す
	limiter := &DynamoDBRateLimiter{tableName: "test"}
	allowed, remaining, resetIn, err := limiter.Allow(context.Background(), "key-1", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected allowed=true for unlimited (limitRPM=0)")
	}
	if remaining != 999999 {
		t.Errorf("expected remaining=999999, got %d", remaining)
	}
	if resetIn != 0 {
		t.Errorf("expected resetIn=0, got %v", resetIn)
	}
}

func TestDynamoDBRateLimiter_Allow_NegativeLimit(t *testing.T) {
	limiter := &DynamoDBRateLimiter{tableName: "test"}
	allowed, _, _, err := limiter.Allow(context.Background(), "key-2", -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !allowed {
		t.Error("expected allowed=true for unlimited (limitRPM=-1)")
	}
}

// NOTE: DynamoDB を使用する統合テストは dynamodb-local 起動環境が必要なため
// ここでは実施しない。実際の Allow (count increment / rate limit) は
// compose --profile database を起動した状態での手動検証または E2E テストで確認する。

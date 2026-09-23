package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/northfieldzz/tollgate/internal/domain/entity"
	"github.com/northfieldzz/tollgate/internal/domain/repository"
)

// MockRateLimiter implements repository.RateLimiter for testing.
type MockRateLimiter struct {
	AllowFunc func(ctx context.Context, id string, limitRPM int) (bool, int, time.Duration, error)
}

func (m *MockRateLimiter) Allow(ctx context.Context, id string, limitRPM int) (bool, int, time.Duration, error) {
	if m.AllowFunc != nil {
		return m.AllowFunc(ctx, id, limitRPM)
	}
	// デフォルト: 常に許可
	return true, limitRPM, time.Minute, nil
}

// MockKeyRepository implements repository.KeyRepository for testing.
type MockKeyRepository struct {
	GetKeyByHashFunc          func(ctx context.Context, keyHash string) (*entity.APIKey, error)
	IncrementMonthlyUsageFunc func(ctx context.Context, keyHash string, month string, increment int64) (int64, error)
	UpdateLastUsedAtFunc      func(ctx context.Context, keyHash string, lastUsed time.Time) error
}

func (m *MockKeyRepository) PutKey(ctx context.Context, key *entity.APIKey) error {
	return nil
}

func (m *MockKeyRepository) GetKeyByHash(ctx context.Context, keyHash string) (*entity.APIKey, error) {
	if m.GetKeyByHashFunc != nil {
		return m.GetKeyByHashFunc(ctx, keyHash)
	}
	return nil, nil
}

func (m *MockKeyRepository) GetKeyByID(ctx context.Context, keyID string) (*entity.APIKey, error) {
	return nil, nil
}

func (m *MockKeyRepository) ListKeysByTenant(ctx context.Context, tenantID string) ([]*entity.APIKey, error) {
	return nil, nil
}

func (m *MockKeyRepository) UpdateKeyStatus(ctx context.Context, keyHash string, status entity.KeyStatus, isActive bool) error {
	return nil
}

func (m *MockKeyRepository) UpdateKeySettings(ctx context.Context, keyHash string, input entity.UpdateKeyInput) (*entity.APIKey, error) {
	return nil, nil
}

func (m *MockKeyRepository) RotateKey(ctx context.Context, params entity.RotateKeyParams) (*entity.APIKey, error) {
	return nil, nil
}

func (m *MockKeyRepository) DeleteKey(ctx context.Context, keyHash string) error {
	return nil
}

func (m *MockKeyRepository) IncrementMonthlyUsage(ctx context.Context, keyHash string, month string, increment int64) (int64, error) {
	if m.IncrementMonthlyUsageFunc != nil {
		return m.IncrementMonthlyUsageFunc(ctx, keyHash, month, increment)
	}
	return 0, nil
}

func (m *MockKeyRepository) UpdateLastUsedAt(ctx context.Context, keyHash string, lastUsed time.Time) error {
	if m.UpdateLastUsedAtFunc != nil {
		return m.UpdateLastUsedAtFunc(ctx, keyHash, lastUsed)
	}
	return nil
}

func (m *MockKeyRepository) Ping(ctx context.Context) error {
	return nil
}

func TestVerifyUsecase_VerifyKey(t *testing.T) {
	// デフォルトのモック: 常に許可
	defaultLimiter := &MockRateLimiter{}

	// RPM 超過テスト用のモック: 常に拒否
	rateLimitExceededLimiter := &MockRateLimiter{
		AllowFunc: func(ctx context.Context, id string, limitRPM int) (bool, int, time.Duration, error) {
			return false, 0, time.Second * 30, nil
		},
	}

	now := time.Now()
	pastUnix := now.Add(-time.Hour).Unix()

	currentMonth := now.UTC().Format("2006-01")

	tests := []struct {
		name           string
		inputRawKey    string
		requiredScope  string
		mockGetKey     func(ctx context.Context, keyHash string) (*entity.APIKey, error)
		mockIncUsage   func(ctx context.Context, keyHash string, month string, increment int64) (int64, error)
		expectedValid  bool
		expectedReason string
	}{
		{
			name:        "Happy path",
			inputRawKey: "tlge-live-valid",
			mockGetKey: func(ctx context.Context, keyHash string) (*entity.APIKey, error) {
				return &entity.APIKey{
					KeyID:        "key-1",
					IsActive:     true,
					Status:       entity.StatusActive,
					RateLimitRPM: 100,
					MonthlyQuota: 0,
					Scopes:       []string{"*"},
				}, nil
			},
			expectedValid: true,
		},
		{
			name:        "Key not found",
			inputRawKey: "tlge-live-notfound",
			mockGetKey: func(ctx context.Context, keyHash string) (*entity.APIKey, error) {
				return nil, nil // not found
			},
			expectedValid:  false,
			expectedReason: "invalid_key",
		},
		{
			name:        "DB error",
			inputRawKey: "tlge-live-error",
			mockGetKey: func(ctx context.Context, keyHash string) (*entity.APIKey, error) {
				return nil, errors.New("db connection failed")
			},
			expectedValid: false,
		},
		{
			name:        "Expired (TTL)",
			inputRawKey: "tlge-live-expired",
			mockGetKey: func(ctx context.Context, keyHash string) (*entity.APIKey, error) {
				return &entity.APIKey{
					KeyID:     "key-expired",
					ExpiresAt: &pastUnix,
					IsActive:  true,
					Status:    entity.StatusActive,
				}, nil
			},
			expectedValid:  false,
			expectedReason: "expired",
		},
		{
			name:        "Rotation Expired",
			inputRawKey: "tlge-live-rotation-expired",
			mockGetKey: func(ctx context.Context, keyHash string) (*entity.APIKey, error) {
				return &entity.APIKey{
					KeyID:    "key-rot-expired",
					IsActive: true,
					Status:   entity.StatusRotating,
					Rotation: &entity.RotationMeta{
						GracePeriodExpiresAt: now.Add(-time.Hour),
					},
				}, nil
			},
			expectedValid:  false,
			expectedReason: "rotation_expired",
		},
		{
			name:        "Inactive / Suspended",
			inputRawKey: "tlge-live-suspended",
			mockGetKey: func(ctx context.Context, keyHash string) (*entity.APIKey, error) {
				return &entity.APIKey{
					KeyID:    "key-susp",
					IsActive: true, // Even if true, status overrides
					Status:   entity.StatusSuspended,
				}, nil
			},
			expectedValid:  false,
			expectedReason: "suspended",
		},
		{
			name:        "Revoked",
			inputRawKey: "tlge-live-revoked",
			mockGetKey: func(ctx context.Context, keyHash string) (*entity.APIKey, error) {
				return &entity.APIKey{
					KeyID:    "key-rev",
					IsActive: false,
					Status:   entity.StatusRevoked,
				}, nil
			},
			expectedValid:  false,
			expectedReason: "revoked",
		},
		{
			name:          "Scope Mismatch",
			inputRawKey:   "tlge-live-scope",
			requiredScope: "ai:workflows:execute",
			mockGetKey: func(ctx context.Context, keyHash string) (*entity.APIKey, error) {
				return &entity.APIKey{
					KeyID:    "key-scope",
					IsActive: true,
					Status:   entity.StatusActive,
					Scopes:   []string{"mcp:tools:execute"}, // missing ai scope
				}, nil
			},
			expectedValid:  false,
			expectedReason: "scope_mismatch",
		},
		{
			name:        "Rate Limit Exceeded",
			inputRawKey: "tlge-live-rpm-limit",
			mockGetKey: func(ctx context.Context, keyHash string) (*entity.APIKey, error) {
				return &entity.APIKey{
					KeyID:        "exhausted-rpm",
					IsActive:     true,
					Status:       entity.StatusActive,
					Scopes:       []string{"*"},
					RateLimitRPM: 1,
				}, nil
			},
			expectedValid:  false,
			expectedReason: "rate_limit_exceeded",
		},
		{
			name:        "Monthly Quota Exceeded (current usage >= quota)",
			inputRawKey: "tlge-live-quota-limit",
			mockGetKey: func(ctx context.Context, keyHash string) (*entity.APIKey, error) {
				return &entity.APIKey{
					KeyID:             "key-quota",
					IsActive:          true,
					Status:            entity.StatusActive,
					Scopes:            []string{"*"},
					RateLimitRPM:      100,
					MonthlyQuota:      1000,
					CurrentMonth:      currentMonth,
					CurrentMonthUsage: 1000, // already reached
				}, nil
			},
			expectedValid:  false,
			expectedReason: "quota_exceeded",
		},
		{
			name:        "Monthly Quota Increment",
			inputRawKey: "tlge-live-quota-inc",
			mockGetKey: func(ctx context.Context, keyHash string) (*entity.APIKey, error) {
				return &entity.APIKey{
					KeyID:             "key-quota-inc",
					IsActive:          true,
					Status:            entity.StatusActive,
					Scopes:            []string{"*"},
					RateLimitRPM:      100,
					MonthlyQuota:      1000,
					CurrentMonth:      currentMonth,
					CurrentMonthUsage: 999, // 1 left before request
				}, nil
			},
			mockIncUsage: func(ctx context.Context, keyHash string, month string, increment int64) (int64, error) {
				return 1000, nil // new usage
			},
			expectedValid: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := &MockKeyRepository{
				GetKeyByHashFunc:          tc.mockGetKey,
				IncrementMonthlyUsageFunc: tc.mockIncUsage,
			}
			// Rate Limit Exceeded テストのみ常に拒否する limiter を使用する
			var limiter repository.RateLimiter = defaultLimiter
			if tc.name == "Rate Limit Exceeded" {
				limiter = rateLimitExceededLimiter
			}
			uc := NewVerifyUsecase(repo, limiter)

			input := entity.VerifyKeyInput{
				RawKey:        tc.inputRawKey,
				RequiredScope: tc.requiredScope,
			}

			output, err := uc.VerifyKey(context.Background(), input)
			if tc.name == "DB error" {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if output == nil {
				t.Fatalf("expected output, got nil")
			}

			if output.Valid != tc.expectedValid {
				t.Errorf("expected valid=%v, got %v", tc.expectedValid, output.Valid)
			}

			if tc.expectedReason != "" && output.Reason != tc.expectedReason {
				t.Errorf("expected reason=%q, got %q", tc.expectedReason, output.Reason)
			}
		})
	}
}

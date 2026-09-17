package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/northfieldzz/null_and_void_work_agent/apps/api_manager/internal/domain/entity"
	"github.com/northfieldzz/null_and_void_work_agent/apps/api_manager/internal/domain/repository"
	"github.com/northfieldzz/null_and_void_work_agent/apps/api_manager/internal/infrastructure/metrics"
	"github.com/northfieldzz/null_and_void_work_agent/apps/api_manager/internal/infrastructure/ratelimit"
)

type VerifyUsecase struct {
	repo    repository.KeyRepository
	limiter *ratelimit.SlidingWindowLimiter
}

func NewVerifyUsecase(repo repository.KeyRepository, limiter *ratelimit.SlidingWindowLimiter) *VerifyUsecase {
	return &VerifyUsecase{
		repo:    repo,
		limiter: limiter,
	}
}

// matchScope は要求スコープが許可スコープリストに合致するか判定する (ワイルドカード対応)
func matchScope(required string, allowedScopes []string) bool {
	if required == "" {
		return true
	}
	for _, allowed := range allowedScopes {
		if allowed == "*" || allowed == required {
			return true
		}
		if strings.HasSuffix(allowed, ":*") {
			prefix := strings.TrimSuffix(allowed, ":*")
			if strings.HasPrefix(required, prefix+":") || required == prefix {
				return true
			}
		}
	}
	return false
}

// VerifyKey はキーの正当性、有効期限、スコープ、RPM レート、月間クォータを判定し、カウントを更新する
func (u *VerifyUsecase) VerifyKey(ctx context.Context, input entity.VerifyKeyInput) (*entity.VerifyKeyOutput, error) {
	start := time.Now()
	defer func() {
		// レイテンシ記録
		metrics.VerificationDuration.WithLabelValues("global").Observe(time.Since(start).Seconds())
	}()

	keyHash := HashKey(input.RawKey)
	key, err := u.repo.GetKeyByHash(ctx, keyHash)
	if err != nil {
		metrics.VerificationsTotal.WithLabelValues("unknown", "rejected", "db_error").Inc()
		return nil, err
	}
	if key == nil {
		metrics.VerificationsTotal.WithLabelValues("unknown", "rejected", "invalid_key").Inc()
		return &entity.VerifyKeyOutput{
			Valid:  false,
			Reason: "invalid_key",
		}, nil
	}

	tenant := key.TenantID

	// 1. 有効期限 (TTL) チェック
	nowUnix := time.Now().Unix()
	if key.ExpiresAt != nil && *key.ExpiresAt < nowUnix {
		metrics.VerificationsTotal.WithLabelValues(tenant, "rejected", "expired").Inc()
		return &entity.VerifyKeyOutput{
			Valid:     false,
			Reason:    "expired",
			KeyID:     key.KeyID,
			KeyPrefix: key.KeyPrefix,
			TenantID:  tenant,
		}, nil
	}

	// 2. ローテーション猶予期間チェック
	if key.Status == entity.StatusRotating && key.Rotation != nil {
		if time.Now().After(key.Rotation.GracePeriodExpiresAt) {
			metrics.VerificationsTotal.WithLabelValues(tenant, "rejected", "rotation_expired").Inc()
			return &entity.VerifyKeyOutput{
				Valid:     false,
				Reason:    "rotation_expired",
				KeyID:     key.KeyID,
				KeyPrefix: key.KeyPrefix,
				TenantID:  tenant,
			}, nil
		}
	}

	// 3. 有効フラグ (一時停止 / 失効) チェック
	if !key.IsActive || key.Status == entity.StatusSuspended || key.Status == entity.StatusRevoked {
		reason := "suspended"
		if key.Status == entity.StatusRevoked {
			reason = "revoked"
		}
		metrics.VerificationsTotal.WithLabelValues(tenant, "rejected", reason).Inc()
		return &entity.VerifyKeyOutput{
			Valid:     false,
			Reason:    reason,
			KeyID:     key.KeyID,
			KeyPrefix: key.KeyPrefix,
			TenantID:  tenant,
		}, nil
	}

	// 4. スコープ認可判定
	if !matchScope(input.RequiredScope, key.Scopes) {
		metrics.VerificationsTotal.WithLabelValues(tenant, "rejected", "scope_mismatch").Inc()
		return &entity.VerifyKeyOutput{
			Valid:     false,
			Reason:    "scope_mismatch",
			KeyID:     key.KeyID,
			KeyPrefix: key.KeyPrefix,
			TenantID:  tenant,
			Scopes:    key.Scopes,
		}, nil
	}

	// 5. スライディングウィンドウ RPM レート判定
	allowed, remainingRPM, _ := u.limiter.Allow(key.KeyID, key.RateLimitRPM)
	if !allowed {
		metrics.VerificationsTotal.WithLabelValues(tenant, "rejected", "rate_limit_exceeded").Inc()
		metrics.RateLimitExceededTotal.WithLabelValues(tenant, key.KeyPrefix).Inc()
		return &entity.VerifyKeyOutput{
			Valid:        false,
			Reason:       "rate_limit_exceeded",
			KeyID:        key.KeyID,
			KeyPrefix:    key.KeyPrefix,
			TenantID:     tenant,
			RemainingRPM: 0,
			LimitRPM:     key.RateLimitRPM,
		}, nil
	}

	// 6. 月間クォータ判定 & Atomic カウントインクリメント
	currentMonth := time.Now().UTC().Format("2006-01")
	var remainingQuota int64 = -1 // -1 は無制限
	if key.MonthlyQuota > 0 {
		if key.CurrentMonth == currentMonth && key.CurrentMonthUsage >= key.MonthlyQuota {
			metrics.VerificationsTotal.WithLabelValues(tenant, "rejected", "quota_exceeded").Inc()
			metrics.RateLimitExceededTotal.WithLabelValues(tenant, key.KeyPrefix).Inc()
			return &entity.VerifyKeyOutput{
				Valid:          false,
				Reason:         "quota_exceeded",
				KeyID:          key.KeyID,
				KeyPrefix:      key.KeyPrefix,
				TenantID:       tenant,
				RemainingRPM:   remainingRPM,
				RemainingQuota: 0,
				LimitRPM:       key.RateLimitRPM,
				MonthlyQuota:   key.MonthlyQuota,
			}, nil
		}

		// カウント加算
		newUsage, err := u.repo.IncrementMonthlyUsage(ctx, keyHash, currentMonth, 1)
		if err == nil {
			remainingQuota = key.MonthlyQuota - newUsage
			if remainingQuota < 0 {
				remainingQuota = 0
			}
		}
	}

	// 7. 最終利用日時の非同期更新 (メイン処理をブロックしない)
	go func() {
		_ = u.repo.UpdateLastUsedAt(context.Background(), keyHash, time.Now().UTC())
	}()

	metrics.VerificationsTotal.WithLabelValues(tenant, "allowed", "success").Inc()

	return &entity.VerifyKeyOutput{
		Valid:          true,
		KeyID:          key.KeyID,
		KeyPrefix:      key.KeyPrefix,
		TenantID:       tenant,
		ServiceID:      key.ServiceID,
		Scopes:         key.Scopes,
		RemainingRPM:   remainingRPM,
		RemainingQuota: remainingQuota,
		LimitRPM:       key.RateLimitRPM,
		MonthlyQuota:   key.MonthlyQuota,
	}, nil
}

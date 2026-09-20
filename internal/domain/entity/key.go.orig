package entity

import "time"

// KeyStatus は API キーの状態を表す型
type KeyStatus string

const (
	StatusActive    KeyStatus = "active"
	StatusSuspended KeyStatus = "suspended"
	StatusRotating  KeyStatus = "rotating"
	StatusRevoked   KeyStatus = "revoked"
)

// RotationMeta はキーローテーション時の移行メタデータ
type RotationMeta struct {
	OldKeyHash           string    `dynamodbav:"old_key_hash" json:"old_key_hash,omitempty"`
	GracePeriodExpiresAt time.Time `dynamodbav:"grace_period_expires_at" json:"grace_period_expires_at,omitempty"`
}

// APIKey は DynamoDB (TollgateAPIKeys) に永続化される API キー実体
type APIKey struct {
	PK                string        `dynamodbav:"pk" json:"-"`                                          // KEY#<sha256_hash>
	KeyID             string        `dynamodbav:"key_id" json:"key_id"`                                 // UUID
	KeyPrefix         string        `dynamodbav:"key_prefix" json:"key_prefix"`                         // tlge-live-xxxx
	Name              string        `dynamodbav:"name" json:"name"`                                     // 表示名
	TenantID          string        `dynamodbav:"tenant_id" json:"tenant_id"`                           // GSI Partition Key
	ServiceID         string        `dynamodbav:"service_id" json:"service_id,omitempty"`               // サービス名
	Scopes            []string      `dynamodbav:"scopes" json:"scopes"`                                 // 許可スコープ ("llm:*", "ai:workflows", "mcp:tools" 等)
	RateLimitRPM      int           `dynamodbav:"rate_limit_rpm" json:"rate_limit_rpm"`                 // 1分間リクエスト上限
	MonthlyQuota      int64         `dynamodbav:"monthly_quota" json:"monthly_quota"`                   // 月間上限 (0: 無制限)
	CurrentMonthUsage int64         `dynamodbav:"current_month_usage" json:"current_month_usage"`       // 当月利用数
	CurrentMonth      string        `dynamodbav:"current_month" json:"current_month"`                   // YYYY-MM
	Status            KeyStatus     `dynamodbav:"status" json:"status"`                                 // active, suspended, rotating, revoked
	IsActive          bool          `dynamodbav:"is_active" json:"is_active"`                           // 有効フラグ
	LastUsedAt        *time.Time    `dynamodbav:"last_used_at,omitempty" json:"last_used_at,omitempty"` // 最終利用日時
	Rotation          *RotationMeta `dynamodbav:"rotation_meta,omitempty" json:"rotation_meta,omitempty"`
	ExpiresAt         *int64        `dynamodbav:"expires_at,omitempty" json:"expires_at,omitempty"` // Unix 秒 (TTL)
	CreatedAt         string        `dynamodbav:"created_at" json:"created_at"`                     // ISO 8601 (GSI Sort Key)
	UpdatedAt         string        `dynamodbav:"updated_at" json:"updated_at"`                     // ISO 8601
}

// GetHash returns the hash string by removing the "KEY#" prefix from PK.
func (k *APIKey) GetHash() string {
	if len(k.PK) > 4 {
		return k.PK[4:]
	}
	return ""
}

// CreateKeyInput は API キー新規発行の入力パラメータ
type CreateKeyInput struct {
	Name         string   `json:"name" doc:"API キーの名称・用途" required:"true" example:"Production AI Engine Workflow"`
	TenantID     string   `json:"tenant_id" doc:"所属テナント ID" required:"true" example:"dept-risk-01"`
	ServiceID    string   `json:"service_id,omitempty" doc:"呼び出し元サービス識別子" example:"finance-app"`
	Scopes       []string `json:"scopes" doc:"許可スコープ一覧 (例: llm:*, ai:workflows:execute, mcp:tools:execute)" required:"true" example:"[\"llm:*\", \"ai:workflows:execute\", \"mcp:tools:execute\"]"`
	RateLimitRPM int      `json:"rate_limit_rpm,omitempty" doc:"分間リクエスト上限 (デフォルト: 600)" default:"600" example:"600"`
	MonthlyQuota int64    `json:"monthly_quota,omitempty" doc:"月間最大リクエスト上限 (0: 無制限)" default:"0" example:"100000"`
	ExpiresIn    int64    `json:"expires_in,omitempty" doc:"有効期間 (秒)。未指定時は無期限" example:"2592000"`
}

// CreateKeyOutput は API キー新規発行時の出力 (平文 RawKey を1度だけ返却)
type CreateKeyOutput struct {
	APIKey
	RawKey string `json:"raw_key" doc:"生成された平文 API キー (このレスポンス時のみ1度だけ開示)" example:"tlge-live-8f9c2d1e0a4b3c5d6e7f8a9b0c1d2e3f"`
}

// UpdateKeyInput は API キー設定変更の入力パラメータ
type UpdateKeyInput struct {
	Name         *string   `json:"name,omitempty" doc:"更新後のキー名称"`
	Scopes       *[]string `json:"scopes,omitempty" doc:"更新後のスコープ一覧"`
	RateLimitRPM *int      `json:"rate_limit_rpm,omitempty" doc:"更新後の分間上限"`
	MonthlyQuota *int64    `json:"monthly_quota,omitempty" doc:"更新後の月間上限"`
}

// RotateKeyInput はゼロダウンタイム・ローテーションの入力パラメータ
type RotateKeyInput struct {
	GracePeriodHours int `json:"grace_period_hours,omitempty" doc:"旧キーが引き続き有効となる猶予期間 (時間, デフォルト: 24)" default:"24" example:"24"`
}

// RotateKeyOutput はローテーション結果 (新しい平文キーを返却)
type RotateKeyOutput struct {
	KeyID                string    `json:"key_id"`
	KeyPrefix            string    `json:"key_prefix"`
	NewRawKey            string    `json:"new_raw_key" doc:"新しく生成された平文 API キー"`
	GracePeriodExpiresAt time.Time `json:"grace_period_expires_at" doc:"旧キーが自動失効する日時"`
}

// VerifyKeyInput は他サービス (ai_engine / mcp_gateway / llm_gateway) からのキー検証リクエスト
type VerifyKeyInput struct {
	RawKey        string `json:"raw_key" doc:"検証対象の平文 API キー" required:"true" example:"tlge-live-8f9c2d1e0a4b3c5d6e7f8a9b0c1d2e3f"`
	RequiredScope string `json:"required_scope,omitempty" doc:"実行に必要なスコープ (例: ai:workflows:execute)" example:"mcp:tools:execute"`
}

// VerifyKeyOutput はキー検証およびレートリミット消費の判定結果
type VerifyKeyOutput struct {
	Valid          bool     `json:"valid" doc:"キーが有効で実行可能かどうか"`
	Reason         string   `json:"reason,omitempty" doc:"拒否理由 (invalid_key, suspended, expired, scope_mismatch, rate_limit_exceeded, quota_exceeded)"`
	KeyID          string   `json:"key_id,omitempty"`
	KeyPrefix      string   `json:"key_prefix,omitempty"`
	TenantID       string   `json:"tenant_id,omitempty"`
	ServiceID      string   `json:"service_id,omitempty"`
	Scopes         []string `json:"scopes,omitempty"`
	RemainingRPM   int      `json:"remaining_rpm" doc:"当分間の残り可能リクエスト数"`
	RemainingQuota int64    `json:"remaining_quota" doc:"当月の残り可能リクエスト数 (-1: 無制限)"`
	LimitRPM       int      `json:"limit_rpm"`
	MonthlyQuota   int64    `json:"monthly_quota"`
}

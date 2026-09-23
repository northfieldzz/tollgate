package repository

import (
	"context"
	"time"
)

// RateLimiter はレートリミットのバックエンド抽象インターフェース。
// インメモリ・DynamoDB・Redis など複数の実装を差し替え可能にする。
type RateLimiter interface {
	// Allow は指定された識別子のリクエストを 1 件消費し、
	// 許可可否・残り件数・リセット所要時間を返す。
	// バックエンドエラー時は err を返す（呼び出し側でフェイルオープンまたはクローズを判断する）。
	Allow(ctx context.Context, id string, limitRPM int) (allowed bool, remaining int, resetIn time.Duration, err error)
}

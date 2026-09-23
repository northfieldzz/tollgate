package sqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib" // pgx ドライバ登録
)

// NewPostgresRepository は PostgreSQL バックエンドの KeyRepository を生成する。
//
// dsn には PostgreSQL 接続文字列を指定する (例: "postgres://user:pass@localhost:5432/tollgate?sslmode=disable")。
//
// # 特性
//   - マルチインスタンス対応 (分散構成可能)
//   - レートリミットは redis (推奨) または in-memory (単一ノード時) を選択可能
func NewPostgresRepository(ctx context.Context, dsn string) (*SQLRepository, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("postgres open: %w", err)
	}

	// コネクションプールの設定
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	repo := &SQLRepository{db: db, dialect: DialectPostgres}

	if err := AutoMigrate(ctx, db); err != nil {
		return nil, fmt.Errorf("postgres migrate: %w", err)
	}

	return repo, nil
}

package http

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/northfieldzz/tollgate/internal/domain/repository"
)

type HealthOutput struct {
	Body struct {
		Status   string `json:"status" example:"ok" doc:"稼働ステータス"`
		Service  string `json:"service" example:"api_manager" doc:"サービス名"`
		Database string `json:"database,omitempty" example:"connected" doc:"DynamoDB 疎通ステータス"`
	}
}

func RegisterHealthHandler(api huma.API, repo repository.KeyRepository) {
	// 1. GET /health (総合ヘルスチェック)
	huma.Register(api, huma.Operation{
		OperationID: "healthCheck",
		Method:      http.MethodGet,
		Path:        "/health",
		Summary:     "総合ヘルスチェック",
		Description: "プロセスの生存および DynamoDB への疎通状態を総合的に確認します。",
		Tags:        []string{"System"},
	}, func(ctx context.Context, input *struct{}) (*HealthOutput, error) {
		resp := &HealthOutput{}
		resp.Body.Status = "ok"
		resp.Body.Service = "api_manager"

		if err := repo.Ping(ctx); err != nil {
			resp.Body.Status = "degraded"
			resp.Body.Database = "disconnected"
			return resp, huma.Error503ServiceUnavailable("DynamoDB 疎通不可", err)
		}
		resp.Body.Database = "connected"
		return resp, nil
	})

	liveHandler := func(ctx context.Context, input *struct{}) (*HealthOutput, error) {
		resp := &HealthOutput{}
		resp.Body.Status = "alive"
		resp.Body.Service = "api_manager"
		return resp, nil
	}

	readyHandler := func(ctx context.Context, input *struct{}) (*HealthOutput, error) {
		resp := &HealthOutput{}
		resp.Body.Service = "api_manager"

		if err := repo.Ping(ctx); err != nil {
			resp.Body.Status = "not_ready"
			resp.Body.Database = "disconnected"
			return resp, huma.Error503ServiceUnavailable("DynamoDB 未接続", err)
		}
		resp.Body.Status = "ready"
		resp.Body.Database = "connected"
		return resp, nil
	}

	// 2. Liveness プローブ (/health/live, /livez)
	huma.Register(api, huma.Operation{
		OperationID: "livenessCheck",
		Method:      http.MethodGet,
		Path:        "/health/live",
		Summary:     "Liveness プローブ (プロセスの死活監視)",
		Description: "コンテナ・プロセスの生存を確認します。外部依存関係 (DynamoDB 等) を見ずに即座に 200 を返却します。",
		Tags:        []string{"System"},
	}, liveHandler)

	huma.Register(api, huma.Operation{
		OperationID: "livenessProbeLivez",
		Method:      http.MethodGet,
		Path:        "/livez",
		Summary:     "Liveness プローブ (/livez)",
		Description: "Kubernetes 標準 Liveness プローブ用エンドポイントです。",
		Tags:        []string{"System"},
	}, liveHandler)

	// 3. Readiness プローブ (/health/ready, /readyz)
	huma.Register(api, huma.Operation{
		OperationID: "readinessCheck",
		Method:      http.MethodGet,
		Path:        "/health/ready",
		Summary:     "Readiness プローブ (トラフィック受付準備監視)",
		Description: "DynamoDB への接続が完了し、トラフィックを受け入れ可能か確認します。DB 疎通不可時は 503 を返却します。",
		Tags:        []string{"System"},
	}, readyHandler)

	huma.Register(api, huma.Operation{
		OperationID: "readinessProbeReadyz",
		Method:      http.MethodGet,
		Path:        "/readyz",
		Summary:     "Readiness プローブ (/readyz)",
		Description: "Kubernetes 標準 Readiness プローブ用エンドポイントです。",
		Tags:        []string{"System"},
	}, readyHandler)
}

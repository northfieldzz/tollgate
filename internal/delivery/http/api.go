package http

import (
	"encoding/json"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/northfieldzz/null_and_void_work_agent/apps/api_manager/internal/domain/repository"
	"github.com/northfieldzz/null_and_void_work_agent/apps/api_manager/internal/usecase"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(keyUsecase *usecase.KeyUsecase, verifyUsecase *usecase.VerifyUsecase, repo repository.KeyRepository) http.Handler {
	mux := http.NewServeMux()

	// 1. Prometheus メトリクスエンドポイント (/metrics)
	mux.Handle("/metrics", promhttp.Handler())

	// 2. Huma v2 OpenAPI 3.1 設定 (UI は提供せず openapi.json のみ提供)
	config := huma.DefaultConfig("IT Context Platform — API Manager", "1.0.0")
	config.DocsPath = "" // ドキュメント UI を無効化
	config.OpenAPIPath = "/openapi"
	config.Info.Description = "マルチテナント対応 API キー管理 (発行・失効・ローテーション) およびリクエストレートリミット / クォータ検証 Web API"

	api := humago.New(mux, config)

	// OpenAPI 3.1 JSON エイリアス (/openapi.json)
	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.oai.openapi+json")
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(api.OpenAPI())
	})

	// 3. 各エンドポイントの登録
	RegisterHealthHandler(api, repo)
	RegisterKeyHandlers(api, keyUsecase)
	RegisterVerifyHandler(api, verifyUsecase)

	return mux
}

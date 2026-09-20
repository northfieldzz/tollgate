package http

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/northfieldzz/tollgate/internal/config"
	"github.com/northfieldzz/tollgate/internal/domain/repository"
	"github.com/northfieldzz/tollgate/internal/usecase"
)

func NewRouter(cfg *config.Config, keyUsecase *usecase.KeyUsecase, verifyUsecase *usecase.VerifyUsecase, repo repository.KeyRepository, proxyHandler http.Handler) http.Handler {
	mux := http.NewServeMux()

	// 1. Huma v2 OpenAPI 3.1 設定
	humaConfig := huma.DefaultConfig("Tollgate", "1.0.0")
	humaConfig.DocsPath = cfg.DocsPath // 未指定(空文字)の場合はドキュメント UI が無効化される

	if cfg.OpenAPIPath != "" {
		humaConfig.OpenAPIPath = strings.TrimSuffix(cfg.OpenAPIPath, ".json")
	} else {
		humaConfig.OpenAPIPath = "" // 未指定(空文字)の場合は OpenAPI スキーマ提供が無効化される
	}
	humaConfig.Info.Description = "マルチテナント対応 API キー管理およびリクエストレートリミット / クォータ検証 Web API & リバースプロキシ"

	api := humago.New(mux, humaConfig)

	// OpenAPI 3.1 JSON エンドポイント (OPENAPI_PATH が指定されている場合のみ提供)
	if cfg.OpenAPIPath != "" {
		mux.HandleFunc(cfg.OpenAPIPath, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.oai.openapi+json")
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			_ = enc.Encode(api.OpenAPI())
		})
	}

	// 3. 各エンドポイントの登録
	RegisterHealthHandler(api, repo)
	RegisterMetricsHandler(api)
	RegisterKeyHandlers(api, keyUsecase)
	RegisterVerifyHandler(api, verifyUsecase)

	// 4. リバースプロキシ (プロキシハンドラーが存在する場合)
	if proxyHandler != nil {
		// Huma / 管理系エンドポイント以外のすべてのリクエストをプロキシに流す
		mux.Handle("/", proxyHandler)
	}

	return mux
}

package http

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"
	"github.com/northfieldzz/tollgate/internal/domain/repository"
	"github.com/northfieldzz/tollgate/internal/usecase"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func NewRouter(keyUsecase *usecase.KeyUsecase, verifyUsecase *usecase.VerifyUsecase, repo repository.KeyRepository, proxyHandler http.Handler) http.Handler {
	mux := http.NewServeMux()

	// 1. Prometheus メトリクスエンドポイント (/metrics)
	mux.Handle("/metrics", promhttp.Handler())

	// 2. Huma v2 OpenAPI 3.1 設定
	config := huma.DefaultConfig("Tollgate", "1.0.0")

	docsPath := os.Getenv("DOCS_PATH")
	if docsPath != "" && !strings.HasPrefix(docsPath, "/") {
		docsPath = "/" + docsPath
	}
	config.DocsPath = docsPath // 未指定(空文字)の場合はドキュメント UI が無効化される

	openapiPath := os.Getenv("OPENAPI_PATH")
	if openapiPath != "" && !strings.HasPrefix(openapiPath, "/") {
		openapiPath = "/" + openapiPath
	}
	if openapiPath != "" {
		config.OpenAPIPath = strings.TrimSuffix(openapiPath, ".json")
	} else {
		config.OpenAPIPath = "" // 未指定(空文字)の場合は OpenAPI スキーマ提供が無効化される
	}
	config.Info.Description = "マルチテナント対応 API キー管理およびリクエストレートリミット / クォータ検証 Web API & リバースプロキシ"

	api := humago.New(mux, config)

	// OpenAPI 3.1 JSON エンドポイント (OPENAPI_PATH が指定されている場合のみ提供)
	if openapiPath != "" {
		mux.HandleFunc(openapiPath, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/vnd.oai.openapi+json")
			enc := json.NewEncoder(w)
			enc.SetIndent("", "  ")
			_ = enc.Encode(api.OpenAPI())
		})
	}

	// 3. 各エンドポイントの登録
	RegisterHealthHandler(api, repo)
	RegisterKeyHandlers(api, keyUsecase)
	RegisterVerifyHandler(api, verifyUsecase)

	// 4. リバースプロキシ (プロキシハンドラーが存在する場合)
	if proxyHandler != nil {
		// Huma / 管理系エンドポイント以外のすべてのリクエストをプロキシに流す
		mux.Handle("/", proxyHandler)
	}

	return mux
}

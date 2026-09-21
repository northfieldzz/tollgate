package http

import (
	"crypto/subtle"
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

	// Security Schemes 定義
	if humaConfig.Components == nil {
		humaConfig.Components = &huma.Components{}
	}
	if humaConfig.Components.SecuritySchemes == nil {
		humaConfig.Components.SecuritySchemes = map[string]*huma.SecurityScheme{}
	}
	humaConfig.Components.SecuritySchemes["adminBearerAuth"] = &huma.SecurityScheme{
		Type:        "http",
		Scheme:      "bearer",
		Description: "Admin API Key via Authorization: Bearer <ADMIN_API_KEY>",
	}
	humaConfig.Components.SecuritySchemes["adminApiKeyAuth"] = &huma.SecurityScheme{
		Type:        "apiKey",
		In:          "header",
		Name:        "X-Admin-Key",
		Description: "Admin API Key via X-Admin-Key header",
	}

	api := humago.New(mux, humaConfig)

	// 2. Admin API 認証ミドルウェア (/v1/admin/ 配下の保護)
	api.UseMiddleware(func(ctx huma.Context, next func(huma.Context)) {
		path := ctx.URL().Path
		if strings.HasPrefix(path, "/v1/admin/") || path == "/v1/admin" {
			if cfg.AdminAPIKey == "" {
				_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "ADMIN_API_KEY is not configured on server", nil)
				return
			}

			authHeader := ctx.Header("Authorization")
			var providedKey string
			if strings.HasPrefix(authHeader, "Bearer ") {
				providedKey = strings.TrimPrefix(authHeader, "Bearer ")
			} else if k := ctx.Header("X-Admin-Key"); k != "" {
				providedKey = k
			}

			if subtle.ConstantTimeCompare([]byte(providedKey), []byte(cfg.AdminAPIKey)) != 1 {
				_ = huma.WriteErr(api, ctx, http.StatusUnauthorized, "invalid or missing admin api key", nil)
				return
			}
		}
		next(ctx)
	})

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

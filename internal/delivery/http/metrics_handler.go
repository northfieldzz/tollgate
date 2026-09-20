package http

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/danielgtaylor/huma/v2"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RegisterMetricsHandler は Prometheus メトリクスエンドポイントを Huma (OpenAPI) にプロキシ登録する
func RegisterMetricsHandler(api huma.API) {
	promHandler := promhttp.Handler()

	huma.Register(api, huma.Operation{
		OperationID: "getMetrics",
		Method:      http.MethodGet,
		Path:        "/metrics",
		Summary:     "Prometheus メトリクス照会",
		Description: "Prometheus 形式のリアルタイムパフォーマンス・カウンタメトリクスを返却します。",
		Tags:        []string{"System"},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "Prometheus メトリクス (text/plain)",
				Content: map[string]*huma.MediaType{
					"text/plain": {},
				},
			},
		},
	}, func(ctx context.Context, input *struct{}) (*huma.StreamResponse, error) {
		return &huma.StreamResponse{
			Body: func(hCtx huma.Context) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/metrics", nil).WithContext(ctx)
				promHandler.ServeHTTP(rec, req)

				hCtx.SetHeader("Content-Type", rec.Header().Get("Content-Type"))
				_, _ = hCtx.BodyWriter().Write(rec.Body.Bytes())
			},
		}, nil
	})
}

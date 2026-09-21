package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/northfieldzz/tollgate/internal/config"
	"github.com/northfieldzz/tollgate/internal/usecase"
)

func TestAdminAPI_Authentication(t *testing.T) {
	repo := &mockHealthRepo{}
	keyUsecase := usecase.NewKeyUsecase(repo)

	t.Run("Server without ADMIN_API_KEY blocks requests with 401", func(t *testing.T) {
		cfg := &config.Config{
			AdminAPIKey: "", // 未設定
		}
		router := NewRouter(cfg, keyUsecase, nil, repo, nil)

		req := httptest.NewRequest(http.MethodGet, "/v1/admin/keys?tenant_id=t1", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401 Unauthorized, got %d", rec.Code)
		}
	})

	t.Run("Configured ADMIN_API_KEY validates Bearer and X-Admin-Key", func(t *testing.T) {
		adminKey := "super-admin-secret-999"
		cfg := &config.Config{
			AdminAPIKey: adminKey,
		}
		router := NewRouter(cfg, keyUsecase, nil, repo, nil)

		tests := []struct {
			name       string
			path       string
			headerKey  string
			headerVal  string
			wantStatus int
		}{
			{
				name:       "Missing auth header",
				path:       "/v1/admin/keys?tenant_id=t1",
				wantStatus: http.StatusUnauthorized,
			},
			{
				name:       "Invalid Bearer token",
				path:       "/v1/admin/keys?tenant_id=t1",
				headerKey:  "Authorization",
				headerVal:  "Bearer wrong-secret",
				wantStatus: http.StatusUnauthorized,
			},
			{
				name:       "Invalid X-Admin-Key header",
				path:       "/v1/admin/keys?tenant_id=t1",
				headerKey:  "X-Admin-Key",
				headerVal:  "wrong-secret",
				wantStatus: http.StatusUnauthorized,
			},
			{
				name:       "Valid Bearer token",
				path:       "/v1/admin/keys?tenant_id=t1",
				headerKey:  "Authorization",
				headerVal:  "Bearer " + adminKey,
				wantStatus: http.StatusOK,
			},
			{
				name:       "Valid X-Admin-Key header",
				path:       "/v1/admin/keys?tenant_id=t1",
				headerKey:  "X-Admin-Key",
				headerVal:  adminKey,
				wantStatus: http.StatusOK,
			},
			{
				name:       "Old /v1/keys path returns 404",
				path:       "/v1/keys?tenant_id=t1",
				headerKey:  "Authorization",
				headerVal:  "Bearer " + adminKey,
				wantStatus: http.StatusNotFound,
			},
			{
				name:       "Reversed /admin/v1/keys path returns 404",
				path:       "/admin/v1/keys?tenant_id=t1",
				headerKey:  "Authorization",
				headerVal:  "Bearer " + adminKey,
				wantStatus: http.StatusNotFound,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, tt.path, nil)
				if tt.headerKey != "" {
					req.Header.Set(tt.headerKey, tt.headerVal)
				}
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)

				if rec.Code != tt.wantStatus {
					t.Errorf("path %s [%s: %s]: expected %d, got %d, body: %s",
						tt.path, tt.headerKey, tt.headerVal, tt.wantStatus, rec.Code, rec.Body.String())
				}
			})
		}
	})
}

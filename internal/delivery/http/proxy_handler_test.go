package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/northfieldzz/tollgate/internal/domain/entity"
	"github.com/northfieldzz/tollgate/internal/infrastructure/ratelimit"
	"github.com/northfieldzz/tollgate/internal/usecase"
)

type mockKeyRepo struct {
	keys map[string]*entity.APIKey
}

func (m *mockKeyRepo) PutKey(ctx context.Context, key *entity.APIKey) error { return nil }
func (m *mockKeyRepo) GetKeyByHash(ctx context.Context, keyHash string) (*entity.APIKey, error) {
	if k, ok := m.keys[keyHash]; ok {
		return k, nil
	}
	return nil, nil
}
func (m *mockKeyRepo) GetKeyByID(ctx context.Context, keyID string) (*entity.APIKey, error) {
	return nil, nil
}
func (m *mockKeyRepo) ListKeysByTenant(ctx context.Context, tenantID string) ([]*entity.APIKey, error) {
	return nil, nil
}
func (m *mockKeyRepo) UpdateKeyStatus(ctx context.Context, keyHash string, status entity.KeyStatus, isActive bool) error {
	return nil
}
func (m *mockKeyRepo) UpdateKeySettings(ctx context.Context, keyHash string, input entity.UpdateKeyInput) (*entity.APIKey, error) {
	return nil, nil
}
func (m *mockKeyRepo) RotateKey(ctx context.Context, oldKeyHash, newKeyHash, newKeyPrefix string, gracePeriodExpiresAt time.Time) (*entity.APIKey, error) {
	return nil, nil
}
func (m *mockKeyRepo) DeleteKey(ctx context.Context, keyHash string) error { return nil }
func (m *mockKeyRepo) IncrementMonthlyUsage(ctx context.Context, keyHash string, month string, increment int64) (int64, error) {
	return 1, nil
}
func (m *mockKeyRepo) UpdateLastUsedAt(ctx context.Context, keyHash string, lastUsed time.Time) error {
	return nil
}
func (m *mockKeyRepo) Ping(ctx context.Context) error { return nil }

func setupTestMultiProxy(t *testing.T, routes []*RouteConfig, defaultBackend http.HandlerFunc) (*MultiTargetProxy, *mockKeyRepo) {
	repo := &mockKeyRepo{keys: make(map[string]*entity.APIKey)}
	limiter := ratelimit.NewSlidingWindowLimiter(time.Minute)
	vUsecase := usecase.NewVerifyUsecase(repo, limiter)

	var defURL string
	if defaultBackend != nil {
		ts := httptest.NewServer(defaultBackend)
		t.Cleanup(ts.Close)
		defURL = ts.URL
	}

	proxy, err := NewMultiTargetProxy(routes, defURL, vUsecase)
	if err != nil {
		t.Fatalf("failed to create MultiTargetProxy: %v", err)
	}

	return proxy, repo
}

func TestMultiTargetProxy_RoutingAndStripPrefix(t *testing.T) {
	var llmReceivedPath, mcpReceivedPath, aiReceivedPath string

	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		llmReceivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(llmServer.Close)

	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mcpReceivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(mcpServer.Close)

	aiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		aiReceivedPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(aiServer.Close)

	routes := []*RouteConfig{
		{Prefix: "/llm", Target: llmServer.URL, Scope: "llm:*", StripPrefix: true},
		{Prefix: "/mcp", Target: mcpServer.URL, Scope: "mcp:*", StripPrefix: true},
		{Prefix: "/ai", Target: aiServer.URL, Scope: "ai:*", StripPrefix: true},
	}

	proxy, repo := setupTestMultiProxy(t, routes, nil)

	// 万能キー (全スコープ許可)
	rawKey := "tlge-live-allpowerfull1234567890"
	repo.keys[usecase.HashKey(rawKey)] = &entity.APIKey{
		KeyID:        "key-admin",
		TenantID:     "tenant-all",
		IsActive:     true,
		Status:       entity.StatusActive,
		Scopes:       []string{"*"},
		RateLimitRPM: 1000,
	}

	// 1. /llm/v1/chat/completions -> llmServer に /v1/chat/completions として届く
	req1 := httptest.NewRequest("POST", "/llm/v1/chat/completions", nil)
	req1.Header.Set("Authorization", "Bearer "+rawKey)
	rec1 := httptest.NewRecorder()
	proxy.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Errorf("/llm request failed with status: %d", rec1.Code)
	}
	if llmReceivedPath != "/v1/chat/completions" {
		t.Errorf("expected stripped path '/v1/chat/completions', got: %s", llmReceivedPath)
	}

	// 2. /mcp/tools/list -> mcpServer に /tools/list として届く
	req2 := httptest.NewRequest("GET", "/mcp/tools/list", nil)
	req2.Header.Set("X-API-Key", rawKey)
	rec2 := httptest.NewRecorder()
	proxy.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusOK {
		t.Errorf("/mcp request failed with status: %d", rec2.Code)
	}
	if mcpReceivedPath != "/tools/list" {
		t.Errorf("expected stripped path '/tools/list', got: %s", mcpReceivedPath)
	}

	// 3. /ai/workflows/run -> aiServer に /workflows/run として届く
	req3 := httptest.NewRequest("POST", "/ai/workflows/run", nil)
	req3.Header.Set("Authorization", "Bearer "+rawKey)
	rec3 := httptest.NewRecorder()
	proxy.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusOK {
		t.Errorf("/ai request failed with status: %d", rec3.Code)
	}
	if aiReceivedPath != "/workflows/run" {
		t.Errorf("expected stripped path '/workflows/run', got: %s", aiReceivedPath)
	}
}

func TestMultiTargetProxy_ScopeEnforcement(t *testing.T) {
	mcpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(mcpServer.Close)

	routes := []*RouteConfig{
		{Prefix: "/mcp", Target: mcpServer.URL, Scope: "mcp:*", StripPrefix: true},
	}

	proxy, repo := setupTestMultiProxy(t, routes, nil)

	// llm:* のみを持つキー (mcp:* を持たない)
	llmOnlyKey := "tlge-live-llmonly1234567890abcdef"
	repo.keys[usecase.HashKey(llmOnlyKey)] = &entity.APIKey{
		KeyID:        "key-llm",
		TenantID:     "tenant-llm",
		IsActive:     true,
		Status:       entity.StatusActive,
		Scopes:       []string{"llm:*"},
		RateLimitRPM: 100,
	}

	// /mcp を叩く -> 403 Forbidden になるべき
	req := httptest.NewRequest("GET", "/mcp/tools", nil)
	req.Header.Set("Authorization", "Bearer "+llmOnlyKey)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected status %d (Forbidden) for scope mismatch, got: %d", http.StatusForbidden, rec.Code)
	}
}

func TestMultiTargetProxy_NotFoundWhenNoRouteMatches(t *testing.T) {
	routes := []*RouteConfig{
		{Prefix: "/llm", Target: "http://dummy", Scope: "llm:*"},
	}
	proxy, _ := setupTestMultiProxy(t, routes, nil)

	req := httptest.NewRequest("GET", "/unknown/service", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status %d for unmapped route, got: %d", http.StatusNotFound, rec.Code)
	}
}

func TestWriteJSONError(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSONError(rec, http.StatusBadRequest, "bad_request", "Invalid input data", "validation_failed")

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if contentType != "application/json; charset=utf-8" {
		t.Errorf("expected content-type 'application/json; charset=utf-8', got %q", contentType)
	}

	expectedJSON := `{"error":"bad_request","message":"Invalid input data","reason":"validation_failed"}
`
	if rec.Body.String() != expectedJSON {
		t.Errorf("expected body %q, got %q", expectedJSON, rec.Body.String())
	}
}

func TestWriteJSONError_Omitempty(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSONError(rec, http.StatusInternalServerError, "internal_error", "An internal error occurred", "")

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, rec.Code)
	}

	expectedJSON := `{"error":"internal_error","message":"An internal error occurred"}
`
	if rec.Body.String() != expectedJSON {
		t.Errorf("expected body %q, got %q", expectedJSON, rec.Body.String())
	}
}

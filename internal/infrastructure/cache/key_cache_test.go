package cache

import (
	"context"
	"testing"
	"time"

	"github.com/northfieldzz/tollgate/internal/domain/entity"
	"github.com/northfieldzz/tollgate/internal/domain/repository"
)

type mockRepo struct {
	repository.KeyRepository
	getCallCount int
	keys         map[string]*entity.APIKey
}

func (m *mockRepo) GetKeyByHash(ctx context.Context, keyHash string) (*entity.APIKey, error) {
	m.getCallCount++
	key, ok := m.keys[keyHash]
	if !ok {
		return nil, nil
	}
	return key, nil
}

func (m *mockRepo) UpdateKeyStatus(ctx context.Context, keyHash string, status entity.KeyStatus, isActive bool) error {
	if key, ok := m.keys[keyHash]; ok {
		key.Status = status
		key.IsActive = isActive
	}
	return nil
}

func TestCachedKeyRepository(t *testing.T) {
	mock := &mockRepo{
		keys: map[string]*entity.APIKey{
			"hash123": {
				PK:       "KEY#hash123",
				KeyID:    "key-1",
				IsActive: true,
				Status:   entity.StatusActive,
			},
		},
	}

	cachedRepo := NewCachedKeyRepository(mock, 100*time.Millisecond)

	ctx := context.Background()

	// 1回目: キャッシュミス -> underlying 呼び出し
	k1, err := cachedRepo.GetKeyByHash(ctx, "hash123")
	if err != nil || k1 == nil {
		t.Fatalf("expected key, got %v, err: %v", k1, err)
	}
	if mock.getCallCount != 1 {
		t.Errorf("expected 1 call, got %d", mock.getCallCount)
	}

	// 2回目: キャッシュヒット -> underlying は呼ばれない
	k2, err := cachedRepo.GetKeyByHash(ctx, "hash123")
	if err != nil || k2 == nil {
		t.Fatalf("expected key, got %v, err: %v", k2, err)
	}
	if mock.getCallCount != 1 {
		t.Errorf("expected still 1 call, got %d", mock.getCallCount)
	}

	// ステータス更新でキャッシュ無効化されることを検証
	_ = cachedRepo.UpdateKeyStatus(ctx, "hash123", entity.StatusSuspended, false)

	// 3回目: キャッシュ無効化されているため再度 underlying 呼び出し
	k3, err := cachedRepo.GetKeyByHash(ctx, "hash123")
	if err != nil || k3 == nil {
		t.Fatalf("expected key, got %v, err: %v", k3, err)
	}
	if mock.getCallCount != 2 {
		t.Errorf("expected 2 calls after invalidation, got %d", mock.getCallCount)
	}
	if k3.Status != entity.StatusSuspended {
		t.Errorf("expected status suspended, got %s", k3.Status)
	}
}

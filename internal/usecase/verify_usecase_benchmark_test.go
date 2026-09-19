package usecase

import (
	"testing"
)

func BenchmarkMatchScope_Hit(b *testing.B) {
	required := "users:read:123"
	allowedScopes := []string{"system:admin", "users:write", "users:read:*", "users:*"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchScope(required, allowedScopes)
	}
}

func BenchmarkMatchScope_Miss(b *testing.B) {
	required := "users:read:123"
	allowedScopes := []string{"system:admin", "posts:read", "comments:*", "audit:*"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchScope(required, allowedScopes)
	}
}

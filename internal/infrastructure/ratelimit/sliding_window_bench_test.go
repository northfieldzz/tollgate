package ratelimit

import (
	"testing"
	"time"
)

func BenchmarkSlidingWindowLimiter_Allow(b *testing.B) {
	limiter := NewSlidingWindowLimiter(time.Second)
	defer limiter.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Allow("bench-key", 1000)
	}
}

func BenchmarkSlidingWindowLimiter_Concurrent(b *testing.B) {
	limiter := NewSlidingWindowLimiter(time.Second)
	defer limiter.Stop()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			limiter.Allow("bench-key-concurrent", 1000000)
		}
	})
}

package ratelimit

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func BenchmarkCleanupContention(b *testing.B) {
	limiter := NewSlidingWindowLimiter(time.Minute)

	// Populate with 100,000 keys to make the map large
	numKeys := 100000
	for i := 0; i < numKeys; i++ {
		id := fmt.Sprintf("key-%d", i)
		limiter.Allow(id, 1000)
	}

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				// Trigger the loop logic manually to simulate it running often
				now := time.Now()
				cutoff := now.Add(-limiter.window)
				for _, s := range limiter.shards {
					s.mu.Lock()
					keys := make([]string, 0, len(s.windows))
					for id := range s.windows {
						keys = append(keys, id)
					}
					s.mu.Unlock()

					for _, id := range keys {
						s.mu.Lock()
						if timestamps, exists := s.windows[id]; exists {
							validStart := 0
							for validStart < len(timestamps) && !timestamps[validStart].After(cutoff) {
								validStart++
							}
							if validStart >= len(timestamps) {
								delete(s.windows, id)
							} else if validStart > 0 {
								copy(timestamps, timestamps[validStart:])
								s.windows[id] = timestamps[:len(timestamps)-validStart]
							}
						}
						s.mu.Unlock()
					}
					// Small yield to let tests run
					time.Sleep(10 * time.Microsecond)
				}
			}
		}
	}()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for pb.Next() {
			id := fmt.Sprintf("key-%d", r.Intn(numKeys))
			limiter.Allow(id, 1000)
		}
	})
	close(stop)
}

package ratelimit

import (
	"sync"
	"time"
)

// SlidingWindowLimiter は高並行・Goroutine セーフなインメモリ・スライディングウィンドウ式レートリミッター
type SlidingWindowLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
	window  time.Duration
}

func NewSlidingWindowLimiter(window time.Duration) *SlidingWindowLimiter {
	if window <= 0 {
		window = time.Minute
	}
	limiter := &SlidingWindowLimiter{
		windows: make(map[string][]time.Time),
		window:  window,
	}

	// 定期的な古いキーのガベージコレクション (5分おき)
	go limiter.cleanupLoop(5 * time.Minute)

	return limiter
}

// Allow は指定された識別子 (KeyID 等) のリクエストを 1 件消費し、許可可否、残り件数、リセット所要時間を返す
func (l *SlidingWindowLimiter) Allow(id string, limitRPM int) (bool, int, time.Duration) {
	if limitRPM <= 0 {
		// 上限が 0 以下の場合は無制限
		return true, 999999, 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	timestamps := l.windows[id]
	valid := make([]time.Time, 0, len(timestamps)+1)

	// ウィンドウ外の古いタイムスタンプを破棄
	for _, t := range timestamps {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= limitRPM {
		// 上限到達
		oldest := valid[0]
		resetIn := oldest.Add(l.window).Sub(now)
		if resetIn < 0 {
			resetIn = time.Millisecond
		}
		l.windows[id] = valid
		return false, 0, resetIn
	}

	// 許可: 現在時刻を追加
	valid = append(valid, now)
	l.windows[id] = valid

	remaining := limitRPM - len(valid)
	return true, remaining, l.window
}

func (l *SlidingWindowLimiter) cleanupLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		cutoff := now.Add(-l.window)
		for id, timestamps := range l.windows {
			valid := make([]time.Time, 0, len(timestamps))
			for _, t := range timestamps {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(l.windows, id)
			} else {
				l.windows[id] = valid
			}
		}
		l.mu.Unlock()
	}
}

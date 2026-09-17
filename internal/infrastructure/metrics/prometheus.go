package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// VerificationsTotal はキー検証リクエスト総数カウンター
	VerificationsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "itcp_api_keys_verifications_total",
			Help: "Total number of API key verification requests",
		},
		[]string{"tenant", "status", "reason"},
	)

	// RateLimitExceededTotal はレートリミットまたは月間クォータ超過カウンター
	RateLimitExceededTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "itcp_api_keys_rate_limit_exceeded_total",
			Help: "Total number of rate limit or quota exceeded rejections",
		},
		[]string{"tenant", "key_prefix"},
	)

	// VerificationDuration は検証処理レイテンシのヒストグラム
	VerificationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "itcp_api_keys_verification_duration_seconds",
			Help:    "Latency of API key verification in seconds",
			Buckets: []float64{0.0005, 0.001, 0.002, 0.005, 0.01, 0.025, 0.05, 0.1},
		},
		[]string{"tenant"},
	)
)

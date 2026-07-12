package dispatcher

import (
	"math"
	"time"
)

// Compute delay before attempt
func Backoff(n int, base, max time.Duration) time.Duration {
	delay := time.Duration(float64(base) * math.Pow(2, float64(n)))
	if delay > max {
		return max
	}
	return delay
}

func ptr[T any](v T) *T { return &v }
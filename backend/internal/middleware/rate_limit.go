package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"
	"time"
)

type rateBucket struct {
	tokens float64
	last   time.Time
}

// AuthRateLimit limits all auth endpoints together per connecting IP.
func AuthRateLimit(capacity int, period time.Duration) func(http.Handler) http.Handler {
	var mu sync.Mutex
	buckets := map[string]rateBucket{}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}
			now := time.Now()
			mu.Lock()
			bucket, found := buckets[ip]
			if !found {
				bucket = rateBucket{tokens: float64(capacity), last: now}
			}
			bucket.tokens += now.Sub(bucket.last).Seconds() * float64(capacity) / period.Seconds()
			if bucket.tokens > float64(capacity) {
				bucket.tokens = float64(capacity)
			}
			bucket.last = now
			allowed := bucket.tokens >= 1
			if allowed {
				bucket.tokens--
			}
			buckets[ip] = bucket
			if len(buckets) > 1000 {
				for key, value := range buckets {
					if now.Sub(value.last) > period {
						delete(buckets, key)
					}
				}
			}
			mu.Unlock()
			if !allowed {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "6")
				w.WriteHeader(http.StatusTooManyRequests)
				_ = json.NewEncoder(w).Encode(map[string]string{"error": "too many auth requests"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

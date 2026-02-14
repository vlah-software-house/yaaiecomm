package middleware

import (
	"encoding/json"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// bucket represents a token bucket for a single client (IP address).
// It tracks the number of available tokens and the last time tokens were refilled.
type bucket struct {
	tokens     float64
	lastRefill time.Time
	mu         sync.Mutex
}

// rateLimiterState holds the shared state for a rate limiter instance.
type rateLimiterState struct {
	buckets sync.Map // map[string]*bucket (IP -> bucket)
	rate    float64  // tokens added per second
	burst   int      // maximum tokens (bucket capacity)
	done    chan struct{}
}

// healthCheckPaths are endpoints exempt from rate limiting.
var healthCheckPaths = map[string]bool{
	"/healthz":  true,
	"/readyz":   true,
	"/livez":    true,
	"/health":   true,
	"/ready":    true,
	"/ping":     true,
}

// allow checks whether a request from the given IP should be allowed.
// It refills tokens based on elapsed time, then attempts to consume one token.
// Returns (allowed, remaining, limit).
func (s *rateLimiterState) allow(ip string) (bool, int, int) {
	val, _ := s.buckets.LoadOrStore(ip, &bucket{
		tokens:     float64(s.burst),
		lastRefill: time.Now(),
	})
	b := val.(*bucket)

	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * s.rate
	if b.tokens > float64(s.burst) {
		b.tokens = float64(s.burst)
	}
	b.lastRefill = now

	if b.tokens >= 1.0 {
		b.tokens--
		remaining := int(math.Floor(b.tokens))
		return true, remaining, s.burst
	}

	return false, 0, s.burst
}

// retryAfter estimates how many seconds until one token is available.
func (s *rateLimiterState) retryAfter(ip string) int {
	val, ok := s.buckets.Load(ip)
	if !ok {
		return 1
	}
	b := val.(*bucket)

	b.mu.Lock()
	defer b.mu.Unlock()

	if b.tokens >= 1.0 {
		return 0
	}

	deficit := 1.0 - b.tokens
	seconds := deficit / s.rate
	return int(math.Ceil(seconds))
}

// startCleanup launches a background goroutine that removes stale buckets
// every 5 minutes. A bucket is considered stale if it has been idle long
// enough for its tokens to have fully refilled (i.e., it is at capacity
// and has not been used recently).
func (s *rateLimiterState) startCleanup() {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				staleThreshold := time.Now().Add(-10 * time.Minute)
				s.buckets.Range(func(key, value any) bool {
					b := value.(*bucket)
					b.mu.Lock()
					if b.lastRefill.Before(staleThreshold) {
						b.mu.Unlock()
						s.buckets.Delete(key)
					} else {
						b.mu.Unlock()
					}
					return true
				})
			case <-s.done:
				return
			}
		}
	}()
}

// RateLimiter creates middleware that limits requests per IP address using a
// token bucket algorithm with configurable rate and burst.
//
// Parameters:
//   - rate: tokens added per second (e.g., 10 means 10 requests/sec sustained)
//   - burst: maximum bucket size (allows initial spike up to this many requests)
//
// Usage:
//
//	ratelimited := middleware.RateLimiter(10, 20) // 10 req/sec, burst 20
//	mux.Handle("/api/v1/", ratelimited(apiHandler))
func RateLimiter(rate float64, burst int) func(http.Handler) http.Handler {
	state := &rateLimiterState{
		rate:  rate,
		burst: burst,
		done:  make(chan struct{}),
	}
	state.startCleanup()

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Exempt health check endpoints.
			if healthCheckPaths[r.URL.Path] {
				next.ServeHTTP(w, r)
				return
			}

			ip := extractIP(r)

			allowed, remaining, limit := state.allow(ip)

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))

			if !allowed {
				retryAfter := state.retryAfter(ip)
				w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusTooManyRequests)
				resp := map[string]string{
					"error":   "rate limit exceeded",
					"message": "Too many requests. Please try again later.",
				}
				json.NewEncoder(w).Encode(resp)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// LoginRateLimiter returns a stricter rate limiter suitable for login endpoints:
// 5 attempts per minute per IP (rate = 5/60 ~= 0.0833 tokens/sec, burst = 5).
func LoginRateLimiter() func(http.Handler) http.Handler {
	return RateLimiter(5.0/60.0, 5)
}

// extractIP retrieves the client IP from the request, preferring
// X-Forwarded-For and X-Real-IP headers (for reverse proxy setups),
// and falling back to RemoteAddr.
func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first (may contain comma-separated list).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (leftmost) IP, which is the original client.
		parts := strings.SplitN(xff, ",", 2)
		ip := strings.TrimSpace(parts[0])
		if ip != "" {
			return ip
		}
	}

	// Check X-Real-IP.
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr, stripping the port.
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

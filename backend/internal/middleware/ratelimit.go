package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"

	"github.com/groupify/groupify/backend/internal/domain"
	"github.com/groupify/groupify/backend/internal/httpx"
)

// Limiter is a per-key token bucket. Keys are client IPs. It is intentionally
// in-memory: the PRD rules out Redis for the MVP, and a single small API
// instance is the deployment target. Behind multiple replicas the effective
// limit becomes per-replica, which is documented in docs/deployment.md.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   rate.Limit
	burst   int
	maxKeys int
}

type bucket struct {
	lim  *rate.Limiter
	seen time.Time
}

// NewLimiter allows perMinute requests with a burst equal to perMinute.
func NewLimiter(perMinute int) *Limiter {
	if perMinute <= 0 {
		perMinute = 60
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		limit:   rate.Limit(float64(perMinute) / 60.0),
		burst:   perMinute,
		maxKeys: 50_000,
	}
}

// Allow reports whether the key may proceed and records the attempt.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	if len(l.buckets) > l.maxKeys {
		l.pruneLocked(now)
	}

	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{lim: rate.NewLimiter(l.limit, l.burst)}
		l.buckets[key] = b
	}
	b.seen = now
	return b.lim.AllowN(now, 1)
}

// pruneLocked drops buckets that have been idle long enough to be full again.
func (l *Limiter) pruneLocked(now time.Time) {
	idle := time.Duration(float64(time.Second) * float64(l.burst) / float64(l.limit))
	if idle < time.Minute {
		idle = time.Minute
	}
	for k, b := range l.buckets {
		if now.Sub(b.seen) > idle {
			delete(l.buckets, k)
		}
	}
}

// Middleware rejects requests over the limit with 429.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !l.Allow(ClientIP(r)) {
			w.Header().Set("Retry-After", "60")
			httpx.Fail(w, r, domain.ErrRateLimited)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ClientIP returns the best available client address. X-Forwarded-For is only
// consulted when the deployment explicitly opts in via chi's RealIP middleware
// (TRUST_PROXY_HEADERS), because a spoofed header would let a client pick a
// fresh rate-limit bucket per request.
func ClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return strings.ToLower(host)
}

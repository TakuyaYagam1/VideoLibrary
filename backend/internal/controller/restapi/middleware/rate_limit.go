package middleware

import (
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wahrwelt-kit/go-httpkit/httperr"
	httpkit "github.com/wahrwelt-kit/go-httpkit/httputil"
)

type rateLimiter struct {
	mu      sync.Mutex
	limit   int
	window  time.Duration
	entries map[string]rateLimitEntry
	now     func() time.Time
}

const rateLimitMaxEntries = 4096

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

func NewRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		limit:   limit,
		window:  window,
		entries: make(map[string]rateLimitEntry),
		now:     time.Now,
	}
}

func (l *rateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := requestClientIP(r) + " " + r.URL.Path
		if !l.allow(key) {
			httpkit.HandleError(w, r, httperr.ErrTooManyRequests())
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (l *rateLimiter) allow(key string) bool {
	now := l.now()

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, exists := l.entries[key]
	if entry.resetAt.IsZero() || !now.Before(entry.resetAt) {
		if !exists && len(l.entries) >= rateLimitMaxEntries {
			l.cleanupExpired(now)
			if len(l.entries) >= rateLimitMaxEntries {
				return false
			}
		}

		l.entries[key] = rateLimitEntry{
			count:   1,
			resetAt: now.Add(l.window),
		}

		return true
	}

	entry.count++
	l.entries[key] = entry

	return entry.count <= l.limit
}

func (l *rateLimiter) cleanupExpired(now time.Time) {
	if len(l.entries) < 1024 {
		return
	}

	for key, entry := range l.entries {
		if !now.Before(entry.resetAt) {
			delete(l.entries, key)
		}
	}
}

func requestClientIP(r *http.Request) string {
	remoteIP := remoteAddrIP(r.RemoteAddr)
	if isTrustedForwarder(remoteIP) {
		forwardedFor := r.Header.Get("X-Forwarded-For")
		if forwardedFor != "" {
			ip := strings.TrimSpace(strings.Split(forwardedFor, ",")[0])
			if net.ParseIP(ip) != nil {
				return ip
			}
		}
	}

	if remoteIP != "" {
		return remoteIP
	}

	return r.RemoteAddr
}

func remoteAddrIP(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}

	if net.ParseIP(remoteAddr) != nil {
		return remoteAddr
	}

	return ""
}

func isTrustedForwarder(remoteIP string) bool {
	ip := net.ParseIP(remoteIP)
	if ip == nil {
		return false
	}

	return ip.IsLoopback() || ip.IsPrivate()
}

package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// NewStructuredLogger creates a chi middleware that logs requests using slog.
func NewStructuredLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

			defer func() {
				status := ww.Status()
				duration := time.Since(start)

				attrs := []slog.Attr{
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Int("status", status),
					slog.String("duration", duration.String()),
					slog.Int("bytes", ww.BytesWritten()),
					slog.String("remote", r.RemoteAddr),
					slog.String("request_id", middleware.GetReqID(r.Context())),
				}

				level := slog.LevelInfo
				if status >= 500 {
					level = slog.LevelError
				} else if status >= 400 {
					level = slog.LevelWarn
				}

				logger.LogAttrs(r.Context(), level, "http request",
					attrs...,
				)
			}()

			next.ServeHTTP(ww, r)
		})
	}
}

// RateLimitMiddleware creates per-IP rate limiting middleware using Redis.
func (s *Server) RateLimitMiddleware(requestsPerMinute int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := r.RemoteAddr
			key := fmt.Sprintf("%s:%s", ip, r.URL.Path)

			allowed, err := s.redis.CheckRateLimit(r.Context(), key, requestsPerMinute, time.Minute)
			if err != nil {
				// If Redis is down, allow the request (graceful degradation)
				s.logger.Warn("rate limit check failed", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			if !allowed {
				http.Error(w, `{"error": "rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

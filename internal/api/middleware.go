package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	jwtauth "github.com/AtharvGupta360/JobCrawl/internal/auth"
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

// ─────────────────────────────────────────────
// JWT Auth Middleware
// ─────────────────────────────────────────────

type ctxKey string

const ctxKeyClaims ctxKey = "claims"

// JWTMiddleware validates Bearer tokens and injects Claims into the request context.
// Returns 401 if the token is missing or invalid.
func (s *Server) JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractBearerToken(r)
		if token == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing or invalid Authorization header"})
			return
		}

		claims, err := jwtauth.ValidateToken(s.jwtSecret, token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			return
		}

		ctx := context.WithValue(r.Context(), ctxKeyClaims, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminOnly middleware ensures the authenticated user has the 'admin' role.
// Must be used after JWTMiddleware.
func (s *Server) AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromCtx(r.Context())
		if !ok || claims.Role != "admin" {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "admin access required"})
			return
		}
		next.ServeHTTP(w, r)
	})
}

// extractBearerToken reads the token from "Authorization: Bearer <token>" header.
func extractBearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if h == "" {
		return ""
	}
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

// claimsFromCtx retrieves JWT Claims from the request context.
// Returns nil, false if not present (should not happen on JWT-protected routes).
func claimsFromCtx(ctx context.Context) (*jwtauth.Claims, bool) {
	c, ok := ctx.Value(ctxKeyClaims).(*jwtauth.Claims)
	return c, ok
}

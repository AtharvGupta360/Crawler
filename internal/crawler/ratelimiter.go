package crawler

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// RateLimiter enforces per-domain request rate limits to be a good citizen
// when crawling ATS platforms.
type RateLimiter struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	rate     rate.Limit // requests per second
	burst    int
	logger   *slog.Logger
}

// NewRateLimiter creates a rate limiter with the specified requests-per-second and burst size.
func NewRateLimiter(rps float64, burst int, logger *slog.Logger) *RateLimiter {
	return &RateLimiter{
		limiters: make(map[string]*rate.Limiter),
		rate:     rate.Limit(rps),
		burst:    burst,
		logger:   logger,
	}
}

// Wait blocks until the rate limiter allows a request to the given domain.
// Returns an error if the context is cancelled.
func (rl *RateLimiter) Wait(ctx context.Context, domain string) error {
	limiter := rl.getLimiter(domain)
	rl.logger.Debug("rate limiter waiting",
		"domain", domain,
		"tokens", limiter.Tokens(),
	)
	return limiter.Wait(ctx)
}

// Allow checks if a request is allowed without blocking.
func (rl *RateLimiter) Allow(domain string) bool {
	return rl.getLimiter(domain).Allow()
}

// getLimiter returns the rate limiter for a domain, creating one if needed.
func (rl *RateLimiter) getLimiter(domain string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[domain]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = rl.limiters[domain]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rl.rate, rl.burst)
	rl.limiters[domain] = limiter
	rl.logger.Info("created rate limiter", "domain", domain, "rps", rl.rate, "burst", rl.burst)
	return limiter
}

// SetDomainRate overrides the rate limit for a specific domain.
func (rl *RateLimiter) SetDomainRate(domain string, rps float64, burst int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.limiters[domain] = rate.NewLimiter(rate.Limit(rps), burst)
	rl.logger.Info("set custom rate limit", "domain", domain, "rps", rps, "burst", burst)
}

// CircuitBreaker tracks failures for a domain and temporarily blocks requests
// after too many consecutive failures.
type CircuitBreaker struct {
	mu            sync.RWMutex
	failures      map[string]int
	lastFailure   map[string]time.Time
	openUntil     map[string]time.Time
	maxFailures   int
	cooldown      time.Duration
	logger        *slog.Logger
}

// NewCircuitBreaker creates a circuit breaker that opens after maxFailures
// consecutive failures and stays open for the cooldown duration.
func NewCircuitBreaker(maxFailures int, cooldown time.Duration, logger *slog.Logger) *CircuitBreaker {
	return &CircuitBreaker{
		failures:    make(map[string]int),
		lastFailure: make(map[string]time.Time),
		openUntil:   make(map[string]time.Time),
		maxFailures: maxFailures,
		cooldown:    cooldown,
		logger:      logger,
	}
}

// IsOpen returns true if the circuit breaker is open (requests should be blocked).
func (cb *CircuitBreaker) IsOpen(domain string) bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	until, exists := cb.openUntil[domain]
	if !exists {
		return false
	}
	if time.Now().After(until) {
		return false // cooldown expired, allow retry
	}
	return true
}

// RecordSuccess resets the failure counter for a domain.
func (cb *CircuitBreaker) RecordSuccess(domain string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	delete(cb.failures, domain)
	delete(cb.lastFailure, domain)
	delete(cb.openUntil, domain)
}

// RecordFailure increments the failure counter and opens the breaker if threshold is hit.
func (cb *CircuitBreaker) RecordFailure(domain string) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures[domain]++
	cb.lastFailure[domain] = time.Now()

	if cb.failures[domain] >= cb.maxFailures {
		cb.openUntil[domain] = time.Now().Add(cb.cooldown)
		cb.logger.Warn("circuit breaker opened",
			"domain", domain,
			"failures", cb.failures[domain],
			"cooldown", cb.cooldown,
		)
	}
}

// Status returns the current state of the circuit breaker for a domain.
func (cb *CircuitBreaker) Status(domain string) (isOpen bool, failures int, cooldownRemaining time.Duration) {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	failures = cb.failures[domain]
	until, exists := cb.openUntil[domain]
	if exists && time.Now().Before(until) {
		return true, failures, time.Until(until)
	}
	return false, failures, 0
}

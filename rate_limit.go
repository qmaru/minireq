package minireq

import (
	"context"
	"errors"
	"sync"
	"time"
)

type fixedRateLimiter struct {
	interval time.Duration
	mu       sync.Mutex
	next     time.Time
}

// RateLimitFixed returns a limiter that enforces a minimum interval between requests.
func RateLimitFixed(interval time.Duration) RateLimiter {
	return &fixedRateLimiter{
		interval: nonNegativeDelay(interval),
	}
}

// RateLimitFromRPM converts requests per minute into a fixed-interval limiter.
func RateLimitFromRPM(rpm int) (RateLimiter, error) {
	interval, err := RPMToMinInterval(rpm)
	if err != nil {
		return nil, err
	}
	return RateLimitFixed(interval), nil
}

// RateLimitFromRPS converts requests per second into a fixed-interval limiter.
func RateLimitFromRPS(rps int) (RateLimiter, error) {
	if rps <= 0 {
		return nil, errors.New("rps must be > 0")
	}
	return RateLimitFixed(time.Second / time.Duration(rps)), nil
}

func (l *fixedRateLimiter) Wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if l == nil || l.interval <= 0 {
		return nil
	}

	now := time.Now()

	l.mu.Lock()
	slot := now
	if l.next.After(now) {
		slot = l.next
	}
	l.next = slot.Add(l.interval)
	l.mu.Unlock()

	wait := time.Until(slot)
	if wait <= 0 {
		return nil
	}

	timer := time.NewTimer(wait)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

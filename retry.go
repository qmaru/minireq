package minireq

import (
	"errors"
	"math/rand"
	"net/http"
	"time"
)

var defaultRetryPolicy = RetryPolicyWithStatusRange(500, 599)

var defaultRetryDelay = RetryExponentialDelay(100*time.Millisecond, 0.1)

var defaultOnRetry = func(event RetryEvent) {}

type RetryEvent struct {
	Attempt int
	Status  int
	Err     error
	Delay   time.Duration
}

type RetryConfig struct {
	MaxRetries  int
	RetryDelay  RetryDelay
	RetryPolicy RetryPolicy
	OnRetry     OnRetry
}

// RPMToMinInterval converts requests per minute to minimum interval between requests.
func RPMToMinInterval(rpm int) (time.Duration, error) {
	if rpm <= 0 {
		return 0, errors.New("rpm must be > 0")
	}
	return time.Minute / time.Duration(rpm), nil
}

// BackoffBaseForWindow calculates the base delay for exponential backoff
func BackoffBaseForWindow(window time.Duration, maxRetries int) (time.Duration, error) {
	if maxRetries <= 0 {
		return 0, errors.New("maxRetries must > 0")
	}

	return window / time.Duration(1<<(maxRetries-1)), nil
}

func NewRetryDefaultConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:  3,
		RetryPolicy: defaultRetryPolicy,
		RetryDelay:  defaultRetryDelay,
		OnRetry:     defaultOnRetry,
	}
}

func RetryFixedDelay(delay time.Duration) RetryDelay {
	return func(attempt int) time.Duration {
		return delay
	}
}

func RetryExponentialDelay(baseDelay time.Duration, jitterRatio float64) RetryDelay {
	return func(attempt int) time.Duration {
		if attempt <= 0 {
			attempt = 1
		}

		delay := baseDelay * (1 << (attempt - 1))
		if jitterRatio <= 0 {
			return delay
		}

		jitter := 1 + (rand.Float64()*2-1)*jitterRatio
		return time.Duration(float64(delay) * jitter)
	}
}

func RetryExponentialDelayFromRPM(rpm int, jitterRatio float64) (RetryDelay, error) {
	minInterval, err := RPMToMinInterval(rpm)
	if err != nil {
		return nil, err
	}

	return RetryExponentialDelay(minInterval, jitterRatio), nil
}

func RetryLinearDelay(baseDelay time.Duration) RetryDelay {
	return func(attempt int) time.Duration {
		if attempt <= 0 {
			attempt = 1
		}
		return baseDelay * time.Duration(attempt)
	}
}

func RetryNoDelay() func(attempt int) time.Duration {
	return func(attempt int) time.Duration {
		return 0
	}
}

func RetryPolicyWithStatusCodes(statusCodes ...int) RetryPolicy {
	allowed := make(map[int]struct{}, len(statusCodes))
	for _, code := range statusCodes {
		allowed[code] = struct{}{}
	}
	return func(resp *http.Response, err error) bool {
		if err != nil {
			return true
		}
		if resp == nil {
			return false
		}
		_, ok := allowed[resp.StatusCode]
		return ok
	}
}

func RetryPolicyWithStatusRange(min, max int) RetryPolicy {
	return func(resp *http.Response, err error) bool {
		if err != nil {
			return true
		}
		if resp == nil {
			return false
		}
		return resp.StatusCode >= min && resp.StatusCode <= max
	}
}

func RetryPolicyWithErrorCheck(check func(error) bool) RetryPolicy {
	return func(resp *http.Response, err error) bool {
		if err != nil {
			return check(err)
		}
		return false
	}
}

func RetryCombinePolicies(policies ...func(*http.Response, error) bool) func(*http.Response, error) bool {
	return func(resp *http.Response, err error) bool {
		for _, p := range policies {
			if p(resp, err) {
				return true
			}
		}
		return false
	}
}

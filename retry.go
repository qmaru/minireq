package minireq

import (
	"errors"
	"math/rand"
	"net/http"
	"time"
)

const maxDuration = time.Duration(1<<63 - 1)

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

func nonNegativeDelay(delay time.Duration) time.Duration {
	if delay < 0 {
		return 0
	}
	return delay
}

func multiplyDelay(baseDelay time.Duration, attempt int) time.Duration {
	if attempt > 63 {
		return maxDuration
	}

	multiplier := int64(1) << (attempt - 1)
	if int64(baseDelay) > (1<<63-1)/multiplier {
		return maxDuration
	}
	return baseDelay * time.Duration(multiplier)
}

func multiplyLinearDelay(baseDelay time.Duration, attempt int) time.Duration {
	if attempt <= 0 || baseDelay == 0 {
		return 0
	}
	if int64(baseDelay) > (1<<63-1)/int64(attempt) {
		return maxDuration
	}
	return baseDelay * time.Duration(attempt)
}

func applyJitter(delay time.Duration, jitter float64) time.Duration {
	if jitter <= 0 || delay <= 0 {
		return 0
	}
	if jitter > 1 && float64(delay) > float64(maxDuration)/jitter {
		return maxDuration
	}
	return time.Duration(float64(delay) * jitter)
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
	if maxRetries > 63 {
		return 0, errors.New("maxRetries is too large")
	}

	return window / time.Duration(int64(1)<<(maxRetries-1)), nil
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
		return nonNegativeDelay(delay)
	}
}

func RetryExponentialDelay(baseDelay time.Duration, jitterRatio float64) RetryDelay {
	return func(attempt int) time.Duration {
		if attempt <= 0 {
			attempt = 1
		}
		baseDelay = nonNegativeDelay(baseDelay)
		if baseDelay == 0 {
			return 0
		}

		delay := multiplyDelay(baseDelay, attempt)
		if jitterRatio <= 0 {
			return delay
		}
		if jitterRatio > 1 {
			jitterRatio = 1
		}

		jitter := 1 + (rand.Float64()*2-1)*jitterRatio
		return applyJitter(delay, jitter)
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
		return multiplyLinearDelay(nonNegativeDelay(baseDelay), attempt)
	}
}

func RetryNoDelay() RetryDelay {
	return func(attempt int) time.Duration {
		return 0
	}
}

func RetryPolicyWithStatusCode(statusCode int) RetryPolicy {
	return func(resp *http.Response, err error) bool {
		if resp == nil {
			return false
		}
		return statusCode == resp.StatusCode
	}
}

func RetryPolicyWithStatusCodes(statusCodes ...int) RetryPolicy {
	allowed := make(map[int]struct{}, len(statusCodes))
	for _, code := range statusCodes {
		allowed[code] = struct{}{}
	}
	return func(resp *http.Response, err error) bool {
		if resp == nil {
			return false
		}
		_, ok := allowed[resp.StatusCode]
		return ok
	}
}

func RetryPolicyWithStatusRange(min, max int) RetryPolicy {
	return func(resp *http.Response, err error) bool {
		if resp == nil {
			return false
		}
		return resp.StatusCode >= min && resp.StatusCode <= max
	}
}

func RetryPolicyWithErrorCheck(check func(error) bool) RetryPolicy {
	return func(resp *http.Response, err error) bool {
		if err != nil && check != nil {
			return check(err)
		}
		return false
	}
}

func RetryAny(policies ...RetryPolicy) RetryPolicy {
	return func(resp *http.Response, err error) bool {
		for _, p := range policies {
			if p == nil {
				continue
			}
			if p(resp, err) {
				return true
			}
		}
		return false
	}
}

func RetryAll(policies ...RetryPolicy) RetryPolicy {
	return func(resp *http.Response, err error) bool {
		if len(policies) == 0 {
			return false
		}

		for _, p := range policies {
			if p == nil {
				return false
			}
			if !p(resp, err) {
				return false
			}
		}
		return true
	}
}

func RetryDenyAllow(deny []RetryPolicy, allow []RetryPolicy) RetryPolicy {
	return func(resp *http.Response, err error) bool {
		for _, p := range deny {
			if p == nil {
				continue
			}
			if p(resp, err) {
				return false
			}
		}
		for _, p := range allow {
			if p == nil {
				continue
			}
			if p(resp, err) {
				return true
			}
		}
		return false
	}
}

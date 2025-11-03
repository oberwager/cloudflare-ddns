package retry

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/rand"
	"net"
	"net/http"
	"time"
)

var HTTPClient = &http.Client{
	Timeout: 30 * time.Second,
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       60 * time.Second,
	},
}

type Config struct {
	MaxRetries  int
	InitialWait time.Duration
	MaxWait     time.Duration
}

func DefaultConfig() Config {
	return Config{
		MaxRetries:  5,
		InitialWait: 1 * time.Second,
		MaxWait:     32 * time.Second,
	}
}

type Func func() error

func WithBackoff(ctx context.Context, operation string, config Config, fn Func) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * config.InitialWait
			if backoff > config.MaxWait {
				backoff = config.MaxWait
			}

			jitter := time.Duration(rand.Float64() * 0.5 * float64(backoff))
			if rand.Intn(2) == 0 {
				backoff += jitter
			} else {
				backoff -= jitter
			}

			slog.Warn("retrying operation",
				"operation", operation,
				"attempt", attempt,
				"max_attempts", config.MaxRetries,
				"backoff", backoff,
				"error", lastErr)

			select {
			case <-ctx.Done():
				return fmt.Errorf("context cancelled: %w", ctx.Err())
			case <-time.After(backoff):
			}
		}

		if err := fn(); err != nil {
			lastErr = err

			if ctx.Err() != nil {
				return fmt.Errorf("context cancelled during %s: %w", operation, ctx.Err())
			}

			if !isRetryableError(err) {
				return fmt.Errorf("%s: non-retryable error: %w", operation, err)
			}

			continue
		}

		if attempt > 0 {
			slog.Info("operation succeeded after retry",
				"operation", operation,
				"attempts", attempt+1)
		}
		return nil
	}

	return fmt.Errorf("%s: max retries (%d) exceeded: %w", operation, config.MaxRetries, lastErr)
}

func isRetryableError(err error) bool {
	if netErr, ok := err.(net.Error); ok {
		return netErr.Timeout() || netErr.Temporary()
	}

	errStr := err.Error()
	retryablePatterns := []string{
		"connection refused",
		"connection reset",
		"no such host",
		"timeout",
		"temporary failure",
		"too many open files",
		"i/o timeout",
	}

	for _, pattern := range retryablePatterns {
		if contains(errStr, pattern) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

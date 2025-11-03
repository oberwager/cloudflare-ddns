package retry

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxRetries != 5 {
		t.Errorf("expected MaxRetries=5, got %d", cfg.MaxRetries)
	}
	if cfg.InitialWait != 1*time.Second {
		t.Errorf("expected InitialWait=1s, got %v", cfg.InitialWait)
	}
	if cfg.MaxWait != 32*time.Second {
		t.Errorf("expected MaxWait=32s, got %v", cfg.MaxWait)
	}
}

func TestWithBackoffSuccess(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxRetries:  3,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     10 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return nil
	}

	err := WithBackoff(ctx, "test", cfg, fn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestRetryWithBackoffEventualSuccess(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxRetries:  3,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     10 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return &net.DNSError{IsTimeout: true}
		}
		return nil
	}

	err := WithBackoff(ctx, "test", cfg, fn)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls, got %d", callCount)
	}
}

func TestRetryWithBackoffMaxRetriesExceeded(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxRetries:  2,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     10 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return &net.DNSError{IsTimeout: true}
	}

	err := WithBackoff(ctx, "test", cfg, fn)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if callCount != 3 {
		t.Errorf("expected 3 calls (initial + 2 retries), got %d", callCount)
	}
}

func TestRetryWithBackoffContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := Config{
		MaxRetries:  3,
		InitialWait: 100 * time.Millisecond,
		MaxWait:     1 * time.Second,
	}

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 1 {
			cancel()
		}
		return &net.DNSError{IsTimeout: true}
	}

	err := WithBackoff(ctx, "test", cfg, fn)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
}

func TestRetryWithBackoffNonRetryableError(t *testing.T) {
	ctx := context.Background()
	cfg := Config{
		MaxRetries:  3,
		InitialWait: 1 * time.Millisecond,
		MaxWait:     10 * time.Millisecond,
	}

	callCount := 0
	fn := func() error {
		callCount++
		return errors.New("permanent error")
	}

	err := WithBackoff(ctx, "test", cfg, fn)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		retryable bool
	}{
		{
			name:      "timeout error",
			err:       &net.DNSError{IsTimeout: true},
			retryable: true,
		},
		{
			name:      "temporary error",
			err:       &net.DNSError{IsTemporary: true},
			retryable: true,
		},
		{
			name:      "connection refused",
			err:       errors.New("connection refused"),
			retryable: true,
		},
		{
			name:      "connection reset",
			err:       errors.New("connection reset by peer"),
			retryable: true,
		},
		{
			name:      "no such host",
			err:       errors.New("no such host"),
			retryable: true,
		},
		{
			name:      "timeout in message",
			err:       errors.New("request timeout"),
			retryable: true,
		},
		{
			name:      "i/o timeout",
			err:       errors.New("i/o timeout"),
			retryable: true,
		},
		{
			name:      "permanent error",
			err:       errors.New("permanent failure"),
			retryable: false,
		},
		{
			name:      "validation error",
			err:       errors.New("invalid input"),
			retryable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isRetryableError(tt.err)
			if result != tt.retryable {
				t.Errorf("expected %v, got %v", tt.retryable, result)
			}
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "lo wo", true},
		{"hello world", "xyz", false},
		{"test", "test", true},
		{"test", "testing", false},
		{"", "test", false},
		{"test", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := contains(tt.s, tt.substr)
			if result != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.want)
			}
		})
	}
}

func TestFindSubstring(t *testing.T) {
	tests := []struct {
		s      string
		substr string
		want   bool
	}{
		{"hello world", "world", true},
		{"hello world", "hello", true},
		{"hello world", "lo wo", true},
		{"hello world", "xyz", false},
		{"test", "test", true},
		{"test", "testing", false},
	}

	for _, tt := range tests {
		t.Run(tt.s+"_"+tt.substr, func(t *testing.T) {
			result := findSubstring(tt.s, tt.substr)
			if result != tt.want {
				t.Errorf("findSubstring(%q, %q) = %v, want %v", tt.s, tt.substr, result, tt.want)
			}
		})
	}
}

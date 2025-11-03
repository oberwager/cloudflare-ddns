package ip

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"github.com/oberwager/cloudflare-ddns/internal/retry"
)

func GetWithRetry(ctx context.Context, url string, isIPv6 bool) (string, error) {
	var result string
	config := retry.DefaultConfig()

	ipType := "IPv4"
	if isIPv6 {
		ipType = "IPv6"
	}

	err := retry.WithBackoff(ctx, fmt.Sprintf("get %s", ipType), config, func() error {
		ip, err := getIP(ctx, url)
		if err != nil {
			return err
		}

		if err := validateIP(ip, isIPv6); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}

		result = ip
		return nil
	})

	if err != nil {
		return "", err
	}

	return result, nil
}

func getIP(ctx context.Context, url string) (string, error) {
	slog.Debug("fetching ip address", "url", url)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	resp, err := retry.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	result := strings.TrimSpace(string(ip))
	if result == "" {
		return "", fmt.Errorf("empty response from %s", url)
	}

	return result, nil
}

func validateIP(ip string, isIPv6 bool) error {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return fmt.Errorf("invalid IP address: %s", ip)
	}

	if isIPv6 {
		if parsed.To4() != nil {
			return fmt.Errorf("expected IPv6 but got IPv4: %s", ip)
		}
	} else {
		if parsed.To4() == nil {
			return fmt.Errorf("expected IPv4 but got IPv6: %s", ip)
		}
	}

	return nil
}

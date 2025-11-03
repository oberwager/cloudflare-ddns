package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"sync"

	"github.com/oberwager/cloudflare-ddns/internal/cloudflare"
	"github.com/oberwager/cloudflare-ddns/internal/config"
	"github.com/oberwager/cloudflare-ddns/internal/ip"
)

var Version = "dev"

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	slog.Info("starting cloudflare-ddns", "version", Version)

	token := mustEnv("CF_API_TOKEN")
	configJSON := mustEnv("CF_CONFIG")
	ipv6Enabled := os.Getenv("CF_IPV6_ENABLED") == "true"

	var cfg config.Config
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		fatal("parse config", err)
	}

	if err := config.Validate(&cfg); err != nil {
		fatal("invalid config", err)
	}

	if cfg.DefaultTTL == 0 {
		cfg.DefaultTTL = 300
	}

	if cfg.ConcurrencyLimit == 0 {
		cfg.ConcurrencyLimit = 10
	}

	ctx := context.Background()

	ipv4, err := ip.GetWithRetry(ctx, "https://api.ipify.org", false)
	if err != nil {
		fatal("get IPv4", err)
	}
	slog.Info("detected public ip", "type", "ipv4", "ip", ipv4)

	var ipv6 string
	if ipv6Enabled {
		if ipv6, err = ip.GetWithRetry(ctx, "https://api6.ipify.org", true); err != nil {
			slog.Warn("ipv6 detection failed after retries", "error", err)
		} else {
			slog.Info("detected public ip", "type", "ipv6", "ip", ipv6)
		}
	}

	var wg sync.WaitGroup
	for _, zone := range cfg.Zones {
		wg.Add(1)
		go func(z config.Zone) {
			defer wg.Done()
			if err := cloudflare.ProcessZone(ctx, token, z, ipv4, ipv6, cfg.DefaultTTL, cfg.ConcurrencyLimit); err != nil {
				slog.Error("failed to process zone", "zone_id", z.ZoneID, "error", err)
			}
		}(zone)
	}
	wg.Wait()

	slog.Info("cloudflare-ddns completed successfully")
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		slog.Error("missing required env var", "key", key)
		os.Exit(1)
	}
	return val
}

func fatal(msg string, err error) {
	slog.Error("fatal error", "context", msg, "error", err)
	os.Exit(1)
}

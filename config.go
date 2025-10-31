package main

import "fmt"

type Subdomain struct {
	Name    string `json:"name"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl,omitempty"`
}

type Zone struct {
	ZoneID     string      `json:"zone_id"`
	Subdomains []Subdomain `json:"subdomains"`
	TTL        int         `json:"ttl,omitempty"`
}

type Config struct {
	Zones            []Zone `json:"zones"`
	DefaultTTL       int    `json:"default_ttl,omitempty"`
	ConcurrencyLimit int    `json:"concurrency_limit,omitempty"`
}

func validateConfig(cfg *Config) error {
	if len(cfg.Zones) == 0 {
		return fmt.Errorf("no zones configured")
	}

	for i, zone := range cfg.Zones {
		if zone.ZoneID == "" {
			return fmt.Errorf("zone[%d]: missing zone_id", i)
		}
		if len(zone.Subdomains) == 0 {
			return fmt.Errorf("zone[%d]: no subdomains configured", i)
		}
		for j, sub := range zone.Subdomains {
			if sub.TTL != 0 && (sub.TTL < 60 || sub.TTL > 86400) {
				return fmt.Errorf("zone[%d].subdomain[%d]: TTL must be between 60 and 86400 or 0 for default", i, j)
			}
		}
	}

	if cfg.ConcurrencyLimit < 0 {
		return fmt.Errorf("concurrency_limit must be positive")
	}

	return nil
}

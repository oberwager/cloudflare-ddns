package config

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: Config{
				DefaultTTL:       300,
				ConcurrencyLimit: 10,
				Zones: []Zone{
					{
						ZoneID: "zone123",
						TTL:    600,
						Subdomains: []Subdomain{
							{Name: "www", Proxied: true},
							{Name: "@", Proxied: false, TTL: 120},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with multiple zones",
			config: Config{
				Zones: []Zone{
					{
						ZoneID:     "zone1",
						Subdomains: []Subdomain{{Name: "www"}},
					},
					{
						ZoneID:     "zone2",
						Subdomains: []Subdomain{{Name: "api"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name:    "no zones",
			config:  Config{},
			wantErr: true,
			errMsg:  "no zones configured",
		},
		{
			name: "missing zone_id",
			config: Config{
				Zones: []Zone{
					{
						ZoneID:     "",
						Subdomains: []Subdomain{{Name: "www"}},
					},
				},
			},
			wantErr: true,
			errMsg:  "missing zone_id",
		},
		{
			name: "no subdomains",
			config: Config{
				Zones: []Zone{
					{
						ZoneID:     "zone123",
						Subdomains: []Subdomain{},
					},
				},
			},
			wantErr: true,
			errMsg:  "no subdomains configured",
		},
		{
			name: "ttl too low",
			config: Config{
				Zones: []Zone{
					{
						ZoneID: "zone123",
						Subdomains: []Subdomain{
							{Name: "www", TTL: 59},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "TTL must be between 60 and 86400",
		},
		{
			name: "ttl too high",
			config: Config{
				Zones: []Zone{
					{
						ZoneID: "zone123",
						Subdomains: []Subdomain{
							{Name: "www", TTL: 86401},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "TTL must be between 60 and 86400",
		},
		{
			name: "ttl zero is valid",
			config: Config{
				Zones: []Zone{
					{
						ZoneID: "zone123",
						Subdomains: []Subdomain{
							{Name: "www", TTL: 0},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ttl at minimum",
			config: Config{
				Zones: []Zone{
					{
						ZoneID: "zone123",
						Subdomains: []Subdomain{
							{Name: "www", TTL: 60},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "ttl at maximum",
			config: Config{
				Zones: []Zone{
					{
						ZoneID: "zone123",
						Subdomains: []Subdomain{
							{Name: "www", TTL: 86400},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "negative concurrency limit",
			config: Config{
				ConcurrencyLimit: -1,
				Zones: []Zone{
					{
						ZoneID:     "zone123",
						Subdomains: []Subdomain{{Name: "www"}},
					},
				},
			},
			wantErr: true,
			errMsg:  "concurrency_limit must be positive",
		},
		{
			name: "zero concurrency limit is valid",
			config: Config{
				ConcurrencyLimit: 0,
				Zones: []Zone{
					{
						ZoneID:     "zone123",
						Subdomains: []Subdomain{{Name: "www"}},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "multiple subdomains with mixed ttl",
			config: Config{
				Zones: []Zone{
					{
						ZoneID: "zone123",
						Subdomains: []Subdomain{
							{Name: "www", TTL: 300},
							{Name: "api", TTL: 0},
							{Name: "cdn", TTL: 600},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(&tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errMsg != "" {
				if err == nil || !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("Validate() error = %v, want error containing %q", err, tt.errMsg)
				}
			}
		})
	}
}

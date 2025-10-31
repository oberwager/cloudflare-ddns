package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
)

type Record struct {
	ID      string `json:"id,omitempty"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	Proxied bool   `json:"proxied"`
	TTL     int    `json:"ttl"`
}

type ZoneResponse struct {
	Result struct {
		Name string `json:"name"`
	} `json:"result"`
	Success bool     `json:"success"`
	Errors  []string `json:"errors"`
}

type ListRecordsResponse struct {
	Result  []Record `json:"result"`
	Success bool     `json:"success"`
	Errors  []string `json:"errors"`
}

func processZone(ctx context.Context, token string, zone Zone, ipv4, ipv6 string, defaultTTL, concurrencyLimit int) error {
	zoneData, err := cfAPI(ctx, "GET", fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s", zone.ZoneID), token, nil)
	if err != nil {
		return fmt.Errorf("get zone: %w", err)
	}

	var zoneResp ZoneResponse
	if err := json.Unmarshal(zoneData, &zoneResp); err != nil {
		return fmt.Errorf("unmarshal zone response: %w", err)
	}
	if !zoneResp.Success {
		return fmt.Errorf("zone API error: %v", zoneResp.Errors)
	}

	baseDomain := zoneResp.Result.Name
	slog.Debug("processing zone", "zone_id", zone.ZoneID, "domain", baseDomain)

	zoneTTL := zone.TTL
	if zoneTTL == 0 {
		zoneTTL = defaultTTL
	}

	sem := make(chan struct{}, concurrencyLimit)
	var wg sync.WaitGroup

	for _, sub := range zone.Subdomains {
		wg.Add(1)
		go func(s Subdomain) {
			defer wg.Done()

			sem <- struct{}{}
			defer func() { <-sem }()

			name := strings.ToLower(strings.TrimSpace(s.Name))
			fqdn := baseDomain
			if name != "" && name != "@" {
				fqdn = name + "." + baseDomain
			}

			ttl := s.TTL
			if ttl == 0 {
				ttl = zoneTTL
			}

			if err := upsertRecord(ctx, token, zone.ZoneID, fqdn, "A", ipv4, s.Proxied, ttl); err != nil {
				slog.Error("failed to upsert A record", "fqdn", fqdn, "error", err)
			}

			if ipv6 != "" {
				if err := upsertRecord(ctx, token, zone.ZoneID, fqdn, "AAAA", ipv6, s.Proxied, ttl); err != nil {
					slog.Error("failed to upsert AAAA record", "fqdn", fqdn, "error", err)
				}
			}
		}(sub)
	}

	wg.Wait()
	return nil
}

func upsertRecord(ctx context.Context, token, zoneID, fqdn, recordType, ip string, proxied bool, ttl int) error {
	listURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records?type=%s&name=%s", zoneID, recordType, fqdn)
	listData, err := cfAPI(ctx, "GET", listURL, token, nil)
	if err != nil {
		return fmt.Errorf("list records: %w", err)
	}

	var listResp ListRecordsResponse
	if err := json.Unmarshal(listData, &listResp); err != nil {
		return fmt.Errorf("unmarshal list response: %w", err)
	}
	if !listResp.Success {
		return fmt.Errorf("list API error: %v", listResp.Errors)
	}

	record := Record{
		Type:    recordType,
		Name:    fqdn,
		Content: ip,
		Proxied: proxied,
		TTL:     ttl,
	}

	if len(listResp.Result) == 0 {
		createURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", zoneID)
		if _, err := cfAPI(ctx, "POST", createURL, token, record); err != nil {
			return fmt.Errorf("create record: %w", err)
		}
		slog.Info("created record", "fqdn", fqdn, "type", recordType, "ip", ip, "proxied", proxied, "ttl", ttl)
		return nil
	}

	if len(listResp.Result) > 1 {
		slog.Warn("multiple records found, updating the first one", "fqdn", fqdn, "type", recordType, "count", len(listResp.Result))
	}

	existing := listResp.Result[0]
	ttlMatches := existing.TTL == ttl || (proxied && existing.TTL == 1)
	if existing.Content == ip && existing.Proxied == proxied && ttlMatches {
		slog.Debug("record already up to date", "fqdn", fqdn, "type", recordType, "ip", ip)
		return nil
	}

	updateURL := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", zoneID, existing.ID)
	if _, err := cfAPI(ctx, "PUT", updateURL, token, record); err != nil {
		return fmt.Errorf("update record: %w", err)
	}
	slog.Info("updated record", "fqdn", fqdn, "type", recordType, "ip", ip, "proxied", proxied, "ttl", ttl,
		"old_ip", existing.Content, "old_proxied", existing.Proxied, "old_ttl", existing.TTL)
	return nil
}

func cfAPI(ctx context.Context, method, url, token string, body any) ([]byte, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("execute request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, respBody)
	}

	return respBody, nil
}

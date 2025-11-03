package cloudflare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/oberwager/cloudflare-ddns/internal/config"
	"github.com/oberwager/cloudflare-ddns/internal/retry"
)

type mockTransport struct {
	server *httptest.Server
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	serverURL, _ := url.Parse(m.server.URL)
	req.URL.Scheme = serverURL.Scheme
	req.URL.Host = serverURL.Host
	return m.server.Client().Transport.RoundTrip(req)
}

func TestCfAPI(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		body       interface{}
		response   string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful GET",
			method:     "GET",
			body:       nil,
			response:   `{"success": true}`,
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "successful POST with body",
			method:     "POST",
			body:       map[string]string{"test": "value"},
			response:   `{"success": true}`,
			statusCode: http.StatusOK,
			wantErr:    false,
		},
		{
			name:       "error status",
			method:     "GET",
			body:       nil,
			response:   `{"error": "bad request"}`,
			statusCode: http.StatusBadRequest,
			wantErr:    true,
		},
		{
			name:       "server error",
			method:     "GET",
			body:       nil,
			response:   `{"error": "internal error"}`,
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != tt.method {
					t.Errorf("expected method %s, got %s", tt.method, r.Method)
				}

				auth := r.Header.Get("Authorization")
				if !strings.HasPrefix(auth, "Bearer ") {
					t.Error("missing or invalid Authorization header")
				}

				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			ctx := context.Background()
			_, err := cfAPI(ctx, tt.method, server.URL, "test-token", tt.body)

			if (err != nil) != tt.wantErr {
				t.Errorf("cfAPI() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUpsertRecordCreate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/dns_records") && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "result": []}`))
			return
		}

		if strings.Contains(r.URL.Path, "/dns_records") && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalClient := retry.HTTPClient
	retry.HTTPClient = &http.Client{
		Transport: &mockTransport{server: server},
		Timeout:   originalClient.Timeout,
	}
	defer func() { retry.HTTPClient = originalClient }()

	ctx := context.Background()
	zoneID := "zone123"
	err := upsertRecord(ctx, "token", zoneID, "test.example.com", "A", "1.2.3.4", true, 300)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUpsertRecordUpdate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/dns_records") && r.Method == "GET" {
			resp := ListRecordsResponse{
				Success: true,
				Result: []Record{
					{
						ID:      "rec123",
						Type:    "A",
						Name:    "test.example.com",
						Content: "5.6.7.8",
						Proxied: true,
						TTL:     300,
					},
				},
			}
			data, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		if strings.Contains(r.URL.Path, "/dns_records/rec123") && r.Method == "PUT" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalClient := retry.HTTPClient
	retry.HTTPClient = &http.Client{
		Transport: &mockTransport{server: server},
		Timeout:   originalClient.Timeout,
	}
	defer func() { retry.HTTPClient = originalClient }()

	ctx := context.Background()
	zoneID := "zone123"
	err := upsertRecord(ctx, "token", zoneID, "test.example.com", "A", "1.2.3.4", true, 300)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUpsertRecordNoChange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/dns_records") && r.Method == "GET" {
			resp := ListRecordsResponse{
				Success: true,
				Result: []Record{
					{
						ID:      "rec123",
						Type:    "A",
						Name:    "test.example.com",
						Content: "1.2.3.4",
						Proxied: true,
						TTL:     300,
					},
				},
			}
			data, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		t.Error("unexpected PUT request - record should be up to date")
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	originalClient := retry.HTTPClient
	retry.HTTPClient = &http.Client{
		Transport: &mockTransport{server: server},
		Timeout:   originalClient.Timeout,
	}
	defer func() { retry.HTTPClient = originalClient }()

	ctx := context.Background()
	zoneID := "zone123"
	err := upsertRecord(ctx, "token", zoneID, "test.example.com", "A", "1.2.3.4", true, 300)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUpsertRecordProxiedTTL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/dns_records") && r.Method == "GET" {
			resp := ListRecordsResponse{
				Success: true,
				Result: []Record{
					{
						ID:      "rec123",
						Type:    "A",
						Name:    "test.example.com",
						Content: "1.2.3.4",
						Proxied: true,
						TTL:     1,
					},
				},
			}
			data, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		t.Error("unexpected PUT request - proxied record with TTL=1 should match")
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	originalClient := retry.HTTPClient
	retry.HTTPClient = &http.Client{
		Transport: &mockTransport{server: server},
		Timeout:   originalClient.Timeout,
	}
	defer func() { retry.HTTPClient = originalClient }()

	ctx := context.Background()
	zoneID := "zone123"
	err := upsertRecord(ctx, "token", zoneID, "test.example.com", "A", "1.2.3.4", true, 300)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestUpsertRecordMultipleFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/dns_records") && r.Method == "GET" {
			resp := ListRecordsResponse{
				Success: true,
				Result: []Record{
					{
						ID:      "rec1",
						Type:    "A",
						Name:    "test.example.com",
						Content: "5.6.7.8",
						Proxied: true,
						TTL:     300,
					},
					{
						ID:      "rec2",
						Type:    "A",
						Name:    "test.example.com",
						Content: "5.6.7.8",
						Proxied: true,
						TTL:     300,
					},
				},
			}
			data, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		if strings.Contains(r.URL.Path, "/dns_records/rec1") && r.Method == "PUT" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalClient := retry.HTTPClient
	retry.HTTPClient = &http.Client{
		Transport: &mockTransport{server: server},
		Timeout:   originalClient.Timeout,
	}
	defer func() { retry.HTTPClient = originalClient }()

	ctx := context.Background()
	zoneID := "zone123"
	err := upsertRecord(ctx, "token", zoneID, "test.example.com", "A", "1.2.3.4", true, 300)

	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestProcessZone(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++

		if strings.HasSuffix(r.URL.Path, "/zones/zone123") && r.Method == "GET" {
			resp := ZoneResponse{
				Success: true,
			}
			resp.Result.Name = "example.com"
			data, _ := json.Marshal(resp)
			w.WriteHeader(http.StatusOK)
			w.Write(data)
			return
		}

		if strings.Contains(r.URL.Path, "/dns_records") && r.Method == "GET" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true, "result": []}`))
			return
		}

		if strings.Contains(r.URL.Path, "/dns_records") && r.Method == "POST" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"success": true}`))
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	originalClient := retry.HTTPClient
	retry.HTTPClient = &http.Client{
		Transport: &mockTransport{server: server},
		Timeout:   originalClient.Timeout,
	}
	defer func() { retry.HTTPClient = originalClient }()

	ctx := context.Background()
	zone := config.Zone{
		ZoneID: "zone123",
		Subdomains: []config.Subdomain{
			{Name: "www", Proxied: true},
			{Name: "api", Proxied: false},
		},
	}

	err := ProcessZone(ctx, "token", zone, "1.2.3.4", "", 300, 10)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if callCount < 3 {
		t.Errorf("expected at least 3 API calls (1 zone + 2 subdomains), got %d", callCount)
	}
}

package ip

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidateIP(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		isIPv6  bool
		wantErr bool
	}{
		{"valid ipv4", "192.168.1.1", false, false},
		{"valid ipv4 public", "8.8.8.8", false, false},
		{"valid ipv6", "2001:0db8:85a3::8a2e:0370:7334", true, false},
		{"valid ipv6 short", "::1", true, false},
		{"invalid ip", "not-an-ip", false, true},
		{"invalid ip", "999.999.999.999", false, true},
		{"ipv4 when expecting ipv6", "192.168.1.1", true, true},
		{"ipv6 when expecting ipv4", "2001:0db8::1", false, true},
		{"empty string", "", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIP(tt.ip, tt.isIPv6)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateIP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetIP(t *testing.T) {
	tests := []struct {
		name       string
		response   string
		statusCode int
		wantIP     string
		wantErr    bool
	}{
		{
			name:       "successful response",
			response:   "192.168.1.1",
			statusCode: http.StatusOK,
			wantIP:     "192.168.1.1",
			wantErr:    false,
		},
		{
			name:       "response with whitespace",
			response:   "  192.168.1.1\n",
			statusCode: http.StatusOK,
			wantIP:     "192.168.1.1",
			wantErr:    false,
		},
		{
			name:       "ipv6 response",
			response:   "2001:0db8::1",
			statusCode: http.StatusOK,
			wantIP:     "2001:0db8::1",
			wantErr:    false,
		},
		{
			name:       "error status",
			response:   "error",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:       "not found",
			response:   "not found",
			statusCode: http.StatusNotFound,
			wantErr:    true,
		},
		{
			name:       "empty response",
			response:   "",
			statusCode: http.StatusOK,
			wantErr:    true,
		},
		{
			name:       "whitespace only response",
			response:   "   \n\t  ",
			statusCode: http.StatusOK,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.response))
			}))
			defer server.Close()

			ctx := context.Background()
			ip, err := getIP(ctx, server.URL)

			if (err != nil) != tt.wantErr {
				t.Errorf("getIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && ip != tt.wantIP {
				t.Errorf("getIP() = %v, want %v", ip, tt.wantIP)
			}
		})
	}
}

func TestGetIPWithRetry(t *testing.T) {
	t.Run("successful on first attempt", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("192.168.1.1"))
		}))
		defer server.Close()

		ctx := context.Background()
		ip, err := GetWithRetry(ctx, server.URL, false)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ip != "192.168.1.1" {
			t.Errorf("expected 192.168.1.1, got %s", ip)
		}
	})

	t.Run("successful ipv6", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("2001:0db8::1"))
		}))
		defer server.Close()

		ctx := context.Background()
		ip, err := GetWithRetry(ctx, server.URL, true)

		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ip != "2001:0db8::1" {
			t.Errorf("expected 2001:0db8::1, got %s", ip)
		}
	})

	t.Run("validation failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("192.168.1.1"))
		}))
		defer server.Close()

		ctx := context.Background()
		_, err := GetWithRetry(ctx, server.URL, true)

		if err == nil {
			t.Fatal("expected error for ipv4 when expecting ipv6, got nil")
		}
	})

	t.Run("invalid ip format", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("not-an-ip"))
		}))
		defer server.Close()

		ctx := context.Background()
		_, err := GetWithRetry(ctx, server.URL, false)

		if err == nil {
			t.Fatal("expected error for invalid ip, got nil")
		}
	})
}

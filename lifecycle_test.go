package caddyutmtracker

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestRequest(url string) *http.Request {
	return httptest.NewRequest(http.MethodGet, url, nil)
}

func TestCaddyModule(t *testing.T) {
	tracker := UTMTracker{}
	info := tracker.CaddyModule()

	if info.ID != "http.handlers.utm_tracker" {
		t.Errorf("module ID = %q, want %q", info.ID, "http.handlers.utm_tracker")
	}

	mod := info.New()
	if _, ok := mod.(*UTMTracker); !ok {
		t.Error("New() did not return *UTMTracker")
	}
}

func TestValidate(t *testing.T) {
	bTrue := true

	tests := []struct {
		name    string
		tracker UTMTracker
		wantErr bool
	}{
		{
			name: "valid config",
			tracker: UTMTracker{
				ClientID:       "test",
				BatchSize:      50,
				flushDuration:  5 * time.Second,
				CookieSameSite: "lax",
				CookieSecure:   &bTrue,
				CookieHTTPOnly: &bTrue,
			},
			wantErr: false,
		},
		{
			name: "missing client_id",
			tracker: UTMTracker{
				BatchSize:      50,
				flushDuration:  5 * time.Second,
				CookieSameSite: "lax",
				CookieSecure:   &bTrue,
				CookieHTTPOnly: &bTrue,
			},
			wantErr: true,
		},
		{
			name: "invalid batch_size",
			tracker: UTMTracker{
				ClientID:       "test",
				BatchSize:      0,
				flushDuration:  5 * time.Second,
				CookieSameSite: "lax",
				CookieSecure:   &bTrue,
				CookieHTTPOnly: &bTrue,
			},
			wantErr: true,
		},
		{
			name: "invalid flush_interval",
			tracker: UTMTracker{
				ClientID:       "test",
				BatchSize:      50,
				flushDuration:  500 * time.Millisecond,
				CookieSameSite: "lax",
				CookieSecure:   &bTrue,
				CookieHTTPOnly: &bTrue,
			},
			wantErr: true,
		},
		{
			name: "invalid cookie_same_site",
			tracker: UTMTracker{
				ClientID:       "test",
				BatchSize:      50,
				flushDuration:  5 * time.Second,
				CookieSameSite: "invalid",
				CookieSecure:   &bTrue,
				CookieHTTPOnly: &bTrue,
			},
			wantErr: true,
		},
		{
			name: "strict same_site",
			tracker: UTMTracker{
				ClientID:       "test",
				BatchSize:      50,
				flushDuration:  5 * time.Second,
				CookieSameSite: "strict",
				CookieSecure:   &bTrue,
				CookieHTTPOnly: &bTrue,
			},
			wantErr: false,
		},
		{
			name: "none same_site",
			tracker: UTMTracker{
				ClientID:       "test",
				BatchSize:      50,
				flushDuration:  5 * time.Second,
				CookieSameSite: "none",
				CookieSecure:   &bTrue,
				CookieHTTPOnly: &bTrue,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tracker.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCleanup(t *testing.T) {
	bTrue := true
	tracker := &UTMTracker{
		ClientID:       "test",
		CookieSecure:   &bTrue,
		CookieHTTPOnly: &bTrue,
		CookieSameSite: "lax",
	}

	// Simulate what Provision would set up
	tracker.httpClient = nil // will be nil-safe
	tracker.buffer = nil     // will be nil-safe

	// Cleanup with nil buffer and client should not panic
	err := tracker.Cleanup()
	if err != nil {
		t.Errorf("Cleanup() error = %v", err)
	}
}

func TestHasTrackingParams(t *testing.T) {
	bTrue := true
	tracker := &UTMTracker{
		ExtraParams:    []string{"via"},
		CookieSecure:   &bTrue,
		CookieHTTPOnly: &bTrue,
	}

	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"has utm_source", "/page?utm_source=google", true},
		{"has gclid", "/page?gclid=abc", true},
		{"has extra param", "/page?via=email", true},
		{"no tracking params", "/page?foo=bar", false},
		{"no params", "/page", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newTestRequest(tt.url)
			got := tracker.hasTrackingParams(r)
			if got != tt.want {
				t.Errorf("hasTrackingParams() = %v, want %v", got, tt.want)
			}
		})
	}
}

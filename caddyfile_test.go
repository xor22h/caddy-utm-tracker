package caddyutmtracker

import (
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
		err   bool
	}{
		{"5s", 5 * time.Second, false},
		{"30m", 30 * time.Minute, false},
		{"1h", 1 * time.Hour, false},
		{"365d", 365 * 24 * time.Hour, false},
		{"7d", 7 * 24 * time.Hour, false},
		{"invalid", 0, true},
		{"xd", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseDuration(tt.input)
			if (err != nil) != tt.err {
				t.Errorf("parseDuration(%q) error = %v, wantErr %v", tt.input, err, tt.err)
				return
			}
			if got != tt.want {
				t.Errorf("parseDuration(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestUnmarshalCaddyfile(t *testing.T) {
	input := `utm_tracker {
		client_id          "op_client_abc123"
		client_secret      "op_secret_xyz789"
		endpoint           "https://my-openpanel.example.com"
		cookie_name        "_visitor"
		cookie_domain      ".example.com"
		cookie_max_age     7d
		cookie_secure      false
		cookie_http_only   false
		cookie_same_site   strict
		session_cookie     "_session"
		session_max_age    1h
		strip_params       true
		only_with_params   true
		track_methods      GET POST
		ignore_paths       /api/* /health
		track_paths        /landing/*
		batch_size         100
		flush_interval     10s
		extra_params       via partner_id
		bot_patterns       MyBot CustomCrawler
	}`

	d := caddyfile.NewTestDispenser(input)
	var tracker UTMTracker
	err := tracker.UnmarshalCaddyfile(d)
	if err != nil {
		t.Fatalf("UnmarshalCaddyfile error: %v", err)
	}

	if tracker.ClientID != "op_client_abc123" {
		t.Errorf("ClientID = %q, want %q", tracker.ClientID, "op_client_abc123")
	}
	if tracker.ClientSecret != "op_secret_xyz789" {
		t.Errorf("ClientSecret = %q, want %q", tracker.ClientSecret, "op_secret_xyz789")
	}
	if tracker.Endpoint != "https://my-openpanel.example.com" {
		t.Errorf("Endpoint = %q, want %q", tracker.Endpoint, "https://my-openpanel.example.com")
	}
	if tracker.CookieName != "_visitor" {
		t.Errorf("CookieName = %q, want %q", tracker.CookieName, "_visitor")
	}
	if tracker.CookieDomain != ".example.com" {
		t.Errorf("CookieDomain = %q, want %q", tracker.CookieDomain, ".example.com")
	}
	if tracker.CookieMaxAge != 7*24*3600 {
		t.Errorf("CookieMaxAge = %d, want %d", tracker.CookieMaxAge, 7*24*3600)
	}
	if tracker.CookieSecure == nil || *tracker.CookieSecure != false {
		t.Errorf("CookieSecure = %v, want false", tracker.CookieSecure)
	}
	if tracker.CookieHTTPOnly == nil || *tracker.CookieHTTPOnly != false {
		t.Errorf("CookieHTTPOnly = %v, want false", tracker.CookieHTTPOnly)
	}
	if tracker.CookieSameSite != "strict" {
		t.Errorf("CookieSameSite = %q, want %q", tracker.CookieSameSite, "strict")
	}
	if tracker.SessionCookie != "_session" {
		t.Errorf("SessionCookie = %q, want %q", tracker.SessionCookie, "_session")
	}
	if tracker.SessionMaxAge != 3600 {
		t.Errorf("SessionMaxAge = %d, want %d", tracker.SessionMaxAge, 3600)
	}
	if !tracker.StripParams {
		t.Error("StripParams = false, want true")
	}
	if !tracker.OnlyWithParams {
		t.Error("OnlyWithParams = false, want true")
	}
	if len(tracker.TrackMethods) != 2 || tracker.TrackMethods[0] != "GET" || tracker.TrackMethods[1] != "POST" {
		t.Errorf("TrackMethods = %v, want [GET POST]", tracker.TrackMethods)
	}
	if len(tracker.IgnorePaths) != 2 {
		t.Errorf("IgnorePaths = %v, want 2 items", tracker.IgnorePaths)
	}
	if len(tracker.TrackPaths) != 1 || tracker.TrackPaths[0] != "/landing/*" {
		t.Errorf("TrackPaths = %v, want [/landing/*]", tracker.TrackPaths)
	}
	if tracker.BatchSize != 100 {
		t.Errorf("BatchSize = %d, want 100", tracker.BatchSize)
	}
	if tracker.FlushInterval != "10s" {
		t.Errorf("FlushInterval = %q, want %q", tracker.FlushInterval, "10s")
	}
	if len(tracker.ExtraParams) != 2 {
		t.Errorf("ExtraParams = %v, want 2 items", tracker.ExtraParams)
	}
	if len(tracker.BotPatterns) != 2 {
		t.Errorf("BotPatterns = %v, want 2 items", tracker.BotPatterns)
	}
}

func TestUnmarshalCaddyfile_MinimalConfig(t *testing.T) {
	input := `utm_tracker {
		client_id "my-client"
	}`

	d := caddyfile.NewTestDispenser(input)
	var tracker UTMTracker
	err := tracker.UnmarshalCaddyfile(d)
	if err != nil {
		t.Fatalf("UnmarshalCaddyfile error: %v", err)
	}

	if tracker.ClientID != "my-client" {
		t.Errorf("ClientID = %q, want %q", tracker.ClientID, "my-client")
	}
}

func TestUnmarshalCaddyfile_UnknownDirective(t *testing.T) {
	input := `utm_tracker {
		client_id "test"
		unknown_thing value
	}`

	d := caddyfile.NewTestDispenser(input)
	var tracker UTMTracker
	err := tracker.UnmarshalCaddyfile(d)
	if err == nil {
		t.Fatal("expected error for unknown directive")
	}
}

func TestUnmarshalCaddyfile_MissingArgs(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing client_id value", "utm_tracker {\n\tclient_id\n}"},
		{"missing endpoint value", "utm_tracker {\n\tendpoint\n}"},
		{"missing batch_size value", "utm_tracker {\n\tbatch_size\n}"},
		{"missing track_methods values", "utm_tracker {\n\ttrack_methods\n}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := caddyfile.NewTestDispenser(tt.input)
			var tracker UTMTracker
			err := tracker.UnmarshalCaddyfile(d)
			if err == nil {
				t.Fatal("expected error for missing args")
			}
		})
	}
}

func TestUnmarshalCaddyfile_InvalidValues(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"invalid cookie_max_age", "utm_tracker {\n\tcookie_max_age notaduration\n}"},
		{"invalid cookie_secure", "utm_tracker {\n\tcookie_secure notabool\n}"},
		{"invalid cookie_http_only", "utm_tracker {\n\tcookie_http_only notabool\n}"},
		{"invalid strip_params", "utm_tracker {\n\tstrip_params notabool\n}"},
		{"invalid only_with_params", "utm_tracker {\n\tonly_with_params notabool\n}"},
		{"invalid batch_size", "utm_tracker {\n\tbatch_size notanint\n}"},
		{"invalid session_max_age", "utm_tracker {\n\tsession_max_age notaduration\n}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := caddyfile.NewTestDispenser(tt.input)
			var tracker UTMTracker
			err := tracker.UnmarshalCaddyfile(d)
			if err == nil {
				t.Fatal("expected error for invalid value")
			}
		})
	}
}

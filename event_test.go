package caddyutmtracker

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestExtractParams(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		extraParams []string
		wantKeys    map[string]string
		wantMissing []string
	}{
		{
			name:  "all utm params",
			query: "utm_source=google&utm_medium=cpc&utm_campaign=spring&utm_term=shoes&utm_content=banner",
			wantKeys: map[string]string{
				"utm_source":   "google",
				"utm_medium":   "cpc",
				"utm_campaign": "spring",
				"utm_term":     "shoes",
				"utm_content":  "banner",
			},
		},
		{
			name:  "click IDs",
			query: "gclid=abc123&fbclid=fb456&msclkid=ms789",
			wantKeys: map[string]string{
				"gclid":   "abc123",
				"fbclid":  "fb456",
				"msclkid": "ms789",
			},
		},
		{
			name:  "ref param",
			query: "ref=partner123",
			wantKeys: map[string]string{
				"referral_code": "partner123",
			},
		},
		{
			name:        "no params",
			query:       "",
			wantMissing: []string{"utm_source", "utm_medium", "gclid"},
		},
		{
			name:        "extra params",
			query:       "via=newsletter&partner_id=p100",
			extraParams: []string{"via", "partner_id"},
			wantKeys: map[string]string{
				"via":        "newsletter",
				"partner_id": "p100",
			},
		},
		{
			name:        "mixed standard and extra",
			query:       "utm_source=twitter&via=sidebar",
			extraParams: []string{"via"},
			wantKeys: map[string]string{
				"utm_source": "twitter",
				"via":        "sidebar",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values, _ := url.ParseQuery(tt.query)
			result := extractParams(values, tt.extraParams)

			for key, want := range tt.wantKeys {
				got, ok := result[key]
				if !ok {
					t.Errorf("expected key %q to be present", key)
					continue
				}
				if got != want {
					t.Errorf("key %q = %q, want %q", key, got, want)
				}
			}

			for _, key := range tt.wantMissing {
				if _, ok := result[key]; ok {
					t.Errorf("expected key %q to be absent", key)
				}
			}
		})
	}
}

func TestStripTrackingParams(t *testing.T) {
	tests := []struct {
		name        string
		rawURL      string
		extraParams []string
		wantQuery   string
		wantChanged bool
	}{
		{
			name:        "strip utm params",
			rawURL:      "https://example.com/page?utm_source=google&utm_medium=cpc&other=keep",
			wantQuery:   "other=keep",
			wantChanged: true,
		},
		{
			name:        "strip click IDs",
			rawURL:      "https://example.com/?gclid=abc&fbclid=def&keep=yes",
			wantQuery:   "keep=yes",
			wantChanged: true,
		},
		{
			name:        "strip extra params",
			rawURL:      "https://example.com/?via=email&keep=yes",
			extraParams: []string{"via"},
			wantQuery:   "keep=yes",
			wantChanged: true,
		},
		{
			name:        "nothing to strip",
			rawURL:      "https://example.com/?foo=bar",
			wantQuery:   "foo=bar",
			wantChanged: false,
		},
		{
			name:        "strip all params",
			rawURL:      "https://example.com/?utm_source=twitter",
			wantQuery:   "",
			wantChanged: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u, err := url.Parse(tt.rawURL)
			if err != nil {
				t.Fatal(err)
			}

			changed := stripTrackingParams(u, tt.extraParams)

			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if u.RawQuery != tt.wantQuery {
				t.Errorf("query = %q, want %q", u.RawQuery, tt.wantQuery)
			}
		})
	}
}

func TestClientIP(t *testing.T) {
	tests := []struct {
		name       string
		headers    map[string]string
		remoteAddr string
		want       string
	}{
		{
			name:       "X-Forwarded-For single",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4"},
			remoteAddr: "10.0.0.1:1234",
			want:       "1.2.3.4",
		},
		{
			name:       "X-Forwarded-For chain",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4, 5.6.7.8"},
			remoteAddr: "10.0.0.1:1234",
			want:       "1.2.3.4",
		},
		{
			name:       "X-Real-IP",
			headers:    map[string]string{"X-Real-IP": "9.8.7.6"},
			remoteAddr: "10.0.0.1:1234",
			want:       "9.8.7.6",
		},
		{
			name:       "X-Forwarded-For takes precedence",
			headers:    map[string]string{"X-Forwarded-For": "1.2.3.4", "X-Real-IP": "9.8.7.6"},
			remoteAddr: "10.0.0.1:1234",
			want:       "1.2.3.4",
		},
		{
			name:       "fallback to RemoteAddr",
			headers:    map[string]string{},
			remoteAddr: "10.0.0.1:1234",
			want:       "10.0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}

			got := clientIP(r)
			if got != tt.want {
				t.Errorf("clientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildEvent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/pricing?utm_source=twitter&utm_campaign=launch", nil)
	r.Header.Set("Referer", "https://google.com")
	r.Header.Set("User-Agent", "Mozilla/5.0")
	r.RemoteAddr = "1.2.3.4:5678"

	event := buildEvent(r, "visitor-123", "session-456", nil)

	if event.VisitorID != "visitor-123" {
		t.Errorf("VisitorID = %q, want %q", event.VisitorID, "visitor-123")
	}
	if event.SessionID != "session-456" {
		t.Errorf("SessionID = %q, want %q", event.SessionID, "session-456")
	}
	if event.Path != "/pricing" {
		t.Errorf("Path = %q, want %q", event.Path, "/pricing")
	}
	if event.Referrer != "https://google.com" {
		t.Errorf("Referrer = %q, want %q", event.Referrer, "https://google.com")
	}
	if event.UserAgent != "Mozilla/5.0" {
		t.Errorf("UserAgent = %q, want %q", event.UserAgent, "Mozilla/5.0")
	}
	if event.ClientIP != "1.2.3.4" {
		t.Errorf("ClientIP = %q, want %q", event.ClientIP, "1.2.3.4")
	}
	if event.Properties["utm_source"] != "twitter" {
		t.Errorf("utm_source = %q, want %q", event.Properties["utm_source"], "twitter")
	}
	if event.Properties["utm_campaign"] != "launch" {
		t.Errorf("utm_campaign = %q, want %q", event.Properties["utm_campaign"], "launch")
	}
	if event.Properties["path"] != "/pricing" {
		t.Errorf("properties path = %q, want %q", event.Properties["path"], "/pricing")
	}
	if event.Properties["referrer"] != "https://google.com" {
		t.Errorf("properties referrer = %q, want %q", event.Properties["referrer"], "https://google.com")
	}
}

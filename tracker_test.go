package caddyutmtracker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"go.uber.org/zap"
)

// mockHandler is a simple handler that records whether it was called.
type mockHandler struct {
	called    bool
	receivedR *http.Request
}

func (m *mockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	m.called = true
	m.receivedR = r
	w.WriteHeader(http.StatusOK)
	return nil
}

func newTestTracker(t *testing.T, server *httptest.Server) *UTMTracker {
	t.Helper()

	bTrue := true
	tracker := &UTMTracker{
		ClientID:       "test-client",
		ClientSecret:   "test-secret",
		Endpoint:       server.URL,
		CookieName:     "_vid",
		CookieDomain:   "",
		CookieMaxAge:   86400,
		CookieSecure:   &bTrue,
		CookieHTTPOnly: &bTrue,
		CookieSameSite: "lax",
		SessionCookie:  "_sid",
		SessionMaxAge:  1800,
		TrackMethods:   []string{"GET"},
		TrackPaths:     []string{"/*"},
		IgnorePaths:    []string{"/api/*", "/static/*", "/favicon.ico"},
		BatchSize:      50,
		FlushInterval:  "100ms",
		logger:         zap.NewNop(),
	}

	dur, _ := time.ParseDuration(tracker.FlushInterval)
	tracker.flushDuration = dur

	tracker.trackMethodSet = map[string]bool{"GET": true}

	tracker.httpClient = server.Client()
	tracker.client = &OpenPanelClient{
		Endpoint:     server.URL,
		ClientID:     tracker.ClientID,
		ClientSecret: tracker.ClientSecret,
		HTTPClient:   tracker.httpClient,
		Logger:       tracker.logger,
	}
	tracker.buffer = NewEventBuffer(tracker.client, tracker.logger, tracker.BatchSize, tracker.flushDuration)

	return tracker
}

func TestServeHTTP_SetsVisitorAndSessionCookies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tracker := newTestTracker(t, server)
	defer tracker.buffer.Stop()

	next := &mockHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/pricing?utm_source=google", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	err := tracker.ServeHTTP(w, r, next)
	if err != nil {
		t.Fatalf("ServeHTTP error: %v", err)
	}

	if !next.called {
		t.Fatal("next handler was not called")
	}

	// Check cookies were set
	cookies := w.Result().Cookies()
	var vidCookie, sidCookie *http.Cookie
	for _, c := range cookies {
		switch c.Name {
		case "_vid":
			vidCookie = c
		case "_sid":
			sidCookie = c
		}
	}

	if vidCookie == nil {
		t.Fatal("_vid cookie not set")
	}
	if sidCookie == nil {
		t.Fatal("_sid cookie not set")
	}
	if vidCookie.Value == "" {
		t.Error("_vid cookie is empty")
	}
	if sidCookie.Value == "" {
		t.Error("_sid cookie is empty")
	}
	if vidCookie.MaxAge != 86400 {
		t.Errorf("_vid MaxAge = %d, want 86400", vidCookie.MaxAge)
	}
	if sidCookie.MaxAge != 1800 {
		t.Errorf("_sid MaxAge = %d, want 1800", sidCookie.MaxAge)
	}
}

func TestServeHTTP_SkipsBots(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tracker := newTestTracker(t, server)
	defer tracker.buffer.Stop()

	next := &mockHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/pricing?utm_source=google", nil)
	r.Header.Set("User-Agent", "Googlebot/2.1 (+http://www.google.com/bot.html)")

	err := tracker.ServeHTTP(w, r, next)
	if err != nil {
		t.Fatalf("ServeHTTP error: %v", err)
	}

	if !next.called {
		t.Fatal("next handler should still be called for bots")
	}

	// Bot requests should not set cookies
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "_vid" || c.Name == "_sid" {
			t.Errorf("unexpected cookie %q set for bot", c.Name)
		}
	}
}

func TestServeHTTP_SkipsNonGET(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tracker := newTestTracker(t, server)
	defer tracker.buffer.Stop()

	next := &mockHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/submit", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0")

	err := tracker.ServeHTTP(w, r, next)
	if err != nil {
		t.Fatalf("ServeHTTP error: %v", err)
	}

	if !next.called {
		t.Fatal("next handler should be called")
	}

	// POST should not set cookies (not tracked)
	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "_vid" || c.Name == "_sid" {
			t.Errorf("unexpected cookie %q set for POST", c.Name)
		}
	}
}

func TestServeHTTP_SkipsIgnoredPaths(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tracker := newTestTracker(t, server)
	defer tracker.buffer.Stop()

	next := &mockHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/users?utm_source=google", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0")

	err := tracker.ServeHTTP(w, r, next)
	if err != nil {
		t.Fatalf("ServeHTTP error: %v", err)
	}

	if !next.called {
		t.Fatal("next handler should be called")
	}

	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "_vid" || c.Name == "_sid" {
			t.Errorf("unexpected cookie %q set for ignored path", c.Name)
		}
	}
}

func TestServeHTTP_StripParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tracker := newTestTracker(t, server)
	tracker.StripParams = true
	defer tracker.buffer.Stop()

	next := &mockHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/pricing?utm_source=twitter&utm_campaign=launch&keep=yes", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	err := tracker.ServeHTTP(w, r, next)
	if err != nil {
		t.Fatalf("ServeHTTP error: %v", err)
	}

	// Verify upstream received clean URL
	if next.receivedR.URL.Query().Has("utm_source") {
		t.Error("utm_source should have been stripped")
	}
	if next.receivedR.URL.Query().Has("utm_campaign") {
		t.Error("utm_campaign should have been stripped")
	}
	if !next.receivedR.URL.Query().Has("keep") {
		t.Error("non-UTM param 'keep' should be preserved")
	}
}

func TestServeHTTP_OnlyWithParams(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tracker := newTestTracker(t, server)
	tracker.OnlyWithParams = true
	defer tracker.buffer.Stop()

	// Request without UTM params
	next := &mockHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/pricing", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	err := tracker.ServeHTTP(w, r, next)
	if err != nil {
		t.Fatalf("ServeHTTP error: %v", err)
	}

	cookies := w.Result().Cookies()
	for _, c := range cookies {
		if c.Name == "_vid" || c.Name == "_sid" {
			t.Errorf("unexpected cookie %q when only_with_params=true and no params", c.Name)
		}
	}

	// Request WITH UTM params
	next2 := &mockHandler{}
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodGet, "/pricing?utm_source=google", nil)
	r2.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	err = tracker.ServeHTTP(w2, r2, next2)
	if err != nil {
		t.Fatalf("ServeHTTP error: %v", err)
	}

	var vidFound bool
	for _, c := range w2.Result().Cookies() {
		if c.Name == "_vid" {
			vidFound = true
		}
	}
	if !vidFound {
		t.Error("_vid cookie should be set when UTM params present")
	}
}

func TestServeHTTP_ExistingCookieReused(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tracker := newTestTracker(t, server)
	defer tracker.buffer.Stop()

	next := &mockHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/pricing", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	r.AddCookie(&http.Cookie{Name: "_vid", Value: "existing-visitor-id"})
	r.AddCookie(&http.Cookie{Name: "_sid", Value: "existing-session-id"})

	err := tracker.ServeHTTP(w, r, next)
	if err != nil {
		t.Fatalf("ServeHTTP error: %v", err)
	}

	// Cookie should be refreshed with the same value
	for _, c := range w.Result().Cookies() {
		if c.Name == "_vid" && c.Value != "existing-visitor-id" {
			t.Errorf("_vid = %q, want %q", c.Value, "existing-visitor-id")
		}
		if c.Name == "_sid" && c.Value != "existing-session-id" {
			t.Errorf("_sid = %q, want %q", c.Value, "existing-session-id")
		}
	}
}

func TestServeHTTP_IntegrationEventReceived(t *testing.T) {
	var mu sync.Mutex
	var receivedEvents []openPanelRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req openPanelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		mu.Lock()
		receivedEvents = append(receivedEvents, req)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	tracker := newTestTracker(t, server)

	// Use small batch to flush quickly
	tracker.buffer.Stop()
	tracker.buffer = NewEventBuffer(tracker.client, tracker.logger, 1, 50*time.Millisecond)
	defer tracker.buffer.Stop()

	next := &mockHandler{}
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/pricing?utm_source=twitter&utm_campaign=launch", nil)
	r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	r.Header.Set("Referer", "https://google.com")
	r.RemoteAddr = "1.2.3.4:5678"

	err := tracker.ServeHTTP(w, r, next)
	if err != nil {
		t.Fatalf("ServeHTTP error: %v", err)
	}

	// Wait for batch flush
	time.Sleep(300 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	if len(receivedEvents) != 1 {
		t.Fatalf("expected 1 event, got %d", len(receivedEvents))
	}

	event := receivedEvents[0]
	if event.Type != "track" {
		t.Errorf("type = %q, want %q", event.Type, "track")
	}
	if event.Payload.Name != "page_view" {
		t.Errorf("name = %q, want %q", event.Payload.Name, "page_view")
	}
	if event.Payload.Properties["utm_source"] != "twitter" {
		t.Errorf("utm_source = %q, want %q", event.Payload.Properties["utm_source"], "twitter")
	}
	if event.Payload.Properties["utm_campaign"] != "launch" {
		t.Errorf("utm_campaign = %q, want %q", event.Payload.Properties["utm_campaign"], "launch")
	}
	if event.Payload.Properties["path"] != "/pricing" {
		t.Errorf("path = %q, want %q", event.Payload.Properties["path"], "/pricing")
	}
}

func TestShouldIgnorePath(t *testing.T) {
	bTrue := true
	tracker := &UTMTracker{
		IgnorePaths:    []string{"/api/*", "/static/*", "/favicon.ico"},
		CookieSecure:   &bTrue,
		CookieHTTPOnly: &bTrue,
	}

	tests := []struct {
		path string
		want bool
	}{
		{"/api/users", true},
		{"/api/v1/data", true},
		{"/static/style.css", true},
		{"/favicon.ico", true},
		{"/pricing", false},
		{"/about", false},
		{"/", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := tracker.shouldIgnorePath(tt.path)
			if got != tt.want {
				t.Errorf("shouldIgnorePath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestShouldTrackPath(t *testing.T) {
	bTrue := true
	tests := []struct {
		name       string
		trackPaths []string
		path       string
		want       bool
	}{
		{"wildcard matches all", []string{"/*"}, "/anything", true},
		{"specific match", []string{"/landing/*"}, "/landing/page1", true},
		{"no match", []string{"/landing/*"}, "/about", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracker := &UTMTracker{
				TrackPaths:     tt.trackPaths,
				CookieSecure:   &bTrue,
				CookieHTTPOnly: &bTrue,
			}
			got := tracker.shouldTrackPath(tt.path)
			if got != tt.want {
				t.Errorf("shouldTrackPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// Verify the interface is satisfied at compile time.
func TestInterfaceGuards(t *testing.T) {
	var _ caddyhttp.MiddlewareHandler = (*UTMTracker)(nil)
	_ = fmt.Sprintf("interfaces satisfied")
}

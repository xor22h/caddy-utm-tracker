package caddyutmtracker

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// UTMTracker is a Caddy HTTP middleware that captures UTM parameters
// and marketing attribution data from incoming requests and forwards
// them as events to OpenPanel.
type UTMTracker struct {
	// OpenPanel configuration
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret,omitempty"`
	Endpoint     string `json:"endpoint,omitempty"`

	// Visitor cookie settings
	CookieName     string `json:"cookie_name,omitempty"`
	CookieDomain   string `json:"cookie_domain,omitempty"`
	CookieMaxAge   int    `json:"cookie_max_age,omitempty"`   // seconds
	CookieSecure   *bool  `json:"cookie_secure,omitempty"`
	CookieHTTPOnly *bool  `json:"cookie_http_only,omitempty"`
	CookieSameSite string `json:"cookie_same_site,omitempty"`

	// Session cookie settings
	SessionCookie string `json:"session_cookie,omitempty"`
	SessionMaxAge int    `json:"session_max_age,omitempty"` // seconds

	// Behavior
	StripParams    bool     `json:"strip_params,omitempty"`
	OnlyWithParams bool     `json:"only_with_params,omitempty"`
	TrackMethods   []string `json:"track_methods,omitempty"`

	// Path filtering
	TrackPaths  []string `json:"track_paths,omitempty"`
	IgnorePaths []string `json:"ignore_paths,omitempty"`

	// Batching
	BatchSize     int    `json:"batch_size,omitempty"`
	FlushInterval string `json:"flush_interval,omitempty"`

	// Custom parameters
	ExtraParams []string `json:"extra_params,omitempty"`

	// Bot filtering
	BotPatterns []string `json:"bot_patterns,omitempty"`

	// Internal state
	logger        *zap.Logger
	buffer        *EventBuffer
	client        *OpenPanelClient
	httpClient    *http.Client
	flushDuration time.Duration
	trackMethodSet map[string]bool
}

// Interface guards
var (
	_ caddy.Module                = (*UTMTracker)(nil)
	_ caddy.Provisioner           = (*UTMTracker)(nil)
	_ caddy.Validator             = (*UTMTracker)(nil)
	_ caddy.CleanerUpper          = (*UTMTracker)(nil)
	_ caddyhttp.MiddlewareHandler = (*UTMTracker)(nil)
)

// CaddyModule returns the Caddy module information.
func (UTMTracker) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "http.handlers.utm_tracker",
		New: func() caddy.Module { return new(UTMTracker) },
	}
}

// Provision sets up the middleware.
func (t *UTMTracker) Provision(ctx caddy.Context) error {
	t.logger = ctx.Logger()

	// Apply defaults
	if t.Endpoint == "" {
		t.Endpoint = "https://api.openpanel.dev"
	}
	if t.CookieName == "" {
		t.CookieName = "_vid"
	}
	if t.CookieMaxAge == 0 {
		t.CookieMaxAge = 365 * 24 * 3600 // 365 days
	}
	if t.CookieSecure == nil {
		b := true
		t.CookieSecure = &b
	}
	if t.CookieHTTPOnly == nil {
		b := true
		t.CookieHTTPOnly = &b
	}
	if t.CookieSameSite == "" {
		t.CookieSameSite = "lax"
	}
	if t.SessionCookie == "" {
		t.SessionCookie = "_sid"
	}
	if t.SessionMaxAge == 0 {
		t.SessionMaxAge = 30 * 60 // 30 minutes
	}
	if len(t.TrackMethods) == 0 {
		t.TrackMethods = []string{"GET"}
	}
	if len(t.TrackPaths) == 0 {
		t.TrackPaths = []string{"/*"}
	}
	if len(t.IgnorePaths) == 0 {
		t.IgnorePaths = []string{
			"/api/*",
			"/static/*",
			"/_next/*",
			"/favicon.ico",
			"*.css",
			"*.js",
			"*.png",
			"*.jpg",
			"*.jpeg",
			"*.gif",
			"*.svg",
			"*.woff",
			"*.woff2",
			"*.ttf",
			"*.eot",
			"*.ico",
			"*.map",
		}
	}
	if t.BatchSize == 0 {
		t.BatchSize = 50
	}
	if t.FlushInterval == "" {
		t.FlushInterval = "5s"
	}

	// Parse flush interval
	var err error
	t.flushDuration, err = time.ParseDuration(t.FlushInterval)
	if err != nil {
		return fmt.Errorf("invalid flush_interval %q: %w", t.FlushInterval, err)
	}

	// Build method set for fast lookup
	t.trackMethodSet = make(map[string]bool, len(t.TrackMethods))
	for _, m := range t.TrackMethods {
		t.trackMethodSet[strings.ToUpper(m)] = true
	}

	// Create HTTP client with connection pooling
	t.httpClient = &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
		},
	}

	// Create OpenPanel client
	t.client = &OpenPanelClient{
		Endpoint:     strings.TrimRight(t.Endpoint, "/"),
		ClientID:     t.ClientID,
		ClientSecret: t.ClientSecret,
		HTTPClient:   t.httpClient,
		Logger:       t.logger,
	}

	// Create event buffer
	t.buffer = NewEventBuffer(t.client, t.logger, t.BatchSize, t.flushDuration)

	t.logger.Info("utm_tracker provisioned",
		zap.String("endpoint", t.Endpoint),
		zap.String("client_id", t.ClientID),
		zap.Int("batch_size", t.BatchSize),
		zap.Duration("flush_interval", t.flushDuration),
	)

	return nil
}

// Validate ensures the configuration is valid.
func (t *UTMTracker) Validate() error {
	if t.ClientID == "" {
		return fmt.Errorf("client_id is required")
	}
	if t.BatchSize < 1 {
		return fmt.Errorf("batch_size must be at least 1")
	}
	if t.flushDuration < 1*time.Second {
		return fmt.Errorf("flush_interval must be at least 1s")
	}

	sameSite := strings.ToLower(t.CookieSameSite)
	if sameSite != "lax" && sameSite != "strict" && sameSite != "none" {
		return fmt.Errorf("cookie_same_site must be lax, strict, or none")
	}

	return nil
}

// Cleanup flushes remaining events and releases resources.
func (t *UTMTracker) Cleanup() error {
	if t.logger != nil {
		t.logger.Info("utm_tracker shutting down, flushing remaining events")
	}
	if t.buffer != nil {
		t.buffer.Stop()
	}
	if t.httpClient != nil {
		t.httpClient.CloseIdleConnections()
	}
	return nil
}

// ServeHTTP implements caddyhttp.MiddlewareHandler.
func (t *UTMTracker) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) error {
	// Check if method should be tracked
	if !t.trackMethodSet[r.Method] {
		return next.ServeHTTP(w, r)
	}

	// Bot check
	if isBot(r.UserAgent(), t.BotPatterns) {
		t.logger.Debug("skipping bot request", zap.String("ua", r.UserAgent()))
		return next.ServeHTTP(w, r)
	}

	// Path filtering
	if t.shouldIgnorePath(r.URL.Path) {
		return next.ServeHTTP(w, r)
	}

	if !t.shouldTrackPath(r.URL.Path) {
		return next.ServeHTTP(w, r)
	}

	// Check only_with_params
	if t.OnlyWithParams && !t.hasTrackingParams(r) {
		return next.ServeHTTP(w, r)
	}

	// Handle visitor cookie
	visitorID := t.getOrCreateCookie(w, r, t.CookieName, t.CookieMaxAge)

	// Handle session cookie (sliding expiration)
	sessionID := t.getOrCreateCookie(w, r, t.SessionCookie, t.SessionMaxAge)

	// Build and push event
	event := buildEvent(r, visitorID, sessionID, t.ExtraParams)

	t.logger.Debug("tracking event",
		zap.String("visitor_id", visitorID),
		zap.String("session_id", sessionID),
		zap.String("path", r.URL.Path),
	)

	t.buffer.Push(event)

	// Strip params if configured
	if t.StripParams {
		stripTrackingParams(r.URL, t.ExtraParams)
	}

	return next.ServeHTTP(w, r)
}

// getOrCreateCookie reads an existing cookie or creates a new one with a UUIDv4.
func (t *UTMTracker) getOrCreateCookie(w http.ResponseWriter, r *http.Request, name string, maxAge int) string {
	if c, err := r.Cookie(name); err == nil && c.Value != "" {
		// Extend cookie lifetime (sliding expiration)
		t.setCookie(w, r, name, c.Value, maxAge)
		return c.Value
	}

	id := uuid.New().String()
	t.setCookie(w, r, name, id, maxAge)
	return id
}

func (t *UTMTracker) setCookie(w http.ResponseWriter, r *http.Request, name, value string, maxAge int) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   *t.CookieSecure,
		HttpOnly: *t.CookieHTTPOnly,
	}

	if t.CookieDomain != "" {
		cookie.Domain = t.CookieDomain
	}

	switch strings.ToLower(t.CookieSameSite) {
	case "strict":
		cookie.SameSite = http.SameSiteStrictMode
	case "none":
		cookie.SameSite = http.SameSiteNoneMode
	default:
		cookie.SameSite = http.SameSiteLaxMode
	}

	http.SetCookie(w, cookie)
}

func (t *UTMTracker) shouldIgnorePath(path string) bool {
	for _, pattern := range t.IgnorePaths {
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Handle prefix patterns like /api/*
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(path, prefix+"/") || path == prefix {
				return true
			}
		}
	}
	return false
}

func (t *UTMTracker) shouldTrackPath(path string) bool {
	for _, pattern := range t.TrackPaths {
		if pattern == "/*" {
			return true
		}
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Handle prefix patterns
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(path, prefix+"/") || path == prefix {
				return true
			}
		}
	}
	return false
}

func (t *UTMTracker) hasTrackingParams(r *http.Request) bool {
	query := r.URL.Query()
	for param := range defaultParams {
		if query.Has(param) {
			return true
		}
	}
	for _, param := range t.ExtraParams {
		if query.Has(param) {
			return true
		}
	}
	return false
}

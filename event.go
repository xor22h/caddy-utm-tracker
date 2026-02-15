package caddyutmtracker

import (
	"net/http"
	"net/url"
	"strings"
)

// TrackingEvent represents a captured marketing attribution event.
type TrackingEvent struct {
	VisitorID  string            `json:"visitor_id"`
	SessionID  string            `json:"session_id"`
	Path       string            `json:"path"`
	Referrer   string            `json:"referrer"`
	UserAgent  string            `json:"user_agent"`
	ClientIP   string            `json:"client_ip"`
	Properties map[string]string `json:"properties"`
}

// defaultParams are the built-in UTM/tracking parameters to extract.
var defaultParams = map[string]string{
	"utm_source":   "utm_source",
	"utm_medium":   "utm_medium",
	"utm_campaign": "utm_campaign",
	"utm_term":     "utm_term",
	"utm_content":  "utm_content",
	"ref":          "referral_code",
	"gclid":        "gclid",
	"fbclid":       "fbclid",
	"msclkid":      "msclkid",
}

// extractParams extracts tracking parameters from the query string.
// extraParams are additional parameter names to track (mapped to themselves).
func extractParams(query url.Values, extraParams []string) map[string]string {
	props := make(map[string]string)

	for param, propName := range defaultParams {
		if v := query.Get(param); v != "" {
			props[propName] = v
		}
	}

	for _, param := range extraParams {
		if v := query.Get(param); v != "" {
			props[param] = v
		}
	}

	return props
}

// stripTrackingParams removes UTM/tracking parameters from the URL query string.
// Returns the modified URL and whether any params were removed.
func stripTrackingParams(u *url.URL, extraParams []string) bool {
	query := u.Query()
	changed := false

	for param := range defaultParams {
		if query.Has(param) {
			query.Del(param)
			changed = true
		}
	}

	for _, param := range extraParams {
		if query.Has(param) {
			query.Del(param)
			changed = true
		}
	}

	if changed {
		u.RawQuery = query.Encode()
	}
	return changed
}

// clientIP extracts the real client IP from the request,
// respecting X-Forwarded-For, X-Real-IP, and falling back to RemoteAddr.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the chain (original client)
		if idx := strings.IndexByte(xff, ','); idx != -1 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	// Fall back to RemoteAddr, stripping port
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

// buildEvent creates a TrackingEvent from the request and extracted data.
func buildEvent(r *http.Request, visitorID, sessionID string, extraParams []string) TrackingEvent {
	props := extractParams(r.URL.Query(), extraParams)
	props["path"] = r.URL.Path

	// Include the full URL with scheme and host
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	props["__url"] = scheme + "://" + r.Host + r.URL.RequestURI()
	props["__domain"] = r.Host

	if ref := r.Header.Get("Referer"); ref != "" {
		props["referrer"] = ref
	}

	return TrackingEvent{
		VisitorID:  visitorID,
		SessionID:  sessionID,
		Path:       r.URL.Path,
		Referrer:   r.Header.Get("Referer"),
		UserAgent:  r.UserAgent(),
		ClientIP:   clientIP(r),
		Properties: props,
	}
}

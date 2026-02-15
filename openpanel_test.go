package caddyutmtracker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestOpenPanelClient_SendEvents(t *testing.T) {
	var receivedRequests []openPanelRequest
	var receivedHeaders []http.Header

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/track" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		var req openPanelRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		receivedRequests = append(receivedRequests, req)
		receivedHeaders = append(receivedHeaders, r.Header.Clone())
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &OpenPanelClient{
		Endpoint:     server.URL,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		HTTPClient:   server.Client(),
		Logger:       zap.NewNop(),
	}

	events := []TrackingEvent{
		{
			VisitorID: "visitor-1",
			SessionID: "session-1",
			Path:      "/pricing",
			Referrer:  "https://google.com",
			UserAgent: "Mozilla/5.0 Test",
			ClientIP:  "1.2.3.4",
			Properties: map[string]string{
				"path":         "/pricing",
				"referrer":     "https://google.com",
				"utm_source":   "twitter",
				"utm_campaign": "launch",
			},
		},
		{
			VisitorID: "visitor-2",
			SessionID: "session-2",
			Path:      "/about",
			UserAgent: "Mozilla/5.0 Test2",
			ClientIP:  "5.6.7.8",
			Properties: map[string]string{
				"path": "/about",
			},
		},
	}

	err := client.SendEvents(context.Background(), events)
	if err != nil {
		t.Fatalf("SendEvents failed: %v", err)
	}

	if len(receivedRequests) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(receivedRequests))
	}

	// Verify first event
	req := receivedRequests[0]
	if req.Type != "track" {
		t.Errorf("type = %q, want %q", req.Type, "track")
	}
	if req.Payload.Name != "page_view" {
		t.Errorf("name = %q, want %q", req.Payload.Name, "page_view")
	}
	if req.Payload.ProfileID != "visitor-1" {
		t.Errorf("profileId = %q, want %q", req.Payload.ProfileID, "visitor-1")
	}
	if req.Payload.Properties["utm_source"] != "twitter" {
		t.Errorf("utm_source = %q, want %q", req.Payload.Properties["utm_source"], "twitter")
	}

	// Verify headers
	headers := receivedHeaders[0]
	if headers.Get("Content-Type") != "application/json" {
		t.Errorf("Content-Type = %q, want %q", headers.Get("Content-Type"), "application/json")
	}
	if headers.Get("openpanel-client-id") != "test-client-id" {
		t.Errorf("openpanel-client-id = %q, want %q", headers.Get("openpanel-client-id"), "test-client-id")
	}
	if headers.Get("openpanel-client-secret") != "test-client-secret" {
		t.Errorf("openpanel-client-secret = %q, want %q", headers.Get("openpanel-client-secret"), "test-client-secret")
	}
	if headers.Get("x-client-ip") != "1.2.3.4" {
		t.Errorf("x-client-ip = %q, want %q", headers.Get("x-client-ip"), "1.2.3.4")
	}
	if headers.Get("User-Agent") != "Mozilla/5.0 Test" {
		t.Errorf("User-Agent = %q, want %q", headers.Get("User-Agent"), "Mozilla/5.0 Test")
	}
}

func TestOpenPanelClient_SendEvents_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := &OpenPanelClient{
		Endpoint:   server.URL,
		ClientID:   "test",
		HTTPClient: server.Client(),
		Logger:     zap.NewNop(),
	}

	events := []TrackingEvent{
		{
			VisitorID:  "v1",
			Properties: map[string]string{"path": "/"},
		},
	}

	err := client.SendEvents(context.Background(), events)
	if err == nil {
		t.Fatal("expected error for server error response")
	}
}

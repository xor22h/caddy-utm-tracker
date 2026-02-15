package caddyutmtracker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

// OpenPanelClient sends tracking events to the OpenPanel API.
type OpenPanelClient struct {
	Endpoint     string
	ClientID     string
	ClientSecret string
	HTTPClient   *http.Client
	Logger       *zap.Logger
}

// openPanelRequest is the payload sent to the OpenPanel /track endpoint.
type openPanelRequest struct {
	Type    string               `json:"type"`
	Payload openPanelEventPayload `json:"payload"`
}

type openPanelEventPayload struct {
	Name       string            `json:"name"`
	ProfileID  string            `json:"profileId"`
	Properties map[string]string `json:"properties"`
}

// SendEvents sends a batch of tracking events to OpenPanel.
// Each event is sent as an individual request since the OpenPanel /track
// endpoint accepts single events.
func (c *OpenPanelClient) SendEvents(ctx context.Context, events []TrackingEvent) error {
	var lastErr error

	for _, event := range events {
		if err := c.sendEvent(ctx, event); err != nil {
			c.Logger.Warn("failed to send event to OpenPanel",
				zap.String("visitor_id", event.VisitorID),
				zap.Error(err),
			)
			lastErr = err
		}
	}

	return lastErr
}

func (c *OpenPanelClient) sendEvent(ctx context.Context, event TrackingEvent) error {
	payload := openPanelRequest{
		Type: "track",
		Payload: openPanelEventPayload{
			Name:       "page_view",
			ProfileID:  event.VisitorID,
			Properties: event.Properties,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal event: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint+"/track", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("openpanel-client-id", c.ClientID)
	if c.ClientSecret != "" {
		req.Header.Set("openpanel-client-secret", c.ClientSecret)
	}
	req.Header.Set("x-client-ip", event.ClientIP)
	req.Header.Set("User-Agent", event.UserAgent)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("OpenPanel API returned status %d", resp.StatusCode)
	}

	return nil
}

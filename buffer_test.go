package caddyutmtracker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestEventBuffer_FlushOnBatchSize(t *testing.T) {
	var received atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &OpenPanelClient{
		Endpoint:   server.URL,
		ClientID:   "test",
		HTTPClient: server.Client(),
		Logger:     zap.NewNop(),
	}

	buf := NewEventBuffer(client, zap.NewNop(), 5, 1*time.Minute)
	defer buf.Stop()

	// Push exactly batch_size events
	for i := 0; i < 5; i++ {
		buf.Push(TrackingEvent{
			VisitorID:  "v1",
			Properties: map[string]string{"path": "/"},
		})
	}

	// Wait for flush
	time.Sleep(500 * time.Millisecond)

	count := received.Load()
	if count != 5 {
		t.Errorf("expected 5 events sent, got %d", count)
	}
}

func TestEventBuffer_FlushOnInterval(t *testing.T) {
	var received atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &OpenPanelClient{
		Endpoint:   server.URL,
		ClientID:   "test",
		HTTPClient: server.Client(),
		Logger:     zap.NewNop(),
	}

	buf := NewEventBuffer(client, zap.NewNop(), 100, 200*time.Millisecond)
	defer buf.Stop()

	// Push fewer than batch_size events
	buf.Push(TrackingEvent{
		VisitorID:  "v1",
		Properties: map[string]string{"path": "/"},
	})
	buf.Push(TrackingEvent{
		VisitorID:  "v2",
		Properties: map[string]string{"path": "/about"},
	})

	// Wait for timer flush
	time.Sleep(500 * time.Millisecond)

	count := received.Load()
	if count != 2 {
		t.Errorf("expected 2 events sent, got %d", count)
	}
}

func TestEventBuffer_FlushOnStop(t *testing.T) {
	var mu sync.Mutex
	var received int

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		received++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &OpenPanelClient{
		Endpoint:   server.URL,
		ClientID:   "test",
		HTTPClient: server.Client(),
		Logger:     zap.NewNop(),
	}

	buf := NewEventBuffer(client, zap.NewNop(), 100, 1*time.Minute)

	buf.Push(TrackingEvent{
		VisitorID:  "v1",
		Properties: map[string]string{"path": "/"},
	})

	// Give time for the event to be received by the run goroutine
	time.Sleep(50 * time.Millisecond)

	buf.Stop()

	mu.Lock()
	defer mu.Unlock()
	if received != 1 {
		t.Errorf("expected 1 event flushed on stop, got %d", received)
	}
}

func TestEventBuffer_DropOnFullChannel(t *testing.T) {
	// Server that blocks to simulate slow consumer
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := &OpenPanelClient{
		Endpoint:   server.URL,
		ClientID:   "test",
		HTTPClient: &http.Client{Timeout: 1 * time.Second},
		Logger:     zap.NewNop(),
	}

	// Create buffer with tiny capacity to test dropping
	buf := &EventBuffer{
		client:        client,
		logger:        zap.NewNop(),
		batchSize:     1000,
		flushInterval: 1 * time.Minute,
		events:        make([]TrackingEvent, 0, 1000),
		eventCh:       make(chan TrackingEvent, 2), // very small channel
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}
	go buf.run()
	defer buf.Stop()

	// Fill up the channel - first 2 should succeed, rest may be dropped
	for i := 0; i < 10; i++ {
		buf.Push(TrackingEvent{
			VisitorID:  "v1",
			Properties: map[string]string{"path": "/"},
		})
	}

	// No panic or deadlock = success
	_ = context.Background()
}

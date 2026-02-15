package caddyutmtracker

import (
	"context"
	"sync"
	"time"

	"go.uber.org/zap"
)

// EventBuffer collects tracking events and flushes them in batches.
type EventBuffer struct {
	client        *OpenPanelClient
	logger        *zap.Logger
	batchSize     int
	flushInterval time.Duration

	mu     sync.Mutex
	events []TrackingEvent

	eventCh chan TrackingEvent
	stopCh  chan struct{}
	doneCh  chan struct{}
}

// NewEventBuffer creates a new event buffer that batches events and flushes
// them either when batchSize is reached or flushInterval elapses.
func NewEventBuffer(client *OpenPanelClient, logger *zap.Logger, batchSize int, flushInterval time.Duration) *EventBuffer {
	b := &EventBuffer{
		client:        client,
		logger:        logger,
		batchSize:     batchSize,
		flushInterval: flushInterval,
		events:        make([]TrackingEvent, 0, batchSize),
		eventCh:       make(chan TrackingEvent, batchSize*4),
		stopCh:        make(chan struct{}),
		doneCh:        make(chan struct{}),
	}

	go b.run()
	return b
}

// Push adds an event to the buffer. Non-blocking; drops if channel is full.
func (b *EventBuffer) Push(event TrackingEvent) {
	select {
	case b.eventCh <- event:
	default:
		b.logger.Warn("event buffer full, dropping event",
			zap.String("visitor_id", event.VisitorID),
			zap.String("path", event.Path),
		)
	}
}

// Stop signals the buffer to flush remaining events and shut down.
func (b *EventBuffer) Stop() {
	close(b.stopCh)
	<-b.doneCh
}

func (b *EventBuffer) run() {
	defer close(b.doneCh)

	ticker := time.NewTicker(b.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case event := <-b.eventCh:
			b.mu.Lock()
			b.events = append(b.events, event)
			shouldFlush := len(b.events) >= b.batchSize
			b.mu.Unlock()

			if shouldFlush {
				b.flush()
			}

		case <-ticker.C:
			b.flush()

		case <-b.stopCh:
			// Drain remaining events from channel
			for {
				select {
				case event := <-b.eventCh:
					b.mu.Lock()
					b.events = append(b.events, event)
					b.mu.Unlock()
				default:
					b.flush()
					return
				}
			}
		}
	}
}

func (b *EventBuffer) flush() {
	b.mu.Lock()
	if len(b.events) == 0 {
		b.mu.Unlock()
		return
	}

	batch := b.events
	b.events = make([]TrackingEvent, 0, b.batchSize)
	b.mu.Unlock()

	start := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := b.client.SendEvents(ctx, batch)
	duration := time.Since(start)

	if err != nil {
		b.logger.Warn("failed to flush events to OpenPanel",
			zap.Int("count", len(batch)),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
	} else {
		b.logger.Info("flushed events to OpenPanel",
			zap.Int("count", len(batch)),
			zap.Duration("duration", duration),
		)
	}
}

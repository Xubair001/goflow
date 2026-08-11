package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// event is one Server-Sent Event's "data:" field. There's only one kind of
// event today (a stats snapshot), so there's no separate "event:" name to
// carry — add one if/when a second event type shows up.
type event struct {
	Data []byte
}

// broadcaster fans out events to any number of connected SSE clients. A
// slow or stalled client never blocks publishing to the others: its
// channel is buffered, and a full buffer just drops the update for that
// one client rather than backing up the whole broadcaster.
type broadcaster struct {
	mu      sync.Mutex
	clients map[chan event]struct{}
}

func newBroadcaster() *broadcaster {
	return &broadcaster{clients: make(map[chan event]struct{})}
}

func (b *broadcaster) subscribe() chan event {
	ch := make(chan event, 8)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *broadcaster) unsubscribe(ch chan event) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
	close(ch)
}

func (b *broadcaster) publish(data []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- event{Data: data}:
		default: // slow client; drop this update rather than block the rest
		}
	}
}

func (b *broadcaster) subscriberCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.clients)
}

// handleEvents streams queue stats over Server-Sent Events. Clients should
// still fetch /api/v1/queue/stats for their initial render and treat this
// as a live update feed layered on top of it.
func (s *Server) handleEvents(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	// Flushed immediately, before the first event: otherwise a client
	// waiting on the response headers (e.g. to confirm the connection
	// opened) would block until the first stats snapshot ever publishes.
	c.Status(http.StatusOK)
	c.Writer.Flush()

	ch := s.broadcaster.subscribe()
	defer s.broadcaster.unsubscribe(ch)

	c.Stream(func(w io.Writer) bool {
		select {
		case <-c.Request.Context().Done():
			return false
		case evt, ok := <-ch:
			if !ok {
				return false
			}
			_, err := fmt.Fprintf(w, "event: stats\ndata: %s\n\n", evt.Data)
			return err == nil
		}
	})
}

// RunEventsPoller periodically publishes queue stats to connected SSE
// clients until ctx is cancelled.
//
// Polling the store rather than reacting to individual writes is the
// simplest thing that works: job status changes happen in separate
// worker/dispatcher processes, so the apiserver has no in-process signal
// to react to. Postgres LISTEN/NOTIFY would give lower-latency updates at
// the cost of another moving part; worth revisiting if 2-second staleness
// ever matters.
func (s *Server) RunEventsPoller(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		stats, err := s.store.Stats(ctx)
		if err != nil {
			s.logger.Error("events poller: fetch stats", "error", err)
			continue
		}
		data, err := json.Marshal(stats)
		if err != nil {
			s.logger.Error("events poller: marshal stats", "error", err)
			continue
		}
		s.broadcaster.publish(data)
	}
}

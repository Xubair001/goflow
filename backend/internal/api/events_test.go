package api

import (
	"bufio"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestBroadcaster_PublishToSubscribers(t *testing.T) {
	b := newBroadcaster()
	ch1 := b.subscribe()
	ch2 := b.subscribe()
	defer b.unsubscribe(ch1)
	defer b.unsubscribe(ch2)

	b.publish([]byte(`{"pending":1}`))

	for i, ch := range []chan event{ch1, ch2} {
		select {
		case evt := <-ch:
			if string(evt.Data) != `{"pending":1}` {
				t.Errorf("subscriber %d: Data = %q, want %q", i, evt.Data, `{"pending":1}`)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d did not receive the published event", i)
		}
	}
}

func TestBroadcaster_UnsubscribeClosesChannel(t *testing.T) {
	b := newBroadcaster()
	ch := b.subscribe()
	b.unsubscribe(ch)

	b.publish([]byte(`{}`)) // must not panic sending to a removed subscriber

	if _, ok := <-ch; ok {
		t.Error("channel should be closed after unsubscribe")
	}
}

func TestBroadcaster_SlowSubscriberDoesNotBlockOthers(t *testing.T) {
	b := newBroadcaster()
	slow := b.subscribe() // intentionally never drained
	fast := b.subscribe()
	defer b.unsubscribe(slow)
	defer b.unsubscribe(fast)

	// Fill the slow subscriber's buffer; the next publish should drop for
	// slow rather than block delivery to fast.
	for range 8 {
		b.publish([]byte(`{}`))
	}

	done := make(chan struct{})
	go func() {
		b.publish([]byte(`{"final":true}`))
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("publish blocked despite a slow subscriber's full buffer")
	}

	select {
	case <-fast:
	case <-time.After(time.Second):
		t.Fatal("fast subscriber did not receive any event")
	}
}

func TestBroadcaster_SubscriberCount(t *testing.T) {
	b := newBroadcaster()
	if got := b.subscriberCount(); got != 0 {
		t.Errorf("subscriberCount() = %d, want 0", got)
	}
	ch := b.subscribe()
	if got := b.subscriberCount(); got != 1 {
		t.Errorf("subscriberCount() = %d, want 1", got)
	}
	b.unsubscribe(ch)
	if got := b.subscriberCount(); got != 0 {
		t.Errorf("subscriberCount() = %d, want 0", got)
	}
}

// TestHandleEvents_StreamsPublishedEvent wires a real HTTP server (SSE
// needs genuine streaming, which httptest.ResponseRecorder can't provide)
// and checks a published event actually reaches a connected client.
func TestHandleEvents_StreamsPublishedEvent(t *testing.T) {
	srv := testServer(t, &fakeStore{})
	httpSrv := httptest.NewServer(srv.Routes())
	defer httpSrv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, httpSrv.URL+"/api/v1/events", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("Content-Type = %q, want %q", ct, "text/event-stream")
	}

	deadline := time.Now().Add(2 * time.Second)
	for srv.broadcaster.subscriberCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if srv.broadcaster.subscriberCount() == 0 {
		t.Fatal("handler never subscribed to the broadcaster")
	}

	srv.broadcaster.publish([]byte(`{"pending":5}`))

	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("read from stream: %v", err)
	}
	if !strings.HasPrefix(line, "event: stats") {
		t.Errorf("first line = %q, want prefix %q", line, "event: stats")
	}
}

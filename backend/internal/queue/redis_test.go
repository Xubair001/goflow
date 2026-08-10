//go:build integration

package queue_test

import (
	"context"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"github.com/abdullah-zubair/jobqueue/internal/queue"
)

var redisClient *redis.Client

func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

func runTests(m *testing.M) int {
	ctx := context.Background()

	container, err := tcredis.Run(ctx, "redis:8-alpine")
	if err != nil {
		log.Fatalf("start redis container: %v", err)
	}
	defer func() { _ = container.Terminate(ctx) }()

	connStr, err := container.ConnectionString(ctx)
	if err != nil {
		log.Fatalf("build connection string: %v", err)
	}
	opts, err := redis.ParseURL(connStr)
	if err != nil {
		log.Fatalf("parse redis url: %v", err)
	}

	redisClient = redis.NewClient(opts)
	defer redisClient.Close()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("ping redis: %v", err)
	}

	return m.Run()
}

// newTestQueue gives each test its own stream/group name so tests can share
// the one Redis container without cleaning up global state between them.
func newTestQueue(t *testing.T) *queue.Redis {
	t.Helper()
	ctx := context.Background()
	stream := fmt.Sprintf("test-stream-%s-%d", t.Name(), time.Now().UnixNano())

	q, err := queue.NewRedis(ctx, redisClient, stream, "workers", 200*time.Millisecond)
	if err != nil {
		t.Fatalf("NewRedis() error = %v", err)
	}
	t.Cleanup(func() {
		redisClient.Del(context.Background(), stream)
	})
	return q
}

func TestRedis_PublishAndConsume(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	if err := q.Publish(ctx, "job-1"); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	msgs, err := q.Consume(ctx, "consumer-1", 10)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("Consume() returned %d messages, want 1", len(msgs))
	}
	if msgs[0].JobID != "job-1" {
		t.Errorf("JobID = %q, want %q", msgs[0].JobID, "job-1")
	}

	if err := q.Ack(ctx, msgs[0]); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}
}

func TestRedis_Consume_NoMessages_ReturnsEmpty(t *testing.T) {
	q := newTestQueue(t)
	msgs, err := q.Consume(context.Background(), "consumer-1", 10)
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}
	if len(msgs) != 0 {
		t.Errorf("Consume() = %v, want empty", msgs)
	}
}

// TestRedis_ConsumerGroup_DisjointDelivery proves the property the worker
// pool depends on: a consumer group never delivers the same stream entry to
// two different consumers.
func TestRedis_ConsumerGroup_DisjointDelivery(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	const total = 20
	for i := range total {
		if err := q.Publish(ctx, fmt.Sprintf("job-%d", i)); err != nil {
			t.Fatalf("Publish() error = %v", err)
		}
	}

	seen := make(map[string]int)
	for _, consumer := range []string{"consumer-a", "consumer-b"} {
		msgs, err := q.Consume(ctx, consumer, total)
		if err != nil {
			t.Fatalf("Consume(%s) error = %v", consumer, err)
		}
		for _, m := range msgs {
			seen[m.JobID]++
		}
	}

	if len(seen) != total {
		t.Fatalf("saw %d distinct jobs across consumers, want %d", len(seen), total)
	}
	for id, count := range seen {
		if count != 1 {
			t.Errorf("job %s delivered %d times, want exactly 1", id, count)
		}
	}
}

// TestRedis_Reclaim_PicksUpAbandonedMessage simulates a worker crashing
// after Consume but before Ack, and checks the reconciler's tool for
// recovering that job (Reclaim, backed by XAUTOCLAIM) works.
func TestRedis_Reclaim_PicksUpAbandonedMessage(t *testing.T) {
	q := newTestQueue(t)
	ctx := context.Background()

	if err := q.Publish(ctx, "job-1"); err != nil {
		t.Fatalf("Publish() error = %v", err)
	}

	msgs, err := q.Consume(ctx, "consumer-a", 10)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("Consume() = %v, %v", msgs, err)
	}
	// consumer-a never Acks -- as if it crashed mid-job.

	reclaimed, err := q.Reclaim(ctx, "consumer-b", 0, 10)
	if err != nil {
		t.Fatalf("Reclaim() error = %v", err)
	}
	if len(reclaimed) != 1 || reclaimed[0].JobID != "job-1" {
		t.Fatalf("Reclaim() = %v, want exactly [job-1]", reclaimed)
	}

	if err := q.Ack(ctx, reclaimed[0]); err != nil {
		t.Fatalf("Ack() error = %v", err)
	}

	again, err := q.Reclaim(ctx, "consumer-b", 0, 10)
	if err != nil {
		t.Fatalf("Reclaim() second call error = %v", err)
	}
	if len(again) != 0 {
		t.Errorf("Reclaim() after Ack = %v, want empty", again)
	}
}

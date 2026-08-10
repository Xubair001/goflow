// Package queue is the fast delivery layer between the dispatcher and the
// worker pool. It carries only a hint — a job ID — telling a worker "look
// at this row in Postgres"; the queue itself is never the source of truth
// for job state (see package store).
package queue

import (
	"context"
	"time"
)

// Message is one delivered queue entry: a job ID plus the identifier a
// consumer needs to acknowledge it.
//
// Delivery counts are deliberately not tracked here: package store already
// tracks Job.Attempts against Job.MaxAttempts, and that application-level
// count is what retry/dead-lettering decisions are based on. A second,
// queue-level counter would just be state nobody reads.
type Message struct {
	// ID is the queue's own entry ID (e.g. a Redis Stream ID), used to Ack
	// this specific delivery — not the job's ID.
	ID string
	// JobID is the store.Store job identifier this delivery refers to.
	JobID string
}

// Queue is the transport between dispatcher (producer) and workers
// (consumer group members). Implementations must provide at-least-once
// delivery: a message is only removed from the pending set on Ack, so a
// consumer that dies mid-job leaves it reclaimable by Reclaim.
type Queue interface {
	// Publish enqueues jobID for delivery to the consumer group.
	Publish(ctx context.Context, jobID string) error

	// Consume blocks (up to the implementation's own poll timeout) for up
	// to max new messages not yet delivered to any consumer in the group.
	// An empty, nil-error result is a normal timeout, not a failure.
	Consume(ctx context.Context, consumer string, max int64) ([]Message, error)

	// Ack removes a message from the pending set after it has been fully
	// processed (success or a terminal failure already recorded in Postgres).
	Ack(ctx context.Context, msg Message) error

	// Reclaim takes ownership of messages idle (undelivered-and-unacked)
	// for longer than minIdle, handing them back to whichever consumer
	// calls Reclaim — this is how a dead worker's in-flight job gets picked
	// up by another one.
	Reclaim(ctx context.Context, consumer string, minIdle time.Duration, max int64) ([]Message, error)
}

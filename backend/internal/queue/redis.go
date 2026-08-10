package queue

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// jobIDField is the Stream entry field holding the job's store ID.
const jobIDField = "job_id"

// Redis is a Queue backed by a Redis Stream and consumer group, giving
// at-least-once delivery: XReadGroup hands a message to exactly one
// consumer and parks it in the group's Pending Entries List until Ack'd;
// Reclaim (XAUTOCLAIM) hands abandoned entries to a new consumer.
type Redis struct {
	client *redis.Client
	stream string
	group  string
	block  time.Duration
}

var _ Queue = (*Redis)(nil)

// NewRedis returns a Queue over the given stream and consumer group,
// creating both if they don't already exist. block bounds how long Consume
// waits for new entries before returning an empty result.
func NewRedis(ctx context.Context, client *redis.Client, stream, group string, block time.Duration) (*Redis, error) {
	q := &Redis{client: client, stream: stream, group: group, block: block}
	if err := q.ensureGroup(ctx); err != nil {
		return nil, err
	}
	return q, nil
}

// ensureGroup creates the consumer group starting from the beginning of the
// stream. Redis has no "create if not exists" for groups, so a BUSYGROUP
// error — meaning it's already there — is treated as success.
func (q *Redis) ensureGroup(ctx context.Context) error {
	err := q.client.XGroupCreateMkStream(ctx, q.stream, q.group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return fmt.Errorf("queue: create consumer group %s/%s: %w", q.stream, q.group, err)
	}
	return nil
}

// Publish implements Queue.
func (q *Redis) Publish(ctx context.Context, jobID string) error {
	err := q.client.XAdd(ctx, &redis.XAddArgs{
		Stream: q.stream,
		Values: map[string]any{jobIDField: jobID},
	}).Err()
	if err != nil {
		return fmt.Errorf("queue: publish job %s: %w", jobID, err)
	}
	return nil
}

func (q *Redis) Consume(ctx context.Context, consumer string, max int64) ([]Message, error) {
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    q.group,
		Consumer: consumer,
		Streams:  []string{q.stream, ">"},
		Count:    max,
		Block:    q.block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil // normal poll timeout, nothing new
	}
	if err != nil {
		return nil, fmt.Errorf("queue: consume: %w", err)
	}
	if len(streams) == 0 {
		return nil, nil
	}
	return toMessages(streams[0].Messages), nil
}

func (q *Redis) Ack(ctx context.Context, msg Message) error {
	if err := q.client.XAck(ctx, q.stream, q.group, msg.ID).Err(); err != nil {
		return fmt.Errorf("queue: ack %s: %w", msg.ID, err)
	}
	return nil
}

func (q *Redis) Reclaim(ctx context.Context, consumer string, minIdle time.Duration, max int64) ([]Message, error) {
	messages, _, err := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   q.stream,
		Group:    q.group,
		Consumer: consumer,
		MinIdle:  minIdle,
		Start:    "0",
		Count:    max,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("queue: reclaim: %w", err)
	}
	return toMessages(messages), nil
}

func toMessages(raw []redis.XMessage) []Message {
	if len(raw) == 0 {
		return nil
	}
	messages := make([]Message, 0, len(raw))
	for _, m := range raw {
		jobID, _ := m.Values[jobIDField].(string)
		if jobID == "" {
			continue // malformed entry (shouldn't happen for anything we published)
		}
		messages = append(messages, Message{ID: m.ID, JobID: jobID})
	}
	return messages
}

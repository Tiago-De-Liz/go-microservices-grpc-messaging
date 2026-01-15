package broker

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Queue struct {
	mu                sync.Mutex
	name              string
	messages          []*Message
	visibilityTimeout time.Duration
	maxRetries        int
	deadLetterQueue   *Queue
	stats             QueueStats
}

type QueueStats struct {
	TotalReceived  int64
	TotalProcessed int64
	TotalFailed    int64
	CurrentSize    int
}

func (q *Queue) Name() string {
	return q.name
}

func (q *Queue) Enqueue(ctx context.Context, msg *Message) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	q.messages = append(q.messages, msg)
	q.stats.TotalReceived++
	q.stats.CurrentSize = len(q.messages)

	logDebug("Enqueued message '%s' to queue '%s'", msg.ID, q.name)

	return nil
}

func (q *Queue) Receive(ctx context.Context) (*Message, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()

	for _, msg := range q.messages {
		if msg.IsVisible() {
			msg.VisibleAt = now.Add(q.visibilityTimeout)
			msg.ReceiptHandle = uuid.New().String()
			msg.RetryCount++

			logDebug("Received message '%s' from queue '%s' (retry %d)",
				msg.ID, q.name, msg.RetryCount)

			return msg, nil
		}
	}

	return nil, nil
}

func (q *Queue) Acknowledge(ctx context.Context, receiptHandle string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for i, msg := range q.messages {
		if msg.ReceiptHandle == receiptHandle {
			q.messages = append(q.messages[:i], q.messages[i+1:]...)
			q.stats.TotalProcessed++
			q.stats.CurrentSize = len(q.messages)

			logDebug("Acknowledged message with receipt '%s' from queue '%s'",
				receiptHandle, q.name)

			return nil
		}
	}

	return ErrInvalidReceiptHandle
}

func (q *Queue) Nack(ctx context.Context, receiptHandle string) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	for _, msg := range q.messages {
		if msg.ReceiptHandle == receiptHandle {
			if msg.RetryCount >= q.maxRetries {
				return q.moveToDeadLetterQueueLocked(msg)
			}

			msg.VisibleAt = time.Time{}
			msg.ReceiptHandle = ""

			logDebug("Nacked message '%s' in queue '%s', will retry", msg.ID, q.name)

			return nil
		}
	}

	return ErrInvalidReceiptHandle
}

func (q *Queue) moveToDeadLetterQueueLocked(msg *Message) error {
	if q.deadLetterQueue == nil {
		q.stats.TotalFailed++
		for i, m := range q.messages {
			if m.ID == msg.ID {
				q.messages = append(q.messages[:i], q.messages[i+1:]...)
				q.stats.CurrentSize = len(q.messages)
				break
			}
		}
		logError("Message '%s' exceeded max retries, no DLQ configured, discarding", msg.ID)
		return nil
	}

	dlqMsg := msg.Clone()
	dlqMsg.SetMetadata("original_queue", q.name)
	dlqMsg.SetMetadata("failure_reason", "max_retries_exceeded")
	dlqMsg.ReceiptHandle = ""
	dlqMsg.VisibleAt = time.Time{}

	for i, m := range q.messages {
		if m.ID == msg.ID {
			q.messages = append(q.messages[:i], q.messages[i+1:]...)
			q.stats.CurrentSize = len(q.messages)
			break
		}
	}

	q.stats.TotalFailed++

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		q.deadLetterQueue.Enqueue(ctx, dlqMsg)
	}()

	logInfo("Message '%s' moved to DLQ '%s' after %d retries",
		msg.ID, q.deadLetterQueue.name, msg.RetryCount)

	return nil
}

func (q *Queue) Stats() QueueStats {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.stats.CurrentSize = len(q.messages)
	return q.stats
}

func (q *Queue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.messages)
}

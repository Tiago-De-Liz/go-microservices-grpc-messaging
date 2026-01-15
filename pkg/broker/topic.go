package broker

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Topic struct {
	mu          sync.RWMutex
	name        string
	subscribers []*Queue
}

func (t *Topic) Name() string {
	return t.name
}

func (t *Topic) addSubscriber(queue *Queue) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.subscribers = append(t.subscribers, queue)
}

func (t *Topic) Publish(ctx context.Context, msg *Message) error {
	t.mu.RLock()
	subscribers := make([]*Queue, len(t.subscribers))
	copy(subscribers, t.subscribers)
	t.mu.RUnlock()

	if msg.Timestamp.IsZero() {
		msg.Timestamp = time.Now()
	}

	for _, queue := range subscribers {
		clone := msg.Clone()
		clone.SetMetadata("source_topic", t.name)
		clone.SetMetadata("delivery_id", uuid.New().String())

		if err := queue.Enqueue(ctx, clone); err != nil {
			logError("Failed to deliver message to queue '%s': %v", queue.name, err)
		}
	}

	return nil
}

func (t *Topic) SubscriberCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return len(t.subscribers)
}

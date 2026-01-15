package broker

import (
	"context"
	"sync"
	"time"
)

type BrokerConfig struct {
	DefaultVisibilityTimeout time.Duration
	DefaultMaxRetries        int
	EnableLogging            bool
}

func DefaultBrokerConfig() BrokerConfig {
	return BrokerConfig{
		DefaultVisibilityTimeout: 30 * time.Second,
		DefaultMaxRetries:        3,
		EnableLogging:            true,
	}
}

type Broker struct {
	mu     sync.RWMutex
	topics map[string]*Topic
	queues map[string]*Queue
	config BrokerConfig
}

func NewBroker(config BrokerConfig) *Broker {
	return &Broker{
		topics: make(map[string]*Topic),
		queues: make(map[string]*Queue),
		config: config,
	}
}

func (b *Broker) CreateTopic(name string) *Topic {
	b.mu.Lock()
	defer b.mu.Unlock()

	if existing, ok := b.topics[name]; ok {
		return existing
	}

	topic := &Topic{
		name:        name,
		subscribers: make([]*Queue, 0),
	}
	b.topics[name] = topic

	if b.config.EnableLogging {
		logInfo("Created topic: %s", name)
	}

	return topic
}

func (b *Broker) GetTopic(name string) (*Topic, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	topic, ok := b.topics[name]
	return topic, ok
}

type QueueOption func(*Queue)

func WithVisibilityTimeout(d time.Duration) QueueOption {
	return func(q *Queue) {
		q.visibilityTimeout = d
	}
}

func WithMaxRetries(n int) QueueOption {
	return func(q *Queue) {
		q.maxRetries = n
	}
}

func WithDLQ(dlq *Queue) QueueOption {
	return func(q *Queue) {
		q.deadLetterQueue = dlq
	}
}

func (b *Broker) CreateQueue(name string, opts ...QueueOption) *Queue {
	b.mu.Lock()
	defer b.mu.Unlock()

	if existing, ok := b.queues[name]; ok {
		return existing
	}

	queue := &Queue{
		name:              name,
		messages:          make([]*Message, 0),
		visibilityTimeout: b.config.DefaultVisibilityTimeout,
		maxRetries:        b.config.DefaultMaxRetries,
	}

	for _, opt := range opts {
		opt(queue)
	}

	b.queues[name] = queue

	if b.config.EnableLogging {
		logInfo("Created queue: %s", name)
	}

	return queue
}

func (b *Broker) GetQueue(name string) (*Queue, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	queue, ok := b.queues[name]
	return queue, ok
}

func (b *Broker) Subscribe(topicName, queueName string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	topic, ok := b.topics[topicName]
	if !ok {
		return ErrTopicNotFound
	}

	queue, ok := b.queues[queueName]
	if !ok {
		return ErrQueueNotFound
	}

	topic.addSubscriber(queue)

	if b.config.EnableLogging {
		logInfo("Subscribed queue '%s' to topic '%s'", queueName, topicName)
	}

	return nil
}

func (b *Broker) Publish(ctx context.Context, topicName string, msg *Message) error {
	b.mu.RLock()
	topic, ok := b.topics[topicName]
	b.mu.RUnlock()

	if !ok {
		return ErrTopicNotFound
	}

	return topic.Publish(ctx, msg)
}

func (b *Broker) Stats() BrokerStats {
	b.mu.RLock()
	defer b.mu.RUnlock()

	stats := BrokerStats{
		TopicCount: len(b.topics),
		QueueCount: len(b.queues),
		Queues:     make(map[string]QueueStats),
	}

	for name, queue := range b.queues {
		stats.Queues[name] = queue.Stats()
	}

	return stats
}

type BrokerStats struct {
	TopicCount int
	QueueCount int
	Queues     map[string]QueueStats
}

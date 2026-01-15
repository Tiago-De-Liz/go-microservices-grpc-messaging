package broker

import (
	"context"
	"sync"
	"time"
)

type MessageHandler func(*Message) error

type WorkerConfig struct {
	PollInterval time.Duration
	Concurrency  int
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		PollInterval: 100 * time.Millisecond,
		Concurrency:  1,
	}
}

type Worker struct {
	name    string
	queue   *Queue
	handler MessageHandler
	config  WorkerConfig
	stats   WorkerStats
	mu      sync.Mutex
	running bool
	stopCh  chan struct{}
}

type WorkerStats struct {
	MessagesProcessed int64
	MessagesFailed    int64
	TotalProcessTime  time.Duration
}

func NewWorker(name string, queue *Queue, handler MessageHandler) *Worker {
	return &Worker{
		name:    name,
		queue:   queue,
		handler: handler,
		config:  DefaultWorkerConfig(),
		stopCh:  make(chan struct{}),
	}
}

func NewWorkerWithConfig(name string, queue *Queue, handler MessageHandler, config WorkerConfig) *Worker {
	return &Worker{
		name:    name,
		queue:   queue,
		handler: handler,
		config:  config,
		stopCh:  make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.mu.Unlock()

	logInfo("Worker '%s' started, polling queue '%s'", w.name, w.queue.name)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-w.stopCh:
			return nil
		default:
		}

		msg, err := w.queue.Receive(ctx)
		if err != nil {
			logError("Worker '%s' failed to receive message: %v", w.name, err)
			time.Sleep(w.config.PollInterval)
			continue
		}

		if msg == nil {
			time.Sleep(w.config.PollInterval)
			continue
		}

		w.processMessage(ctx, msg)
	}
}

func (w *Worker) processMessage(ctx context.Context, msg *Message) {
	start := time.Now()

	err := w.handler(msg)

	elapsed := time.Since(start)

	if err != nil {
		w.mu.Lock()
		w.stats.MessagesFailed++
		w.mu.Unlock()

		logError("Worker '%s' failed to process message '%s': %v", w.name, msg.ID, err)

		if nackErr := w.queue.Nack(ctx, msg.ReceiptHandle); nackErr != nil {
			logError("Worker '%s' failed to nack message '%s': %v", w.name, msg.ID, nackErr)
		}
		return
	}

	if ackErr := w.queue.Acknowledge(ctx, msg.ReceiptHandle); ackErr != nil {
		logError("Worker '%s' failed to ack message '%s': %v", w.name, msg.ID, ackErr)
		return
	}

	w.mu.Lock()
	w.stats.MessagesProcessed++
	w.stats.TotalProcessTime += elapsed
	w.mu.Unlock()
}

func (w *Worker) Stop() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.running {
		close(w.stopCh)
		w.running = false
		logInfo("Worker '%s' stopped", w.name)
	}
}

func (w *Worker) Stats() WorkerStats {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.stats
}

type IdempotencyStore interface {
	IsProcessed(messageID string) bool
	MarkProcessed(messageID string) error
}

type InMemoryIdempotencyStore struct {
	mu        sync.RWMutex
	processed map[string]time.Time
	ttl       time.Duration
}

func NewInMemoryIdempotencyStore(ttl time.Duration) *InMemoryIdempotencyStore {
	return &InMemoryIdempotencyStore{
		processed: make(map[string]time.Time),
		ttl:       ttl,
	}
}

func (s *InMemoryIdempotencyStore) IsProcessed(messageID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	timestamp, ok := s.processed[messageID]
	if !ok {
		return false
	}

	if time.Since(timestamp) > s.ttl {
		return false
	}

	return true
}

func (s *InMemoryIdempotencyStore) MarkProcessed(messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.processed[messageID] = time.Now()
	return nil
}

func IdempotentWorker(name string, queue *Queue, handler MessageHandler, store IdempotencyStore) *Worker {
	wrappedHandler := func(msg *Message) error {
		if store.IsProcessed(msg.ID) {
			logInfo("Message '%s' already processed, skipping", msg.ID)
			return nil
		}

		if err := handler(msg); err != nil {
			return err
		}

		return store.MarkProcessed(msg.ID)
	}

	return NewWorker(name, queue, wrappedHandler)
}

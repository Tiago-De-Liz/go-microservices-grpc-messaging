package broker

import (
	"errors"
	"log"
	"time"
)

var (
	ErrTopicNotFound        = errors.New("topic not found")
	ErrQueueNotFound        = errors.New("queue not found")
	ErrMessageNotFound      = errors.New("message not found")
	ErrInvalidReceiptHandle = errors.New("invalid or expired receipt handle")
	ErrQueueEmpty           = errors.New("queue is empty")
)

var loggingEnabled = true

func SetLogging(enabled bool) {
	loggingEnabled = enabled
}

func logInfo(format string, args ...interface{}) {
	if loggingEnabled {
		log.Printf("[BROKER] "+format, args...)
	}
}

func logError(format string, args ...interface{}) {
	if loggingEnabled {
		log.Printf("[BROKER] ERROR: "+format, args...)
	}
}

func logDebug(format string, args ...interface{}) {
	// Debug logging disabled by default
}

type RetryConfig struct {
	MaxRetries     int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	BackoffFactor  float64
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     30 * time.Second,
		BackoffFactor:  2.0,
	}
}

func (c RetryConfig) BackoffDuration(attempt int) time.Duration {
	if attempt <= 0 {
		return c.InitialBackoff
	}

	backoff := c.InitialBackoff
	for i := 0; i < attempt; i++ {
		backoff = time.Duration(float64(backoff) * c.BackoffFactor)
		if backoff > c.MaxBackoff {
			return c.MaxBackoff
		}
	}

	return backoff
}

package broker

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

type Message struct {
	ID            string            `json:"id"`
	Type          string            `json:"type"`
	Payload       json.RawMessage   `json:"payload"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	Timestamp     time.Time         `json:"timestamp"`
	RetryCount    int               `json:"retry_count"`
	VisibleAt     time.Time         `json:"-"`
	ReceiptHandle string            `json:"-"`
}

func NewMessage(messageType string, payload interface{}) (*Message, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:        uuid.New().String(),
		Type:      messageType,
		Payload:   payloadBytes,
		Metadata:  make(map[string]string),
		Timestamp: time.Now(),
	}, nil
}

func (m *Message) Decode(v interface{}) error {
	return json.Unmarshal(m.Payload, v)
}

func (m *Message) SetMetadata(key, value string) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]string)
	}
	m.Metadata[key] = value
}

func (m *Message) GetMetadata(key string) string {
	if m.Metadata == nil {
		return ""
	}
	return m.Metadata[key]
}

func (m *Message) Clone() *Message {
	clone := &Message{
		ID:         uuid.New().String(),
		Type:       m.Type,
		Payload:    make(json.RawMessage, len(m.Payload)),
		Timestamp:  m.Timestamp,
		RetryCount: 0,
	}

	copy(clone.Payload, m.Payload)

	if m.Metadata != nil {
		clone.Metadata = make(map[string]string)
		for k, v := range m.Metadata {
			clone.Metadata[k] = v
		}
	}

	return clone
}

func (m *Message) IsVisible() bool {
	return m.VisibleAt.IsZero() || time.Now().After(m.VisibleAt)
}

func (m *Message) Age() time.Duration {
	return time.Since(m.Timestamp)
}

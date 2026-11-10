// Package session holds the append-only event log that every model-visible
// fact is derived from, mirroring packages/core/session.
package session

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Event is one durable fact appended to the log.
type Event struct {
	Type string          `json:"type"`
	Seq  int             `json:"seq"`
	Time int64           `json:"time"`
	Data json.RawMessage `json:"data,omitempty"`
}

// Log is an in-memory append-only session log.
type Log struct {
	ID     string
	mu     sync.RWMutex
	events []Event
	next   int
}

// NewLog creates an empty log for the given session id.
func NewLog(id string) *Log { return &Log{ID: id} }

// Append records one event and returns its sequence number.
func (l *Log) Append(eventType string, data any) (Event, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("session: encode %s: %w", eventType, err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	event := Event{Type: eventType, Seq: l.next, Time: time.Now().UnixMilli(), Data: encoded}
	l.next++
	l.events = append(l.events, event)
	return event, nil
}

// Events returns a copy of the log.
func (l *Log) Events() []Event {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return append([]Event(nil), l.events...)
}

// Len reports the number of events.
func (l *Log) Len() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.events)
}

// Message is one model-visible message derived from the log.
type Message struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// DeriveMessages projects model history from the durable log: user input and
// assistant text only, in log order. It is the Go counterpart of
// deriveMessages() in the TypeScript core.
func (l *Log) DeriveMessages() []Message {
	messages := make([]Message, 0, 8)
	for _, event := range l.Events() {
		switch event.Type {
		case "user/message":
			var payload struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(event.Data, &payload) == nil && payload.Text != "" {
				messages = append(messages, Message{Role: "user", Text: payload.Text})
			}
		case "assistant/message":
			var payload struct {
				Text string `json:"text"`
			}
			if json.Unmarshal(event.Data, &payload) == nil && payload.Text != "" {
				messages = append(messages, Message{Role: "assistant", Text: payload.Text})
			}
		}
	}
	return messages
}

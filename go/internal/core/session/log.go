// Package session holds the append-only event log that every model-visible
// fact is derived from, mirroring packages/core/session.
package session

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Header is the first JSONL line of a session artifact.
type Header struct {
	Version         int    `json:"version"`
	ID              string `json:"id"`
	CreatedAt       int64  `json:"createdAt"`
	Cwd             string `json:"cwd,omitempty"`
	ParentSession   string `json:"parentSession,omitempty"`
	IsSeeded        bool   `json:"isSeeded"`
	DelegationDepth int    `json:"delegationDepth"`
}

// SurfaceOp is `"append"` or a positional replacement range.
type SurfaceOp struct {
	Append   bool
	StartSeq int
	EndSeq   int
}

// Event is one durable fact. Seq is assigned from log position; Time defaults
// to the append time but is not part of the persisted row.
type Event struct {
	Type            string          `json:"type"`
	Seq             int             `json:"-"`
	Time            int64           `json:"-"`
	Data            json.RawMessage `json:"data"`
	SurfaceOp       *SurfaceOp      `json:"-"`
	SourceEventSeqs []int           `json:"-"`
	Ignorable       bool            `json:"-"`
}

// Message is one derived model-visible message, kept as its original JSON
// object so provider content survives without a second schema.
type Message struct {
	Raw map[string]any
}

// Role reports the message role.
func (m Message) Role() string {
	role, _ := m.Raw["role"].(string)
	return role
}

// Content returns the message content blocks.
func (m Message) Content() []any {
	blocks, _ := m.Raw["content"].([]any)
	return blocks
}

// MarshalJSON emits the original object.
func (m Message) MarshalJSON() ([]byte, error) { return json.Marshal(m.Raw) }

// Log is an in-memory session log.
type Log struct {
	Header Header
	mu     sync.RWMutex
	events []Event
}

// NewLog creates an empty log for the given session id.
func NewLog(id string) *Log {
	return &Log{
		Header: Header{Version: CurrentVersion, ID: id, CreatedAt: time.Now().UnixMilli()},
	}
}

// CurrentVersion is the session format this package writes.
const CurrentVersion = 4

// Append records one event and returns its sequence number. surfaceOp and
// sourceEventSeqs are optional; message-producing events carry an append or
// replace marker exactly like the TypeScript session.
func (l *Log) Append(eventType string, data any, options ...AppendOption) (Event, error) {
	encoded, err := json.Marshal(data)
	if err != nil {
		return Event{}, fmt.Errorf("session: encode %s: %w", eventType, err)
	}
	var settings appendSettings
	for _, option := range options {
		option(&settings)
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	event := Event{
		Type:            eventType,
		Seq:             len(l.events),
		Time:            time.Now().UnixMilli(),
		Data:            encoded,
		SurfaceOp:       settings.surfaceOp,
		SourceEventSeqs: settings.sourceEventSeqs,
		Ignorable:       settings.ignorable,
	}
	l.events = append(l.events, event)
	return event, nil
}

// appendSettings collects the optional markers.
type appendSettings struct {
	surfaceOp       *SurfaceOp
	sourceEventSeqs []int
	ignorable       bool
}

// AppendOption configures one appended event.
type AppendOption func(*appendSettings)

// WithAppend marks the event as appending to the model-visible surface.
func WithAppend() AppendOption {
	return func(settings *appendSettings) {
		settings.surfaceOp = &SurfaceOp{Append: true}
	}
}

// WithReplace marks the event as replacing the surface range [start, end].
func WithReplace(start, end int, sourceEventSeqs []int) AppendOption {
	return func(settings *appendSettings) {
		settings.surfaceOp = &SurfaceOp{StartSeq: start, EndSeq: end}
		settings.sourceEventSeqs = sourceEventSeqs
	}
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

// SurfaceNodes folds the surface operations and returns the message-producing
// event sequences in model-visible order.
func (l *Log) SurfaceNodes() ([]int, error) {
	return foldSurface(l.Events())
}

func foldSurface(events []Event) ([]int, error) {
	nodes := make([]int, 0, len(events))
	for _, event := range events {
		if event.SurfaceOp == nil {
			if isSurfaceEligible(event.Type) {
				return nil, fmt.Errorf("session: %s at seq %d requires a surfaceOp", event.Type, event.Seq)
			}
			continue
		}
		if event.SurfaceOp.Append {
			nodes = append(nodes, event.Seq)
			continue
		}
		startIndex, endIndex := -1, -1
		for i, seq := range nodes {
			if seq == event.SurfaceOp.StartSeq {
				startIndex = i
			}
			if seq == event.SurfaceOp.EndSeq {
				endIndex = i
			}
		}
		if startIndex < 0 || endIndex < 0 || startIndex > endIndex {
			return nil, fmt.Errorf("session: surface replace at seq %d targets invalid range %d..%d",
				event.Seq, event.SurfaceOp.StartSeq, event.SurfaceOp.EndSeq)
		}
		next := make([]int, 0, len(nodes)-(endIndex-startIndex+1)+1)
		next = append(next, nodes[:startIndex]...)
		next = append(next, event.Seq)
		next = append(next, nodes[endIndex+1:]...)
		nodes = next
	}
	return nodes, nil
}

func isSurfaceEligible(eventType string) bool {
	switch eventType {
	case "system/message", "developer/message", "user/message", "assistant/message", "tool/result":
		return true
	default:
		return false
	}
}

// DeriveMessages projects model history by walking the surface and rendering
// every message-producing event. Empty assistant/system/developer messages are
// skipped, matching deriveEventMessage.
func (l *Log) DeriveMessages() ([]Message, error) {
	events := l.Events()
	nodes, err := foldSurface(events)
	if err != nil {
		return nil, err
	}
	messages := make([]Message, 0, len(nodes))
	for _, seq := range nodes {
		event := events[seq]
		raw, err := deriveEventMessage(event)
		if err != nil {
			return nil, err
		}
		if raw != nil {
			messages = append(messages, Message{Raw: raw})
		}
	}
	return messages, nil
}

func deriveEventMessage(event Event) (map[string]any, error) {
	switch event.Type {
	case "user/message":
		return decodeObject(event.Data)
	case "system/message", "developer/message", "assistant/message":
		var payload struct {
			Message map[string]any `json:"message"`
		}
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			return nil, fmt.Errorf("session: decode %s at seq %d: %w", event.Type, event.Seq, err)
		}
		if payload.Message == nil {
			return nil, nil
		}
		if content, ok := payload.Message["content"].([]any); ok && len(content) == 0 {
			return nil, nil
		}
		return payload.Message, nil
	case "tool/result":
		var payload struct {
			Message map[string]any `json:"message"`
		}
		if err := json.Unmarshal(event.Data, &payload); err != nil {
			return nil, fmt.Errorf("session: decode tool/result at seq %d: %w", event.Seq, err)
		}
		return payload.Message, nil
	default:
		return nil, nil
	}
}

func decodeObject(data json.RawMessage) (map[string]any, error) {
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("session: decode message: %w", err)
	}
	return value, nil
}

// ReadFile loads a JSONL session artifact: a header line followed by event
// rows. Seq is taken from row order, matching the TypeScript loader.
func ReadFile(path string) (*Log, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("session: read %s: %w", path, err)
	}
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)
	first := true
	log := &Log{}
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		if first {
			if err := json.Unmarshal(line, &log.Header); err != nil {
				return nil, fmt.Errorf("session: parse header %s: %w", path, err)
			}
			first = false
			continue
		}
		event, err := decodeRow(line, len(log.events))
		if err != nil {
			return nil, fmt.Errorf("session: %s: line %d: %w", path, len(log.events)+2, err)
		}
		log.events = append(log.events, event)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("session: read %s: %w", path, err)
	}
	if first {
		return nil, fmt.Errorf("session: %s is empty", path)
	}
	return log, nil
}

type row struct {
	Type            string          `json:"type"`
	Data            json.RawMessage `json:"data"`
	SurfaceOp       json.RawMessage `json:"surfaceOp"`
	SourceEventSeqs json.RawMessage `json:"sourceEventSeqs"`
	Ignorable       bool            `json:"ignorable"`
}

func decodeRow(line []byte, seq int) (Event, error) {
	var decoded row
	if err := json.Unmarshal(line, &decoded); err != nil {
		return Event{}, err
	}
	event := Event{Type: decoded.Type, Seq: seq, Data: decoded.Data, Ignorable: decoded.Ignorable}
	if len(decoded.SurfaceOp) > 0 && string(decoded.SurfaceOp) != "null" {
		op, err := decodeSurfaceOp(decoded.SurfaceOp)
		if err != nil {
			return Event{}, err
		}
		event.SurfaceOp = op
	}
	if len(decoded.SourceEventSeqs) > 0 && string(decoded.SourceEventSeqs) != "null" {
		seqs, err := decodeSeqRanges(decoded.SourceEventSeqs)
		if err != nil {
			return Event{}, err
		}
		event.SourceEventSeqs = seqs
	}
	return event, nil
}

func decodeSurfaceOp(raw json.RawMessage) (*SurfaceOp, error) {
	var appendOp string
	if err := json.Unmarshal(raw, &appendOp); err == nil {
		if appendOp != "append" {
			return nil, fmt.Errorf("session: unknown surfaceOp %q", appendOp)
		}
		return &SurfaceOp{Append: true}, nil
	}
	var replaceOp struct {
		Op       string `json:"op"`
		StartSeq int    `json:"startSeq"`
		EndSeq   int    `json:"endSeq"`
	}
	if err := json.Unmarshal(raw, &replaceOp); err != nil {
		return nil, fmt.Errorf("session: invalid surfaceOp: %w", err)
	}
	if replaceOp.Op != "replace" {
		return nil, fmt.Errorf("session: unknown surfaceOp %q", replaceOp.Op)
	}
	return &SurfaceOp{StartSeq: replaceOp.StartSeq, EndSeq: replaceOp.EndSeq}, nil
}

// decodeSeqRanges accepts flat seqs or [start, end] range pairs.
func decodeSeqRanges(raw json.RawMessage) ([]int, error) {
	var value []json.RawMessage
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("session: sourceEventSeqs must be an array: %w", err)
	}
	seqs := make([]int, 0, len(value))
	for _, entry := range value {
		var single int
		if err := json.Unmarshal(entry, &single); err == nil {
			seqs = append(seqs, single)
			continue
		}
		var pair []int
		if err := json.Unmarshal(entry, &pair); err != nil || len(pair) != 2 {
			return nil, fmt.Errorf("session: sourceEventSeqs range must be a [start, end] pair")
		}
		for current := pair[0]; current <= pair[1]; current++ {
			seqs = append(seqs, current)
		}
	}
	return seqs, nil
}

// WriteFile writes the artifact as a header line plus one JSON row per event,
// omitting seq/time exactly like the TypeScript writer.
func (l *Log) WriteFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("session: create %s: %w", path, err)
	}
	defer file.Close()
	return l.Write(file)
}

// Write emits the JSONL artifact.
func (l *Log) Write(writer io.Writer) error {
	header := l.Header
	if header.Version == 0 {
		header.Version = CurrentVersion
	}
	row := headerRow{
		Type:            "session",
		Version:         header.Version,
		ID:              header.ID,
		CreatedAt:       header.CreatedAt,
		IsSeeded:        header.IsSeeded,
		DelegationDepth: header.DelegationDepth,
	}
	if header.Cwd != "" {
		cwd := header.Cwd
		row.Cwd = &cwd
	}
	if header.ParentSession != "" {
		parent := header.ParentSession
		row.ParentSession = &parent
	}
	if err := writeJSONLine(writer, row); err != nil {
		return err
	}
	for _, event := range l.Events() {
		line := eventRow{Type: event.Type, Data: event.Data, Ignorable: event.Ignorable}
		if event.SurfaceOp != nil {
			if event.SurfaceOp.Append {
				line.SurfaceOp = json.RawMessage(`"append"`)
			} else {
				replace, err := json.Marshal(replaceOp{
					Op:       "replace",
					StartSeq: event.SurfaceOp.StartSeq,
					EndSeq:   event.SurfaceOp.EndSeq,
				})
				if err != nil {
					return fmt.Errorf("session: encode surfaceOp: %w", err)
				}
				line.SurfaceOp = replace
				line.SourceEventSeqs = event.SourceEventSeqs
			}
		}
		if err := writeJSONLine(writer, line); err != nil {
			return err
		}
	}
	return nil
}

type headerRow struct {
	Type            string  `json:"type"`
	Version         int     `json:"version"`
	ID              string  `json:"id"`
	CreatedAt       int64   `json:"createdAt"`
	Cwd             *string `json:"cwd,omitempty"`
	ParentSession   *string `json:"parentSession,omitempty"`
	IsSeeded        bool    `json:"isSeeded"`
	DelegationDepth int     `json:"delegationDepth"`
}

type eventRow struct {
	Type            string          `json:"type"`
	Data            json.RawMessage `json:"data"`
	SurfaceOp       json.RawMessage `json:"surfaceOp,omitempty"`
	SourceEventSeqs []int           `json:"sourceEventSeqs,omitempty"`
	Ignorable       bool            `json:"ignorable,omitempty"`
}

type replaceOp struct {
	Op       string `json:"op"`
	StartSeq int    `json:"startSeq"`
	EndSeq   int    `json:"endSeq"`
}

func writeJSONLine(writer io.Writer, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("session: encode row: %w", err)
	}
	if _, err := writer.Write(append(encoded, '\n')); err != nil {
		return fmt.Errorf("session: write row: %w", err)
	}
	return nil
}

// Fork copies the prefix through boundary (inclusive) into a new log with the
// given id and marks the parent, matching Session.fork's durable prefix copy.
func (l *Log) Fork(id string, boundary int) (*Log, error) {
	events := l.Events()
	if boundary < 0 || boundary >= len(events) {
		return nil, fmt.Errorf("session: fork boundary %d outside 0..%d", boundary, len(events)-1)
	}
	child := NewLog(id)
	child.Header.Cwd = l.Header.Cwd
	child.Header.ParentSession = l.Header.ID
	child.Header.DelegationDepth = l.Header.DelegationDepth
	child.events = append([]Event(nil), events[:boundary+1]...)
	for i := range child.events {
		child.events[i].Seq = i
	}
	return child, nil
}

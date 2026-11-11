// Package agent owns the turn/step state machine, the Go counterpart of
// packages/core/agent-loop.
package agent

import (
	"context"
	"fmt"
	"time"

	"git.glkj.example.com/glkj/harness/go/internal/core/session"
	"git.glkj.example.com/glkj/harness/go/internal/llm"
	"git.glkj.example.com/glkj/harness/go/internal/plugin"
)

// Agent drives one session: it claims input, assembles the request, streams the
// model answer, and records every durable fact in the session log.
type Agent struct {
	ctx        *plugin.Context
	log        *session.Log
	provider   llm.Provider
	providerID string
	model      string
	messageSeq int
}

// Options configures an Agent.
type Options struct {
	Provider   llm.Provider
	ProviderID string
	Model      string
}

// New creates an agent bound to a plugin context and session log.
func New(ctx *plugin.Context, log *session.Log, options Options) *Agent {
	providerID := options.ProviderID
	if providerID == "" {
		providerID = "deepseek-official"
	}
	return &Agent{ctx: ctx, log: log, provider: options.Provider, providerID: providerID, model: options.Model}
}

// Log exposes the session log.
func (a *Agent) Log() *session.Log { return a.log }

// Run executes one turn: one or more steps until the model stops owing output.
func (a *Agent) Run(ctx context.Context, input string) error {
	if _, err := a.log.Append("turn/start", map[string]any{"turn": 1, "time": time.Now().UnixMilli()}); err != nil {
		return err
	}
	a.ctx.Emit(ctx, "turn/start", a.log.Header.ID)

	if _, err := a.log.Append("step/start", map[string]any{"turn": 1, "step": 1}); err != nil {
		return err
	}
	a.ctx.Emit(ctx, "step/start", a.log.Header.ID)

	userMessage := map[string]any{
		"role":    "user",
		"content": []any{map[string]any{"type": "text", "text": input}},
		"source":  map[string]any{"kind": "user"},
		"id":      a.nextMessageID(),
	}
	if _, err := a.log.Append("user/message", userMessage, session.WithAppend()); err != nil {
		return err
	}

	derived, err := a.log.DeriveMessages()
	if err != nil {
		return err
	}
	payload := PreStepPayload{Messages: derived}
	if _, err := a.ctx.Waterfall(ctx, "agent/pre-step", payload, nil); err != nil {
		_, appendErr := a.log.Append("turn/end", map[string]any{"turn": 1, "reason": err.Error()})
		if appendErr != nil {
			return appendErr
		}
		return fmt.Errorf("agent: pre-step rejected: %w", err)
	}

	response, stream, err := a.request(ctx)
	if err != nil {
		if _, appendErr := a.log.Append("turn/end", map[string]any{"turn": 1, "reason": err.Error()}); appendErr != nil {
			return appendErr
		}
		return err
	}
	assistantMessage := map[string]any{
		"turn": 1,
		"step": 1,
		"message": map[string]any{
			"role":    "assistant",
			"content": []any{map[string]any{"type": "text", "text": response}},
			"source":  map[string]any{"kind": "model", "provider": a.providerID, "model": a.model},
			"id":      a.nextMessageID(),
		},
		"usage":  map[string]any{},
		"stream": stream,
	}
	if _, err := a.log.Append("assistant/message", assistantMessage, session.WithAppend()); err != nil {
		return err
	}

	if _, err := a.log.Append("step/end", map[string]any{"turn": 1, "step": 1}); err != nil {
		return err
	}
	if _, err := a.log.Append("turn/end", map[string]any{"turn": 1, "reason": "completed"}); err != nil {
		return err
	}
	a.ctx.Emit(ctx, "turn/end", a.log.Header.ID)
	return nil
}

func (a *Agent) nextMessageID() string {
	a.messageSeq++
	return fmt.Sprintf("message-%d", a.messageSeq)
}

// PreStepPayload is the waterfall payload that decides what the model sees.
type PreStepPayload struct {
	Messages []session.Message
}

func (a *Agent) request(ctx context.Context) (string, []any, error) {
	if a.provider == nil {
		return "", nil, fmt.Errorf("agent: no model provider configured")
	}
	derived, err := a.log.DeriveMessages()
	if err != nil {
		return "", nil, err
	}
	request := llm.Request{Model: a.model}
	for _, message := range derived {
		request.Messages = append(request.Messages, llm.Message{Role: message.Role(), Content: textOf(message)})
	}

	var builder []byte
	var stream []any
	index := 0
	err = a.provider.Stream(ctx, request, func(chunk llm.Chunk) error {
		stream = append(stream, map[string]any{
			"type": "chunk",
			"time": time.Now().UnixMilli(),
			"chunk": map[string]any{
				"type":  "text-delta",
				"index": index,
				"text":  chunk.Text,
			},
		})
		if chunk.Text == "" {
			return nil
		}
		builder = append(builder, chunk.Text...)
		index++
		if _, err := a.log.Append("assistant/chunk", map[string]any{"text": chunk.Text}); err != nil {
			return err
		}
		a.ctx.Emit(ctx, "assistant/chunk", chunk.Text)
		return nil
	})
	if err != nil {
		return "", nil, fmt.Errorf("agent: request failed: %w", err)
	}
	if len(stream) == 0 {
		stream = append(stream, map[string]any{
			"type":  "chunk",
			"time":  time.Now().UnixMilli(),
			"chunk": map[string]any{"type": "finish", "reason": map[string]any{"kind": "stop"}},
		})
	}
	return string(builder), stream, nil
}

// textOf joins the text blocks of one derived message for the provider request.
func textOf(message session.Message) string {
	var builder []byte
	for _, block := range message.Content() {
		if object, ok := block.(map[string]any); ok && object["type"] == "text" {
			if text, ok := object["text"].(string); ok {
				builder = append(builder, text...)
			}
		}
	}
	return string(builder)
}

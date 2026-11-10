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
	ctx      *plugin.Context
	log      *session.Log
	provider llm.Provider
	model    string
}

// Options configures an Agent.
type Options struct {
	Provider llm.Provider
	Model    string
}

// New creates an agent bound to a plugin context and session log.
func New(ctx *plugin.Context, log *session.Log, options Options) *Agent {
	return &Agent{ctx: ctx, log: log, provider: options.Provider, model: options.Model}
}

// Log exposes the session log.
func (a *Agent) Log() *session.Log { return a.log }

// Run executes one turn: one or more steps until the model stops owing output.
func (a *Agent) Run(ctx context.Context, input string) error {
	if _, err := a.log.Append("turn/start", map[string]any{"turn": 1, "time": time.Now().UnixMilli()}); err != nil {
		return err
	}
	a.ctx.Emit(ctx, "turn/start", a.log.ID)

	if _, err := a.log.Append("step/start", map[string]any{"turn": 1, "step": 1}); err != nil {
		return err
	}
	a.ctx.Emit(ctx, "step/start", a.log.ID)

	if _, err := a.log.Append("user/message", map[string]any{"text": input}); err != nil {
		return err
	}

	payload := PreStepPayload{Messages: a.log.DeriveMessages()}
	if _, err := a.ctx.Waterfall(ctx, "agent/pre-step", payload, nil); err != nil {
		_, appendErr := a.log.Append("turn/end", map[string]any{"turn": 1, "reason": err.Error()})
		if appendErr != nil {
			return appendErr
		}
		return fmt.Errorf("agent: pre-step rejected: %w", err)
	}

	response, err := a.request(ctx)
	if err != nil {
		if _, appendErr := a.log.Append("turn/end", map[string]any{"turn": 1, "reason": err.Error()}); appendErr != nil {
			return appendErr
		}
		return err
	}
	if _, err := a.log.Append("assistant/message", map[string]any{"text": response}); err != nil {
		return err
	}

	if _, err := a.log.Append("step/end", map[string]any{"turn": 1, "step": 1}); err != nil {
		return err
	}
	if _, err := a.log.Append("turn/end", map[string]any{"turn": 1, "reason": "completed"}); err != nil {
		return err
	}
	a.ctx.Emit(ctx, "turn/end", a.log.ID)
	return nil
}

// PreStepPayload is the waterfall payload that decides what the model sees.
type PreStepPayload struct {
	Messages []session.Message
}

func (a *Agent) request(ctx context.Context) (string, error) {
	if a.provider == nil {
		return "", fmt.Errorf("agent: no model provider configured")
	}
	request := llm.Request{Model: a.model}
	for _, message := range a.log.DeriveMessages() {
		request.Messages = append(request.Messages, llm.Message{Role: message.Role, Content: message.Text})
	}

	var builder []byte
	err := a.provider.Stream(ctx, request, func(chunk llm.Chunk) error {
		if chunk.Text == "" {
			return nil
		}
		builder = append(builder, chunk.Text...)
		if _, err := a.log.Append("assistant/chunk", map[string]any{"text": chunk.Text}); err != nil {
			return err
		}
		a.ctx.Emit(ctx, "assistant/chunk", chunk.Text)
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("agent: request failed: %w", err)
	}
	return string(builder), nil
}

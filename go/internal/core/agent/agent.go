// Package agent owns the turn/step state machine and the tool pipeline, the Go
// counterpart of packages/core/agent-loop.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"git.glkj.example.com/glkj/harness/go/internal/core/session"
	"git.glkj.example.com/glkj/harness/go/internal/core/tools"
	"git.glkj.example.com/glkj/harness/go/internal/llm"
	"git.glkj.example.com/glkj/harness/go/internal/plugin"
)

// Agent drives one session: it claims input, assembles requests, streams model
// answers, runs requested tools, and records every durable fact.
type Agent struct {
	ctx        *plugin.Context
	log        *session.Log
	provider   llm.Provider
	providerID string
	model      string
	tools      *tools.Registry
	maxSteps   int
	messageSeq int
	callSeq    int
}

// Options configures an Agent.
type Options struct {
	Provider   llm.Provider
	ProviderID string
	Model      string
	Tools      *tools.Registry
	MaxSteps   int
}

// Invocation is one tool call as it moves through the execution pipeline.
type Invocation struct {
	Call     llm.ToolCall
	Turn     int
	Step     int
	Output   string
	Error    error
	Rejected bool
}

// New creates an agent bound to a plugin context and session log.
func New(ctx *plugin.Context, log *session.Log, options Options) *Agent {
	providerID := options.ProviderID
	if providerID == "" {
		providerID = "deepseek-official"
	}
	maxSteps := options.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 50
	}
	return &Agent{
		ctx:        ctx,
		log:        log,
		provider:   options.Provider,
		providerID: providerID,
		model:      options.Model,
		tools:      options.Tools,
		maxSteps:   maxSteps,
	}
}

// Log exposes the session log.
func (a *Agent) Log() *session.Log { return a.log }

// Run executes one turn: one or more steps until the model asks for no tool.
func (a *Agent) Run(ctx context.Context, input string) error {
	const turn = 1
	if _, err := a.log.Append("turn/start", map[string]any{"turn": turn}); err != nil {
		return err
	}
	a.ctx.Emit(ctx, "turn/start", a.log.Header.ID)

	userMessage := map[string]any{
		"role":    "user",
		"content": []any{map[string]any{"type": "text", "text": input}},
		"source":  map[string]any{"kind": "user"},
		"id":      a.nextMessageID(),
	}

	preStep := PreStepPayload{Messages: []session.Message{{Raw: userMessage}}}
	if _, err := a.ctx.Waterfall(ctx, "agent/pre-step", &preStep, nil); err != nil {
		return a.finish(ctx, turn, fmt.Errorf("agent: pre-step rejected: %w", err))
	}

	for step := 1; step <= a.maxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return a.finish(ctx, turn, err)
		}
		if _, err := a.log.Append("step/start", map[string]any{"turn": turn, "step": step}); err != nil {
			return err
		}
		a.ctx.Emit(ctx, "step/start", a.log.Header.ID)

		if step == 1 {
			if _, err := a.log.Append("user/message", userMessage, session.WithAppend()); err != nil {
				return err
			}
		}

		text, calls, stream, err := a.request(ctx, turn, step)
		if err != nil {
			return a.finish(ctx, turn, err)
		}

		if _, err := a.log.Append("assistant/message", a.assistantData(turn, step, text, calls, stream), session.WithAppend()); err != nil {
			return err
		}

		for _, call := range calls {
			if err := a.runTool(ctx, turn, step, call); err != nil {
				return err
			}
			if err := ctx.Err(); err != nil {
				return a.finish(ctx, turn, err)
			}
		}

		if _, err := a.log.Append("step/end", map[string]any{"turn": turn, "step": step}); err != nil {
			return err
		}
		if len(calls) == 0 {
			break
		}
		if step == a.maxSteps {
			return a.finish(ctx, turn, fmt.Errorf("agent: step limit %d reached with tool calls pending", a.maxSteps))
		}
	}

	if _, err := a.ctx.Waterfall(ctx, "agent/turn-stopping", turn, nil); err != nil {
		return a.finish(ctx, turn, err)
	}
	if _, err := a.log.Append("turn/end", map[string]any{"turn": turn, "reason": map[string]any{"kind": "completed"}}); err != nil {
		return err
	}
	a.ctx.Emit(ctx, "turn/end", a.log.Header.ID)
	return nil
}

func (a *Agent) finish(ctx context.Context, turn int, cause error) error {
	reason := map[string]any{"kind": "error", "message": cause.Error()}
	if ctx.Err() != nil {
		reason = map[string]any{"kind": "cancelled"}
	}
	if _, err := a.log.Append("turn/end", map[string]any{"turn": turn, "reason": reason}); err != nil {
		return err
	}
	a.ctx.Emit(ctx, "turn/end", a.log.Header.ID)
	return cause
}

// PreStepPayload is the waterfall payload that decides what the model sees.
type PreStepPayload struct {
	Messages []session.Message
}

func (a *Agent) nextMessageID() string {
	a.messageSeq++
	return fmt.Sprintf("message-%d", a.messageSeq)
}

func (a *Agent) nextCallID() string {
	a.callSeq++
	return fmt.Sprintf("call-%d", a.callSeq)
}

// assistantData is the durable settlement: the message plus the stream it came
// from, so replay reconstructs identical content.
func (a *Agent) assistantData(turn, step int, text string, calls []llm.ToolCall, stream []any) map[string]any {
	content := make([]any, 0, len(calls)+1)
	if text != "" {
		content = append(content, map[string]any{"type": "text", "text": text})
	}
	for _, call := range calls {
		content = append(content, map[string]any{
			"type": "tool-call", "id": call.ID, "name": call.Name, "arguments": call.Arguments,
		})
	}
	return map[string]any{
		"turn": turn,
		"step": step,
		"message": map[string]any{
			"role":    "assistant",
			"content": content,
			"source":  map[string]any{"kind": "model", "provider": a.providerID, "model": a.model},
			"id":      a.nextMessageID(),
		},
		"stream": stream,
	}
}

func (a *Agent) runTool(ctx context.Context, turn, step int, call llm.ToolCall) error {
	if _, err := a.log.Append("tool/call", map[string]any{
		"turn": turn, "step": step, "callId": call.ID, "name": call.Name, "arguments": call.Arguments,
	}); err != nil {
		return err
	}

	invocation := &Invocation{Call: call, Turn: turn, Step: step}
	if _, err := a.ctx.Waterfall(ctx, "tools/pre-execute", invocation, nil); err != nil {
		return err
	}
	if !invocation.Rejected {
		if a.tools == nil {
			invocation.Error = fmt.Errorf("agent: no tool registry configured")
		} else {
			invocation.Output, invocation.Error = a.tools.Call(ctx, call.Name, json.RawMessage(argumentJSON(call.Arguments)))
		}
	}
	if _, err := a.ctx.Waterfall(ctx, "tools/post-execute", invocation, nil); err != nil {
		return err
	}
	return a.recordToolResult(turn, step, invocation)
}

func argumentJSON(arguments string) []byte {
	if arguments == "" {
		return []byte("{}")
	}
	if !json.Valid([]byte(arguments)) {
		encoded, err := json.Marshal(arguments)
		if err != nil {
			return []byte("{}")
		}
		return encoded
	}
	return []byte(arguments)
}

func (a *Agent) recordToolResult(turn, step int, invocation *Invocation) error {
	text := invocation.Output
	message := map[string]any{
		"role":       "tool",
		"source":     map[string]any{"kind": "tool", "callId": invocation.Call.ID},
		"toolCallId": invocation.Call.ID,
		"content":    []any{map[string]any{"type": "text", "text": text}},
		"id":         a.nextMessageID(),
	}
	data := map[string]any{
		"turn":    turn,
		"step":    step,
		"message": message,
	}
	if invocation.Error != nil {
		message["isError"] = true
		message["content"] = []any{map[string]any{"type": "text", "text": invocation.Error.Error()}}
		data["error"] = map[string]any{"message": invocation.Error.Error()}
	}
	_, err := a.log.Append("tool/result", data, session.WithAppend())
	return err
}

// request assembles the prompt, streams one model answer, and returns its text,
// tool calls, and the exact stream records for the durable settlement.
func (a *Agent) request(ctx context.Context, turn, step int) (string, []llm.ToolCall, []any, error) {
	if a.provider == nil {
		return "", nil, nil, fmt.Errorf("agent: no model provider configured")
	}
	derived, err := a.log.DeriveMessages()
	if err != nil {
		return "", nil, nil, err
	}

	request := llm.Request{Model: a.model}
	for _, message := range derived {
		request.Messages = append(request.Messages, toLLMMessage(message))
	}
	if a.tools != nil {
		for _, definition := range a.tools.Schemas() {
			request.Tools = append(request.Tools, llm.ToolSchema{
				Name: definition.Name, Description: definition.Description, Parameters: definition.Parameters,
			})
		}
	}
	if _, err := a.ctx.Waterfall(ctx, "agent/request", &request, nil); err != nil {
		return "", nil, nil, err
	}

	var text []byte
	var calls []llm.ToolCall
	var stream []any
	indexes := make(map[string]int)

	err = a.provider.Stream(ctx, request, func(chunk llm.Chunk) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if chunk.Text != "" {
			index, exists := indexes["text"]
			if !exists {
				index = 0
				indexes["text"] = index
				stream = append(stream, blockStart(index, "text"))
			}
			text = append(text, chunk.Text...)
			stream = append(stream, timedChunk(map[string]any{
				"type": "text-delta", "index": index, "text": chunk.Text,
			}))
		}
		for _, call := range chunk.ToolCalls {
			if call.ID == "" {
				call.ID = a.nextCallID()
			}
			index := len(indexes) + 1
			indexes["call:"+call.ID] = index
			stream = append(stream, blockStart(index, "tool-call"))
			stream = append(stream, timedChunk(map[string]any{
				"type": "tool-call-delta", "index": index, "id": call.ID,
				"name": call.Name, "argumentsDelta": call.Arguments,
			}))
			stream = append(stream, blockEnd(index, map[string]any{
				"type": "tool-call", "id": call.ID, "name": call.Name, "arguments": call.Arguments,
			}))
			calls = append(calls, call)
		}
		return nil
	})
	if err != nil {
		return "", nil, nil, fmt.Errorf("agent: request failed: %w", err)
	}

	if len(stream) == 0 {
		stream = append(stream, timedChunk(map[string]any{
			"type": "finish", "reason": map[string]any{"kind": "stop"},
		}))
	} else {
		stream = append(stream, timedChunk(map[string]any{
			"type": "finish", "reason": map[string]any{"kind": finishKind(calls)},
		}))
	}
	return string(text), calls, stream, nil
}

func finishKind(calls []llm.ToolCall) string {
	if len(calls) > 0 {
		return "tool-calls"
	}
	return "stop"
}

func blockStart(index int, blockType string) map[string]any {
	return timedChunk(map[string]any{"type": "block-start", "index": index, "blockType": blockType})
}

func blockEnd(index int, block map[string]any) map[string]any {
	return timedChunk(map[string]any{"type": "block-end", "index": index, "block": block})
}

func timedChunk(chunk map[string]any) map[string]any {
	return map[string]any{"type": "chunk", "time": time.Now().UnixMilli(), "chunk": chunk}
}

// toLLMMessage projects one derived message onto the provider wire format.
func toLLMMessage(message session.Message) llm.Message {
	out := llm.Message{Role: message.Role()}
	var text []byte
	for _, block := range message.Content() {
		object, ok := block.(map[string]any)
		if !ok {
			continue
		}
		switch object["type"] {
		case "text":
			if value, ok := object["text"].(string); ok {
				if len(text) > 0 {
					text = append(text, '\n')
				}
				text = append(text, value...)
			}
		case "tool-call":
			id, _ := object["id"].(string)
			name, _ := object["name"].(string)
			arguments, _ := object["arguments"].(string)
			out.ToolCalls = append(out.ToolCalls, llm.ToolCall{ID: id, Name: name, Arguments: arguments})
		}
	}
	out.Content = string(text)
	if callID, ok := message.Raw["toolCallId"].(string); ok {
		out.ToolCallID = callID
	}
	return out
}

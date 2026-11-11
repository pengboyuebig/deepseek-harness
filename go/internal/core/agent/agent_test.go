package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"git.glkj.example.com/glkj/harness/go/internal/core/agent"
	"git.glkj.example.com/glkj/harness/go/internal/core/session"
	"git.glkj.example.com/glkj/harness/go/internal/core/tools"
	"git.glkj.example.com/glkj/harness/go/internal/llm"
	"git.glkj.example.com/glkj/harness/go/internal/plugin"
)

// scriptedProvider replays one scripted response per Stream call.
type scriptedProvider struct {
	responses []scriptedResponse
	requests  []llm.Request
}

type scriptedResponse struct {
	text  string
	calls []llm.ToolCall
	err   error
}

func (p *scriptedProvider) Stream(_ context.Context, request llm.Request, emit func(llm.Chunk) error) error {
	p.requests = append(p.requests, request)
	index := len(p.requests) - 1
	if index >= len(p.responses) {
		return emit(llm.Chunk{Finished: true})
	}
	response := p.responses[index]
	if response.err != nil {
		return response.err
	}
	if response.text != "" {
		if err := emit(llm.Chunk{Text: response.text}); err != nil {
			return err
		}
	}
	return emit(llm.Chunk{ToolCalls: response.calls, Finished: true})
}

func eventTypes(log *session.Log) []string {
	var types []string
	for _, event := range log.Events() {
		types = append(types, event.Type)
	}
	return types
}

func assertTypes(t *testing.T, log *session.Log, want []string) {
	t.Helper()
	got := eventTypes(log)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("events =\n  %v\nwant\n  %v", got, want)
	}
}

func TestTextTurnProducesTheDurableSequence(t *testing.T) {
	ctx := plugin.New(nil)
	defer ctx.Dispose()
	log := session.NewLog("s1")
	provider := &scriptedProvider{responses: []scriptedResponse{{text: "hello"}}}

	runner := agent.New(ctx, log, agent.Options{Provider: provider, Model: "deepseek-chat"})
	if err := runner.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertTypes(t, log, []string{
		"turn/start", "step/start", "user/message", "assistant/message", "step/end", "turn/end",
	})
	messages, err := log.DeriveMessages()
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if len(messages) != 2 || messages[1].Role() != "assistant" {
		t.Fatalf("messages = %+v", messages)
	}
	if len(provider.requests) != 1 || provider.requests[0].Messages[0].Content != "hi" {
		t.Fatalf("request = %+v", provider.requests)
	}
}

func TestToolTurnRunsThePipelineThenAnswers(t *testing.T) {
	ctx := plugin.New(nil)
	defer ctx.Dispose()
	registry := tools.NewRegistry()
	calls := 0
	if err := registry.Register(tools.Definition{
		Name:        "echo",
		Description: "echo the input",
		Parameters:  json.RawMessage(`{"type":"object"}`),
		Execute: func(_ context.Context, input json.RawMessage) (string, error) {
			calls++
			return string(input), nil
		},
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	var pre []string
	ctx.OnMiddleware("tools/pre-execute", func(ctx context.Context, payload any, next func(context.Context) (any, error)) (any, error) {
		invocation := payload.(*agent.Invocation)
		pre = append(pre, invocation.Call.Name)
		return next(ctx)
	})
	var post []string
	ctx.OnMiddleware("tools/post-execute", func(ctx context.Context, payload any, next func(context.Context) (any, error)) (any, error) {
		invocation := payload.(*agent.Invocation)
		post = append(post, invocation.Output)
		return next(ctx)
	})

	provider := &scriptedProvider{responses: []scriptedResponse{
		{calls: []llm.ToolCall{{ID: "call-1", Name: "echo", Arguments: `{"value":1}`}}},
		{text: "done"},
	}}
	log := session.NewLog("s1")
	runner := agent.New(ctx, log, agent.Options{Provider: provider, Model: "deepseek-chat", Tools: registry})
	if err := runner.Run(context.Background(), "use echo"); err != nil {
		t.Fatalf("run: %v", err)
	}

	assertTypes(t, log, []string{
		"turn/start",
		"step/start", "user/message", "assistant/message", "tool/call", "tool/result", "step/end",
		"step/start", "assistant/message", "step/end",
		"turn/end",
	})
	if calls != 1 {
		t.Fatalf("tool calls = %d, want 1", calls)
	}
	if len(pre) != 1 || pre[0] != "echo" {
		t.Fatalf("pre-execute = %v", pre)
	}
	if len(post) != 1 || post[0] != `{"value":1}` {
		t.Fatalf("post-execute = %v", post)
	}

	messages, err := log.DeriveMessages()
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if len(messages) != 4 {
		t.Fatalf("messages = %d, want 4", len(messages))
	}
	if messages[2].Role() != "tool" {
		t.Fatalf("third message role = %q", messages[2].Role())
	}
}

func TestToolFailureIsRecordedAsAnErrorResult(t *testing.T) {
	ctx := plugin.New(nil)
	defer ctx.Dispose()
	registry := tools.NewRegistry()
	registry.Register(tools.Definition{
		Name: "boom",
		Execute: func(context.Context, json.RawMessage) (string, error) {
			return "", errors.New("exploded")
		},
	})

	provider := &scriptedProvider{responses: []scriptedResponse{
		{calls: []llm.ToolCall{{ID: "call-1", Name: "boom", Arguments: "{}"}}},
		{text: "recovered"},
	}}
	log := session.NewLog("s1")
	runner := agent.New(ctx, log, agent.Options{Provider: provider, Model: "m", Tools: registry})
	if err := runner.Run(context.Background(), "go"); err != nil {
		t.Fatalf("run: %v", err)
	}

	var found bool
	for _, event := range log.Events() {
		if event.Type != "tool/result" {
			continue
		}
		var data struct {
			Message struct {
				IsError bool `json:"isError"`
				Content []struct {
					Text string `json:"text"`
				} `json:"content"`
			} `json:"message"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("decode: %v", err)
		}
		found = data.Message.IsError && data.Message.Content[0].Text == "exploded"
	}
	if !found {
		t.Fatal("tool failure was not recorded as an error result")
	}
}

func TestStepLimitStopsAnEndlessToolLoop(t *testing.T) {
	ctx := plugin.New(nil)
	defer ctx.Dispose()
	registry := tools.NewRegistry()
	registry.Register(tools.Definition{
		Name:    "again",
		Execute: func(context.Context, json.RawMessage) (string, error) { return "ok", nil },
	})

	provider := &scriptedProvider{}
	for i := 0; i < 5; i++ {
		provider.responses = append(provider.responses, scriptedResponse{
			calls: []llm.ToolCall{{ID: "call", Name: "again", Arguments: "{}"}},
		})
	}
	log := session.NewLog("s1")
	runner := agent.New(ctx, log, agent.Options{Provider: provider, Model: "m", Tools: registry, MaxSteps: 2})
	err := runner.Run(context.Background(), "loop")
	if err == nil || !strings.Contains(err.Error(), "step limit") {
		t.Fatalf("err = %v, want a step-limit error", err)
	}
	types := eventTypes(log)
	if types[len(types)-1] != "turn/end" {
		t.Fatalf("last event = %s, want turn/end", types[len(types)-1])
	}
}

func TestCancellationClosesTheTurn(t *testing.T) {
	ctx := plugin.New(nil)
	defer ctx.Dispose()
	log := session.NewLog("s1")
	provider := &scriptedProvider{responses: []scriptedResponse{{err: context.Canceled}}}

	runner := agent.New(ctx, log, agent.Options{Provider: provider, Model: "m"})
	if err := runner.Run(context.Background(), "hi"); err == nil {
		t.Fatal("expected the canceled request to fail")
	}
	var last struct {
		Reason struct {
			Kind string `json:"kind"`
		} `json:"reason"`
	}
	events := log.Events()
	if err := json.Unmarshal(events[len(events)-1].Data, &last); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if events[len(events)-1].Type != "turn/end" || last.Reason.Kind != "error" {
		t.Fatalf("last = %+v", events[len(events)-1])
	}
	if events[len(events)-2].Type != "step/end" {
		t.Fatalf("open step was not closed before turn/end: %v", eventTypes(log))
	}
}

func TestPreStepWaterfallCanRejectTheTurn(t *testing.T) {
	ctx := plugin.New(nil)
	defer ctx.Dispose()
	ctx.OnMiddleware("agent/pre-step", func(_ context.Context, _ any, _ func(context.Context) (any, error)) (any, error) {
		return nil, context.Canceled
	})

	log := session.NewLog("s1")
	runner := agent.New(ctx, log, agent.Options{Provider: &scriptedProvider{}, Model: "m"})
	if err := runner.Run(context.Background(), "hi"); err == nil {
		t.Fatal("expected the rejected pre-step to fail the turn")
	}
	events := log.Events()
	if events[len(events)-1].Type != "turn/end" {
		t.Fatalf("last event = %s, want turn/end", events[len(events)-1].Type)
	}
}

func TestMissingProviderFailsTheTurn(t *testing.T) {
	ctx := plugin.New(nil)
	defer ctx.Dispose()
	log := session.NewLog("s1")
	runner := agent.New(ctx, log, agent.Options{Model: "m"})
	if err := runner.Run(context.Background(), "hi"); err == nil {
		t.Fatal("expected a missing provider to fail")
	}
}

func TestResumedLogContinuesTurnNumbering(t *testing.T) {
	ctx := plugin.New(nil)
	defer ctx.Dispose()
	log := session.NewLog("s1")
	provider := &scriptedProvider{responses: []scriptedResponse{{text: "one"}, {text: "two"}}}

	runner := agent.New(ctx, log, agent.Options{Provider: provider, Model: "m"})
	if err := runner.Run(context.Background(), "first"); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := runner.Run(context.Background(), "second"); err != nil {
		t.Fatalf("second run: %v", err)
	}

	var turns []int
	for _, event := range log.Events() {
		if event.Type != "turn/start" {
			continue
		}
		var data struct {
			Turn int `json:"turn"`
		}
		if err := json.Unmarshal(event.Data, &data); err != nil {
			t.Fatalf("decode: %v", err)
		}
		turns = append(turns, data.Turn)
	}
	if len(turns) != 2 || turns[0] != 1 || turns[1] != 2 {
		t.Fatalf("turns = %v, want [1 2]", turns)
	}
}

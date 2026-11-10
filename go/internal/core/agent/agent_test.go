package agent_test

import (
	"context"
	"testing"

	"git.glkj.example.com/glkj/harness/go/internal/core/agent"
	"git.glkj.example.com/glkj/harness/go/internal/core/session"
	"git.glkj.example.com/glkj/harness/go/internal/llm"
	"git.glkj.example.com/glkj/harness/go/internal/plugin"
)

type fakeProvider struct {
	chunks []string
	last   llm.Request
}

func (f *fakeProvider) Stream(_ context.Context, request llm.Request, emit func(llm.Chunk) error) error {
	f.last = request
	for _, text := range f.chunks {
		if err := emit(llm.Chunk{Text: text}); err != nil {
			return err
		}
	}
	return emit(llm.Chunk{Finished: true})
}

func TestRunRecordsTheDurableEventSequence(t *testing.T) {
	ctx := plugin.New(nil)
	defer ctx.Dispose()
	log := session.NewLog("s1")
	provider := &fakeProvider{chunks: []string{"he", "llo"}}

	runner := agent.New(ctx, log, agent.Options{Provider: provider, Model: "deepseek-chat"})
	if err := runner.Run(context.Background(), "hi"); err != nil {
		t.Fatalf("run: %v", err)
	}

	var types []string
	for _, event := range log.Events() {
		types = append(types, event.Type)
	}
	want := []string{"turn/start", "step/start", "user/message", "assistant/chunk", "assistant/chunk", "assistant/message", "step/end", "turn/end"}
	if len(types) != len(want) {
		t.Fatalf("events = %v, want %v", types, want)
	}
	for i := range want {
		if types[i] != want[i] {
			t.Fatalf("events = %v, want %v", types, want)
		}
	}

	messages := log.DeriveMessages()
	if len(messages) != 2 || messages[1].Text != "hello" {
		t.Fatalf("messages = %+v", messages)
	}
	if len(provider.last.Messages) != 1 || provider.last.Messages[0].Content != "hi" {
		t.Fatalf("request messages = %+v", provider.last.Messages)
	}
}

func TestPreStepWaterfallCanRejectTheTurn(t *testing.T) {
	ctx := plugin.New(nil)
	defer ctx.Dispose()
	ctx.OnMiddleware("agent/pre-step", func(_ context.Context, _ any, _ func(context.Context) (any, error)) (any, error) {
		return nil, context.Canceled
	})

	log := session.NewLog("s1")
	runner := agent.New(ctx, log, agent.Options{Provider: &fakeProvider{}, Model: "m"})
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

package llm_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"git.glkj.example.com/glkj/harness/go/internal/llm"
)

func TestStreamDecodesSSEChunks(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer key" {
			t.Errorf("authorization = %q", got)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, line := range []string{
			`data: {"choices":[{"delta":{"content":"he"}}]}`,
			`data: {"choices":[{"delta":{"content":"llo"}}]}`,
			`data: {"choices":[{"delta":{},"finish_reason":"stop"}]}`,
			`data: [DONE]`,
		} {
			w.Write([]byte(line + "\n\n"))
		}
	}))
	defer server.Close()

	provider := llm.NewDeepSeek(server.URL, "key")
	var text strings.Builder
	var finished bool
	err := provider.Stream(context.Background(), llm.Request{
		Model:    "deepseek-chat",
		Messages: []llm.Message{{Role: "user", Content: "hi"}},
	}, func(chunk llm.Chunk) error {
		text.WriteString(chunk.Text)
		if chunk.Finished {
			finished = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if text.String() != "hello" {
		t.Fatalf("text = %q, want hello", text.String())
	}
	if !finished {
		t.Fatal("stream never reported a finished chunk")
	}
}

func TestStreamRequiresCredential(t *testing.T) {
	provider := llm.NewDeepSeek("", "")
	err := provider.Stream(context.Background(), llm.Request{}, func(llm.Chunk) error { return nil })
	if err != llm.ErrMissingCredential {
		t.Fatalf("err = %v, want ErrMissingCredential", err)
	}
}

func TestStreamReportsHTTPFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusUnauthorized)
	}))
	defer server.Close()

	provider := llm.NewDeepSeek(server.URL, "key")
	err := provider.Stream(context.Background(), llm.Request{}, func(llm.Chunk) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("err = %v, want a 401 report", err)
	}
}

func TestStreamSettlesToolCallsWithoutAFinishChunk(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		for _, line := range []string{
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","function":{"name":"echo","arguments":"{\"a\":"}}]}}]}`,
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"1}"}}]}}]}`,
			`data: [DONE]`,
		} {
			w.Write([]byte(line + "\n\n"))
		}
	}))
	defer server.Close()

	provider := llm.NewDeepSeek(server.URL, "key")
	var calls []llm.ToolCall
	var finished int
	err := provider.Stream(context.Background(), llm.Request{}, func(chunk llm.Chunk) error {
		calls = append(calls, chunk.ToolCalls...)
		if chunk.Finished {
			finished++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	if len(calls) != 1 || calls[0].ID != "call-1" || calls[0].Name != "echo" || calls[0].Arguments != `{"a":1}` {
		t.Fatalf("calls = %+v", calls)
	}
	if finished != 1 {
		t.Fatalf("finished = %d, want 1 settlement", finished)
	}
}

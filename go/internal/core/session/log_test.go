package session_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"git.glkj.example.com/glkj/harness/go/internal/core/session"
)

func userMessage(text string) map[string]any {
	return map[string]any{
		"role":    "user",
		"content": []any{map[string]any{"type": "text", "text": text}},
		"source":  map[string]any{"kind": "user"},
	}
}

func assistantMessage(text string) map[string]any {
	return map[string]any{
		"turn": 1,
		"step": 1,
		"message": map[string]any{
			"role":    "assistant",
			"content": []any{map[string]any{"type": "text", "text": text}},
			"source":  map[string]any{"kind": "model"},
		},
	}
}

func TestAppendAssignsSequenceNumbers(t *testing.T) {
	log := session.NewLog("s1")
	first, err := log.Append("turn/start", map[string]int{"turn": 1})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	second, err := log.Append("step/start", map[string]int{"turn": 1, "step": 1})
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if first.Seq != 0 || second.Seq != 1 {
		t.Fatalf("seq = %d, %d, want 0, 1", first.Seq, second.Seq)
	}
	if log.Len() != 2 {
		t.Fatalf("len = %d, want 2", log.Len())
	}
}

func TestDeriveMessagesFollowsTheSurface(t *testing.T) {
	log := session.NewLog("s1")
	log.Append("turn/start", map[string]int{"turn": 1})
	if _, err := log.Append("user/message", userMessage("hello"), session.WithAppend()); err != nil {
		t.Fatalf("append user: %v", err)
	}
	log.Append("assistant/chunk", map[string]string{"text": "partial"})
	if _, err := log.Append("assistant/message", assistantMessage("hi"), session.WithAppend()); err != nil {
		t.Fatalf("append assistant: %v", err)
	}

	messages, err := log.DeriveMessages()
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2", len(messages))
	}
	if messages[0].Role() != "user" || messages[1].Role() != "assistant" {
		t.Fatalf("roles = %q, %q", messages[0].Role(), messages[1].Role())
	}
}

func TestSurfaceReplaceShadowsTheRange(t *testing.T) {
	log := session.NewLog("s1")
	log.Append("user/message", userMessage("first"), session.WithAppend())
	start, _ := log.Append("assistant/message", assistantMessage("draft"), session.WithAppend())
	end, _ := log.Append("tool/result", map[string]any{
		"message": map[string]any{"role": "tool", "content": []any{map[string]any{"type": "text", "text": "ok"}}},
	}, session.WithAppend())

	if _, err := log.Append("user/message", userMessage("compact"), session.WithReplace(start.Seq, end.Seq, []int{start.Seq, end.Seq})); err != nil {
		t.Fatalf("append replace: %v", err)
	}

	messages, err := log.DeriveMessages()
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if len(messages) != 2 {
		t.Fatalf("messages = %d, want 2 (user + replacement)", len(messages))
	}
	if messages[0].Role() != "user" || messages[1].Role() != "user" {
		t.Fatalf("roles = %q, %q", messages[0].Role(), messages[1].Role())
	}
}

func TestSurfaceEligibleEventWithoutMarkerFails(t *testing.T) {
	log := session.NewLog("s1")
	log.Append("user/message", userMessage("hello"))
	if _, err := log.DeriveMessages(); err == nil {
		t.Fatal("expected a missing surfaceOp to fail derivation")
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.v4.jsonl")
	log := session.NewLog("s1")
	log.Header.Cwd = "/tmp/work"
	log.Append("turn/start", map[string]int{"turn": 1})
	log.Append("user/message", userMessage("hello"), session.WithAppend())

	if err := log.WriteFile(path); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) != 3 {
		t.Fatalf("lines = %d, want 3", len(lines))
	}
	if !strings.HasPrefix(lines[0], `{"type":"session","version":4`) {
		t.Fatalf("header = %s", lines[0])
	}
	if !strings.Contains(lines[2], `"surfaceOp":"append"`) {
		t.Fatalf("user row = %s", lines[2])
	}

	loaded, err := session.ReadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Header.ID != "s1" || loaded.Header.Cwd != "/tmp/work" {
		t.Fatalf("header = %+v", loaded.Header)
	}
	if loaded.Len() != 2 {
		t.Fatalf("events = %d, want 2", loaded.Len())
	}
	messages, err := loaded.DeriveMessages()
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if len(messages) != 1 || messages[0].Role() != "user" {
		t.Fatalf("messages = %+v", messages)
	}
}

func TestForkCopiesThePrefixAndResumeContinuesSeq(t *testing.T) {
	log := session.NewLog("s1")
	log.Append("turn/start", map[string]int{"turn": 1})
	log.Append("user/message", userMessage("hello"), session.WithAppend())
	log.Append("step/start", map[string]int{"turn": 1, "step": 1})

	child, err := log.Fork("s2", 1)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if child.Header.ID != "s2" || child.Header.ParentSession != "s1" {
		t.Fatalf("child header = %+v", child.Header)
	}
	if child.Len() != 2 {
		t.Fatalf("child events = %d, want 2", child.Len())
	}

	event, err := child.Append("user/message", userMessage("child"), session.WithAppend())
	if err != nil {
		t.Fatalf("append: %v", err)
	}
	if event.Seq != 2 {
		t.Fatalf("child seq = %d, want 2", event.Seq)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "child.v4.jsonl")
	if err := child.WriteFile(path); err != nil {
		t.Fatalf("write: %v", err)
	}
	var buffer bytes.Buffer
	if err := child.Write(&buffer); err != nil {
		t.Fatalf("write buffer: %v", err)
	}
	loaded, err := session.ReadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Header.ParentSession != "s1" || loaded.Len() != 3 {
		t.Fatalf("loaded header = %+v events = %d", loaded.Header, loaded.Len())
	}
}

func TestResumeFromDiskContinuesTheTranscript(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.v4.jsonl")

	first := session.NewLog("s1")
	first.Append("turn/start", map[string]int{"turn": 1})
	first.Append("user/message", userMessage("one"), session.WithAppend())
	first.Append("assistant/message", assistantMessage("two"), session.WithAppend())
	if err := first.WriteFile(path); err != nil {
		t.Fatalf("write: %v", err)
	}

	resumed, err := session.ReadFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	event, err := resumed.Append("user/message", userMessage("three"), session.WithAppend())
	if err != nil {
		t.Fatalf("append after resume: %v", err)
	}
	if event.Seq != 3 {
		t.Fatalf("seq after resume = %d, want 3", event.Seq)
	}

	messages, err := resumed.DeriveMessages()
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if len(messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(messages))
	}
	if messages[0].Role() != "user" || messages[2].Role() != "user" {
		t.Fatalf("roles = %q, %q", messages[0].Role(), messages[2].Role())
	}
}

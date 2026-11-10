package session_test

import (
	"testing"

	"git.glkj.example.com/glkj/harness/go/internal/core/session"
)

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

func TestDeriveMessagesKeepsOnlyModelVisibleText(t *testing.T) {
	log := session.NewLog("s1")
	log.Append("turn/start", map[string]int{"turn": 1})
	log.Append("user/message", map[string]string{"text": "hello"})
	log.Append("assistant/chunk", map[string]string{"text": "partial"})
	log.Append("assistant/message", map[string]string{"text": "hi"})
	log.Append("tool/result", map[string]string{"text": "ignored"})

	messages := log.DeriveMessages()
	if len(messages) != 2 {
		t.Fatalf("messages = %+v, want 2", messages)
	}
	if messages[0] != (session.Message{Role: "user", Text: "hello"}) {
		t.Fatalf("first = %+v", messages[0])
	}
	if messages[1] != (session.Message{Role: "assistant", Text: "hi"}) {
		t.Fatalf("second = %+v", messages[1])
	}
}

package session

import (
	"testing"

	"github.com/agent/ai-terminal/internal/core"
)

func TestSessionCreateAndGet(t *testing.T) {
	store := NewStore(100, "/tmp/agent-test-sessions", false)
	sess := store.Create()

	if sess.ID() == "" {
		t.Fatal("expected non-empty session ID")
	}

	got, ok := store.Get(sess.ID())
	if !ok {
		t.Fatal("expected to find session")
	}
	if got.ID() != sess.ID() {
		t.Fatal("session ID mismatch")
	}
}

func TestSessionAppendMessages(t *testing.T) {
	store := NewStore(100, "/tmp/agent-test-sessions", false)
	sess := store.Create()

	sess.Append(core.Message{Role: core.RoleUser, Content: "hello"})
	sess.Append(core.Message{Role: core.RoleAssistant, Content: "world"})

	msgs := sess.Messages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Content != "hello" {
		t.Fatalf("expected 'hello', got %q", msgs[0].Content)
	}
	if msgs[1].Content != "world" {
		t.Fatalf("expected 'world', got %q", msgs[1].Content)
	}
}

func TestSessionClear(t *testing.T) {
	store := NewStore(100, "/tmp/agent-test-sessions", false)
	sess := store.Create()

	sess.Append(core.Message{Role: core.RoleUser, Content: "hello"})
	sess.Clear()

	msgs := sess.Messages()
	if len(msgs) != 0 {
		t.Fatalf("expected 0 messages after clear, got %d", len(msgs))
	}
}

func TestSessionMetadata(t *testing.T) {
	store := NewStore(100, "/tmp/agent-test-sessions", false)
	sess := store.Create()

	sess.SetMetadata("key1", "value1")
	sess.SetMetadata("key2", "value2")

	val, ok := sess.GetMetadata("key1")
	if !ok || val != "value1" {
		t.Fatalf("expected 'value1', got %q", val)
	}

	_, ok = sess.GetMetadata("nonexistent")
	if ok {
		t.Fatal("expected nonexistent metadata to not be found")
	}
}

func TestSessionDelete(t *testing.T) {
	store := NewStore(100, "/tmp/agent-test-sessions", false)
	sess := store.Create()

	store.Delete(sess.ID())

	_, ok := store.Get(sess.ID())
	if ok {
		t.Fatal("expected session to be deleted")
	}
}

func TestSessionList(t *testing.T) {
	store := NewStore(100, "/tmp/agent-test-sessions", false)

	s1 := store.Create()
	s2 := store.Create()

	list := store.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(list))
	}

	ids := map[string]bool{list[0].ID(): true, list[1].ID(): true}
	if !ids[s1.ID()] || !ids[s2.ID()] {
		t.Fatal("expected both sessions in list")
	}
}

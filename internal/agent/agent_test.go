package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/agent/ai-terminal/internal/core"
)

type testProvider struct {
	stream func(context.Context, *core.Request) <-chan core.Token
}

func (p testProvider) Name() string { return "test" }

func (p testProvider) Stream(ctx context.Context, req *core.Request) (<-chan core.Token, error) {
	return p.stream(ctx, req), nil
}

type recordingSession struct {
	mu       sync.Mutex
	messages []core.Message
}

func (s *recordingSession) ID() string { return "test" }
func (s *recordingSession) Messages() []core.Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]core.Message(nil), s.messages...)
}
func (s *recordingSession) Append(message core.Message) {
	s.mu.Lock()
	s.messages = append(s.messages, message)
	s.mu.Unlock()
}
func (s *recordingSession) Replace(messages []core.Message) {
	s.mu.Lock()
	s.messages = append([]core.Message(nil), messages...)
	s.mu.Unlock()
}
func (s *recordingSession) Clear()                              { s.Replace(nil) }
func (s *recordingSession) SetMetadata(string, string)          {}
func (s *recordingSession) GetMetadata(string) (string, bool)   { return "", false }

func TestRunForwardsTokensBeforeProviderCompletes(t *testing.T) {
	release := make(chan struct{})
	provider := testProvider{stream: func(ctx context.Context, _ *core.Request) <-chan core.Token {
		out := make(chan core.Token)
		go func() {
			defer close(out)
			out <- core.Token{Content: "first"}
			<-release
			out <- core.Token{Content: " second"}
			out <- core.Token{Done: true}
		}()
		return out
	}}

	a := New(provider, nil)
	out, err := a.Run(context.Background(), &core.Request{Messages: []core.Message{{Role: core.RoleUser, Content: "hello"}}})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	select {
	case token := <-out:
		if token.Content != "first" {
			t.Fatalf("first token = %q, want first", token.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("first token was withheld until the provider completed")
	}

	close(release)
	for range out {
	}
}

func TestRunDoesNotDuplicateCallerOwnedHistory(t *testing.T) {
	provider := testProvider{stream: func(ctx context.Context, _ *core.Request) <-chan core.Token {
		out := make(chan core.Token, 2)
		out <- core.Token{Content: "answer"}
		out <- core.Token{Done: true}
		close(out)
		return out
	}}
	sess := &recordingSession{}
	a := New(provider, nil, WithSession(sess))
	request := &core.Request{Messages: []core.Message{
		{Role: core.RoleSystem, Content: "instructions"},
		{Role: core.RoleUser, Content: "question"},
	}}

	out, err := a.Run(context.Background(), request)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	for range out {
	}

	messages := sess.Messages()
	if len(messages) != 1 {
		t.Fatalf("session has %d messages, want only assistant reply: %#v", len(messages), messages)
	}
	if messages[0].Role != core.RoleAssistant || messages[0].Content != "answer" {
		t.Fatalf("unexpected stored message: %#v", messages[0])
	}
}

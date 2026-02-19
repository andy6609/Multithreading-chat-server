package chat

import (
	"strings"
	"testing"
	"time"
)

func TestRegistry_RegisterRejectsDuplicateUsername(t *testing.T) {
	r := NewRegistry(128, nil)
	go r.Run()
	t.Cleanup(func() {
		r.Stop()
		r.Wait()
	})

	c1 := &Client{Out: make(chan string, 64)}
	c2 := &Client{Out: make(chan string, 64)}

	reply1 := make(chan error, 1)
	r.events <- Event{Type: EventRegister, Client: c1, Username: "alice", ReplyChan: reply1}
	if err := <-reply1; err != nil {
		t.Fatalf("expected nil, got %v", err)
	}

	reply2 := make(chan error, 1)
	r.events <- Event{Type: EventRegister, Client: c2, Username: "alice", ReplyChan: reply2}
	if err := <-reply2; err != ErrUsernameTaken {
		t.Fatalf("expected ErrUsernameTaken, got %v", err)
	}
}

func TestRegistry_UsersReflectJoinLeave(t *testing.T) {
	r := NewRegistry(128, nil)
	go r.Run()
	t.Cleanup(func() {
		r.Stop()
		r.Wait()
	})

	alice := &Client{Out: make(chan string, 256)}
	bob := &Client{Out: make(chan string, 256)}

	register(t, r, alice, "alice")
	register(t, r, bob, "bob")

	r.events <- Event{Type: EventUsers, Client: alice}
	line := waitForPrefix(t, alice.Out, "USERS: ")
	if line != "USERS: alice,bob" {
		t.Fatalf("unexpected users line: %q", line)
	}

	r.events <- Event{Type: EventUnregister, Client: bob}

	r.events <- Event{Type: EventUsers, Client: alice}
	line = waitForPrefix(t, alice.Out, "USERS: ")
	if line != "USERS: alice" {
		t.Fatalf("unexpected users line after leave: %q", line)
	}
}

func TestRegistry_WhisperRoutesOrErrors(t *testing.T) {
	r := NewRegistry(128, nil)
	go r.Run()
	t.Cleanup(func() {
		r.Stop()
		r.Wait()
	})

	alice := &Client{Out: make(chan string, 256)}
	bob := &Client{Out: make(chan string, 256)}

	register(t, r, alice, "alice")
	register(t, r, bob, "bob")

	// Successful whisper: only receiver gets it.
	r.events <- Event{Type: EventWhisper, Client: alice, To: "bob", Text: "hello bob"}
	got := waitForPrefix(t, bob.Out, "WHISPER ")
	if got != "WHISPER alice: hello bob" {
		t.Fatalf("unexpected whisper line: %q", got)
	}

	// Unknown receiver: sender gets ERR.
	r.events <- Event{Type: EventWhisper, Client: alice, To: "nobody", Text: "hi"}
	errLine := waitForPrefix(t, alice.Out, "ERR ")
	if errLine != "ERR user_not_found" {
		t.Fatalf("unexpected err line: %q", errLine)
	}

	// Self whisper forbidden in this implementation.
	r.events <- Event{Type: EventWhisper, Client: alice, To: "alice", Text: "me"}
	errLine = waitForPrefix(t, alice.Out, "ERR ")
	if errLine != "ERR cannot_whisper_self" {
		t.Fatalf("unexpected self-whisper err line: %q", errLine)
	}
}

func register(t *testing.T, r *Registry, c *Client, username string) {
	t.Helper()
	reply := make(chan error, 1)
	r.events <- Event{Type: EventRegister, Client: c, Username: username, ReplyChan: reply}
	if err := <-reply; err != nil {
		t.Fatalf("register(%s) error: %v", username, err)
	}
}

func waitForPrefix(t *testing.T, ch <-chan string, prefix string) string {
	t.Helper()
	deadline := time.NewTimer(1 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case s := <-ch:
			if strings.HasPrefix(s, prefix) {
				return s
			}
			// ignore other lines (OK, SYSTEM, etc.)
		case <-deadline.C:
			t.Fatalf("timeout waiting for prefix %q", prefix)
		}
	}
}

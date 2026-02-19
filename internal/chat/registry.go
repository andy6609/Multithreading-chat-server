package chat

import (
	"log/slog"
	"sort"
	"strings"
	"time"
)

type Registry struct {
	events chan Event
	stopCh chan struct{}
	doneCh chan struct{}
	logger *slog.Logger
}

func NewRegistry(buffer int, logger *slog.Logger) *Registry {
	if buffer <= 0 {
		buffer = 64
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Registry{
		events: make(chan Event, buffer),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
		logger: logger,
	}
}

func (r *Registry) Events() chan<- Event {
	return r.events
}

// Stop signals the Run loop to exit.
func (r *Registry) Stop() {
	close(r.stopCh)
}

// Wait blocks until the Run loop has completely finished.
func (r *Registry) Wait() {
	<-r.doneCh
}

func (r *Registry) Run() {
	defer close(r.doneCh)
	// Single-writer ownership: this map is only accessed in this goroutine.
	clients := make(map[string]*Client)

	for {
		select {
		case ev := <-r.events:
			start := time.Now()
			eventType := ""

			switch ev.Type {
			case EventRegister:
				eventType = "register"
				r.handleRegister(clients, ev)
				ConnectedClients.Set(float64(len(clients)))
			case EventUnregister:
				eventType = "unregister"
				r.handleUnregister(clients, ev)
				ConnectedClients.Set(float64(len(clients)))
			case EventBroadcast:
				eventType = "broadcast"
				r.handleBroadcast(clients, ev)
			case EventUsers:
				eventType = "users"
				r.handleUsers(clients, ev)
			case EventWhisper:
				eventType = "whisper"
				r.handleWhisper(clients, ev)
			}

			MessagesTotal.WithLabelValues(eventType).Inc()
			EventProcessingDuration.WithLabelValues(eventType).Observe(time.Since(start).Seconds())
		case <-r.stopCh:
			return
		}
	}
}

func (r *Registry) handleRegister(clients map[string]*Client, ev Event) {
	defer func() {
		// ReplyChan is only used for register.
		if ev.ReplyChan != nil {
			close(ev.ReplyChan)
		}
	}()

	username := strings.TrimSpace(ev.Username)
	if username == "" || len(username) > 16 {
		if ev.ReplyChan != nil {
			ev.ReplyChan <- ErrUsernameInvalid
		}
		return
	}
	if _, exists := clients[username]; exists {
		if ev.ReplyChan != nil {
			ev.ReplyChan <- ErrUsernameTaken
		}
		return
	}

	ev.Client.Username = username
	clients[username] = ev.Client

	r.logger.Info("user registered", "username", username)

	// Minimal UX: notify client it's accepted and notify others.
	sendLine(ev.Client, "OK")
	r.broadcastSystem(clients, username+" joined")

	if ev.ReplyChan != nil {
		ev.ReplyChan <- nil
	}
}

func (r *Registry) handleUnregister(clients map[string]*Client, ev Event) {
	if ev.Client == nil || ev.Client.Username == "" {
		return
	}
	username := ev.Client.Username
	if _, ok := clients[username]; !ok {
		return
	}
	delete(clients, username)

	r.logger.Info("user left", "username", username)

	// Closing Out stops the writer goroutine gracefully.
	close(ev.Client.Out)
	r.broadcastSystem(clients, username+" left")
}

func (r *Registry) handleBroadcast(clients map[string]*Client, ev Event) {
	if ev.Client == nil || ev.Client.Username == "" {
		return
	}
	msg := strings.TrimRight(ev.Text, "\r\n")
	if msg == "" {
		return
	}
	if len(msg) > 512 {
		msg = msg[:512]
	}

	line := ev.Client.Username + ": " + msg
	for _, c := range clients {
		sendLine(c, line)
	}
}

func (r *Registry) handleUsers(clients map[string]*Client, ev Event) {
	if ev.Client == nil {
		return
	}
	names := make([]string, 0, len(clients))
	for name := range clients {
		names = append(names, name)
	}
	sort.Strings(names)
	sendLine(ev.Client, "USERS: "+strings.Join(names, ","))
}

func (r *Registry) handleWhisper(clients map[string]*Client, ev Event) {
	if ev.Client == nil || ev.Client.Username == "" {
		return
	}

	to := strings.TrimSpace(ev.To)
	if to == "" {
		sendLine(ev.Client, "ERR user_not_found")
		return
	}
	if to == ev.Client.Username {
		sendLine(ev.Client, "ERR cannot_whisper_self")
		return
	}

	receiver, ok := clients[to]
	if !ok {
		sendLine(ev.Client, "ERR user_not_found")
		return
	}

	msg := strings.TrimRight(ev.Text, "\r\n")
	if msg == "" {
		return
	}
	if len(msg) > 512 {
		msg = msg[:512]
	}

	sendLine(receiver, "WHISPER "+ev.Client.Username+": "+msg)
}

func (r *Registry) broadcastSystem(clients map[string]*Client, text string) {
	line := "SYSTEM: " + text
	for _, c := range clients {
		sendLine(c, line)
	}
}

func sendLine(c *Client, line string) {
	// Non-blocking send prevents slow/disconnected clients from blocking the registry.
	select {
	case c.Out <- line:
	default:
		// Drop when the client is slow; this is MVP backpressure behavior.
	}
}

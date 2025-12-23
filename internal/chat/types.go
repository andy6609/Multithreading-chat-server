package chat

import "net"

type Client struct {
	Conn     net.Conn
	Username string
	Out      chan string // outbound messages to be written by the writer goroutine
}

type EventType int

const (
	EventRegister EventType = iota
	EventUnregister
	EventBroadcast
	EventUsers
	EventWhisper
)

type Event struct {
	Type      EventType
	Client    *Client
	Username  string
	To        string
	Text      string
	ReplyChan chan error // used by register to ack success/failure
}

var (
	ErrUsernameTaken   = errorString("username_taken")
	ErrUsernameInvalid = errorString("username_invalid")
)

type errorString string

func (e errorString) Error() string { return string(e) }



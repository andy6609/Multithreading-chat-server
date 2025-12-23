package chat

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

var whisperRe = regexp.MustCompile(`^/w\s+(\S+)\s+(.+)$`)

func HandleSession(c *Client, events chan<- Event) {
	defer func() {
		_ = c.Conn.Close()
	}()

	StartOutboundWriter(c.Conn, c.Out)

	reader := bufio.NewReader(c.Conn)

	// Username handshake loop (minimal UX).
	for {
		sendLine(c, "Enter username:")
		line, err := readLine(reader)
		if err != nil {
			return
		}
		username := strings.TrimSpace(line)

		reply := make(chan error, 1)
		events <- Event{
			Type:      EventRegister,
			Client:    c,
			Username:  username,
			ReplyChan: reply,
		}
		if regErr := <-reply; regErr != nil {
			switch regErr {
			case ErrUsernameTaken:
				sendLine(c, "ERR username_taken")
			case ErrUsernameInvalid:
				sendLine(c, "ERR username_invalid")
			default:
				sendLine(c, "ERR register_failed")
			}
			continue
		}
		break
	}

	// Main input loop.
	for {
		line, err := readLine(reader)
		if err != nil {
			events <- Event{Type: EventUnregister, Client: c}
			return
		}

		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			continue
		case line == "/exit":
			sendLine(c, "Bye")
			events <- Event{Type: EventUnregister, Client: c}
			return
		case line == "/users":
			events <- Event{Type: EventUsers, Client: c}
		case strings.HasPrefix(line, "/w"):
			m := whisperRe.FindStringSubmatch(line)
			if m == nil {
				sendLine(c, "ERR whisper_usage")
				continue
			}
			to := m[1]
			text := strings.TrimSpace(m[2])
			if text == "" {
				sendLine(c, "ERR whisper_usage")
				continue
			}
			if len(text) > 512 {
				text = text[:512]
			}
			events <- Event{Type: EventWhisper, Client: c, To: to, Text: text}
		default:
			events <- Event{Type: EventBroadcast, Client: c, Text: line}
		}
	}
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err == nil {
		return strings.TrimRight(line, "\r\n"), nil
	}
	if err == io.EOF && line != "" {
		// last line without newline
		return strings.TrimRight(line, "\r\n"), nil
	}
	if err == io.EOF {
		return "", io.EOF
	}
	return "", fmt.Errorf("read: %w", err)
}



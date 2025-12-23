package chat

import (
	"bufio"
	"net"
)

func StartOutboundWriter(conn net.Conn, out <-chan string) {
	go func() {
		w := bufio.NewWriter(conn)
		for msg := range out {
			// Best-effort. If the connection breaks, just stop the writer.
			if _, err := w.WriteString(msg + "\n"); err != nil {
				return
			}
			if err := w.Flush(); err != nil {
				return
			}
		}
	}()
}



package protostream

import (
	"nanomsg.org/go/mangos/v2"
)

// Socket contains a mangos.Socket to publish/subscribe to
type Socket struct {
	sock   mangos.Socket
	prefix string
	q      chan struct{}
}

// Quit stops the listener and closes the connection to the socket
func (s *Socket) Quit() {
	if s == nil {
		return
	}

	// close the socket
	if s.sock != nil {
		_ = s.sock.Close()
	}

	// send quit signal to any listeners
	close(s.q)
}

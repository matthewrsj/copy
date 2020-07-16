package protostream

import (
	"strings"

	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/protocol/pub"
	"nanomsg.org/go/mangos/v2/transport/ws"
)

// NewPublisher returns a publisher socket to the url
func NewPublisher(url, prefix string) (*Socket, error) {
	var (
		sock mangos.Socket
		err  error
	)

	if sock, err = pub.NewSocket(); err != nil {
		return nil, err
	}

	var opts map[string]interface{}

	if strings.HasPrefix(url, "ws") {
		opts = map[string]interface{}{ws.OptionWebSocketCheckOrigin: false}
	}

	l, err := sock.NewListener(url, opts)
	if err != nil {
		return nil, err
	}

	if err = l.Listen(); err != nil {
		return nil, err
	}

	return &Socket{sock: sock, prefix: prefix, q: make(chan struct{})}, nil
}

// Publish sends the msg to the socket as a []byte
func (s *Socket) Publish(msg []byte) error {
	return s.sock.Send(append([]byte(s.prefix), msg...))
}

// PublishTo sends the msg with prefix to the socket as a []byte
func (s *Socket) PublishTo(prefix string, msg []byte) error {
	return s.sock.Send(append([]byte(prefix), msg...))
}

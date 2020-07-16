package protostream

import (
	"nanomsg.org/go/mangos/v2"
	"nanomsg.org/go/mangos/v2/protocol/rep"
	"nanomsg.org/go/mangos/v2/transport/ws"
)

// NewReplier returns a reply socket to the url
func NewReplier(url, prefix string) (*Socket, error) {
	var (
		sock mangos.Socket
		err  error
	)

	if sock, err = rep.NewSocket(); err != nil {
		return nil, err
	}

	l, err := sock.NewListener(url, map[string]interface{}{ws.OptionWebSocketCheckOrigin: false})
	if err != nil {
		return nil, err
	}

	if err = l.Listen(); err != nil {
		return nil, err
	}

	return &Socket{sock: sock, prefix: prefix, q: make(chan struct{})}, nil
}

// ReqRep is a listener on the socket returned by Subscribe. Returns a channel
// which will receive []byte messages as they are received. Pass messages to the
// reply channel to reply to the request. To stop listening tell the socket to
// Quit()
func (s *Socket) ReqRep(reply chan []byte) (request chan []byte) {
	request = make(chan []byte)

	go func() {
		for {
			select {
			case <-s.q:
				close(request)
				return
			default:
				msg, err := s.sock.Recv()
				if err != nil {
					request <- []byte(err.Error())
				}

				request <- msg

				rp := <-reply

				if err = s.sock.Send(rp); err != nil {
					request <- []byte(err.Error())
				}
			}
		}
	}()

	return
}

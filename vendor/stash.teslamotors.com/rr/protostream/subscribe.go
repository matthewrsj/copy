package protostream

import (
	"bytes"
	"context"
	"time"

	"nanomsg.org/go/mangos/v2"
	mangoerrs "nanomsg.org/go/mangos/v2/errors"
	"nanomsg.org/go/mangos/v2/protocol/sub"
	_ "nanomsg.org/go/mangos/v2/transport/all" // register transports
)

// Message contains a mangos.Message referance and an Err to indicate when a
// RecvMsg failed
type Message struct {
	Msg *mangos.Message
	Err error
}

// NewSubscriber returns a socket to the url
func NewSubscriber(url string) (*Socket, error) {
	var (
		sock mangos.Socket
		err  error
	)

	if sock, err = sub.NewSocket(); err != nil {
		return nil, err
	}

	if err = sock.Dial(url); err != nil {
		return nil, err
	}

	return &Socket{sock: sock, q: make(chan struct{})}, nil
}

// NewSubscriberWithSub returns a socket to the url subscribed to sub
func NewSubscriberWithSub(url, sb string) (*Socket, error) {
	s, err := NewSubscriber(url)
	if err != nil {
		return nil, err
	}

	err = s.Subscribe(sb)

	return s, err
}

// Subscribe subscribes to the sub
func (s *Socket) Subscribe(sb string) error {
	s.prefix = sb
	return s.sock.SetOption(mangos.OptionSubscribe, sb)
}

// getMessage follows the same principle as time.After (and also Listen) as it
// returns a channel to which it will write the result of RecvMsg().
func (s *Socket) getMessage() <-chan *Message {
	c := make(chan *Message)

	go func() {
		msg, err := s.sock.RecvMsg()
		if err != mangoerrs.ErrClosed {
			c <- &Message{Msg: msg, Err: err}
		}

		close(c)
	}()

	return c
}

// Listen is a listener on the socket returned by Subscribe. Returns a channel
// which will receive *Messages as they are received. To stop listening tell the
// socket to Quit()
func (s *Socket) Listen() <-chan *Message {
	c := make(chan *Message)

	go func() {
		useSameChannel := false

		var msgChan <-chan *Message

		for {
			select {
			case <-s.q:
				// quit signal received
				close(c)
				return
			default:
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)

				if !useSameChannel {
					// this is either the first read we did or we actually got a
					// value on the last iteration. Either way we need to get a
					// new channel and spawn a RecvMsg via getMessage().
					msgChan = s.getMessage()
				}
				select {
				case m := <-msgChan:
					// nil message needs to be ignored
					if m == nil || m.Msg == nil || m.Msg.Body == nil {
						break
					}

					m.Msg.Body = bytes.TrimPrefix(m.Msg.Body, []byte(s.prefix))
					c <- m
					// we consumed the value off this channel, so we know the
					// goroutine in getMessage() ended and isn't blocked on
					// RecvMsg anymore. This means on the next iteration we will
					// need to call getMessage() again to get a new channel.
					useSameChannel = false
				case <-ctx.Done():
					// There was nothing on the socket. This is not an error, we
					// just need to iterate again to check for the quit signal
					// because of the timeout use the same channel since RecvMsg
					// is still blocked waiting for a message on the socket.
					useSameChannel = true
				}
				// this context only exists for this one read, it will be
				// created again next iteration
				cancel()
			}
		}
	}()
	// immediately return a future channel
	return c
}

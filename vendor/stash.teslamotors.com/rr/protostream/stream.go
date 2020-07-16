// Package protostream provides a stream interface for tower proto messages for all configured towers
package protostream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/go-socketcan/pkg/socketcan"
	tower "stash.teslamotors.com/rr/towerproto"
)

const (
	// DefaultWebsocketAddress is used if the WithWebsocketAddress option is not passed to NewStream
	DefaultWebsocketAddress = "localhost:8080"
	// WSEndpoint is the endpoint to which the streamer posts
	WSEndpoint = "/proto"
)

const (
	_recvTimeout = time.Second * 3
)

// Stream sends Messages over a socket for other processes to read
type Stream struct {
	fixtures  map[string]CANConfig
	wsAddr    string
	publisher *Socket
	logger    *zap.SugaredLogger
}

// Option function to set configuration on a Stream
type Option func(*Stream) error

// WithFixtures sets the Stream fixtures to the fxrs argument
func WithFixtures(fxrs map[string]CANConfig) Option {
	return func(s *Stream) error {
		s.fixtures = fxrs
		return nil
	}
}

// WithLogger sets the stream logger
func WithLogger(logger *zap.SugaredLogger) Option {
	return func(s *Stream) error {
		s.logger = logger
		return nil
	}
}

// WithWebsocketAddress sets the dealer on the stream to the address
func WithWebsocketAddress(address string) Option {
	return func(s *Stream) error {
		s.wsAddr = address // for logging purposes
		u := url.URL{Scheme: "ws", Host: s.wsAddr, Path: WSEndpoint}

		sock, err := NewPublisher(u.String(), "")
		if err != nil {
			return err
		}

		s.publisher = sock

		return nil
	}
}

// NewStream starts streaming Messages to the socket
func NewStream(opts ...Option) (*Stream, error) {
	s := &Stream{
		fixtures: make(map[string]CANConfig),
		logger:   zap.NewExample().Sugar(),
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return s, err
		}
	}

	if s.wsAddr == "" {
		if err := WithWebsocketAddress(DefaultWebsocketAddress)(s); err != nil {
			return s, err
		}
	}

	return s, nil
}

// ProtoMessage is the data piped over the socket
type ProtoMessage struct {
	Location string `json:"location"`
	Body     []byte `json:"body"`
}

func loopForMessages(sock *Socket, loc string, can CANConfig, ctx context.Context, wg *sync.WaitGroup, logger *zap.SugaredLogger) {
	defer wg.Done()

	cl := logger.With("fixture", loc, "can_bus", can.Bus, "can_rx", fmt.Sprintf("0x%X", can.RX), "can_tx", fmt.Sprintf("0x%X", can.TX))

	dev, err := socketcan.NewIsotpInterface(can.Bus, can.RX, can.TX)
	if err != nil {
		cl.Errorw("create new ISOTP interface", "error", err)
		return
	}

	defer func() {
		_ = dev.Close()
	}()

	if err = dev.SetCANFD(); err != nil {
		cl.Errorw("set CANFD options", "error", err)
		return
	}

	if err = dev.SetRecvTimeout(_recvTimeout); err != nil {
		cl.Errorw("set recv timeout", "timeout", _recvTimeout, "error", err)
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
			buf, err := dev.RecvBuf()
			if err != nil {
				cl.Debugw("receive buffer", "error", err)
				continue
			}

			var msg tower.FixtureToTower

			if err = proto.Unmarshal(buf, &msg); err != nil {
				cl.Debugw("unmarshal proto message", "error", err)
				continue
			}

			cl.Debugw("received FixtureToTower message")

			debugJB, err := json.Marshal(&msg)
			if err != nil {
				cl.Debugw("unmarshal FixtureToTower message", "error", err)
			} else {
				cl.Debugw("FixtureToTower message JSON", "message_json", string(debugJB))
			}

			// todo: stream to socket?
			event := ProtoMessage{
				Location: loc,
				Body:     buf,
			}

			jb, err := json.Marshal(event)
			if err != nil {
				cl.Warnw("marshal event to publish", "error", err)
				continue
			}

			if err := sock.PublishTo(loc, jb); err != nil {
				cl.Warnw("send event JSON", "error", err)
			}

			cl.Info("published FixtureToTower message")
		}
	}
}

// Start a proto stream to the socket
func (s *Stream) Start(ctx context.Context) chan struct{} {
	done := make(chan struct{})

	cl := s.logger.With("ws_addr", s.wsAddr)

	go func() {
		var wg sync.WaitGroup

		wg.Add(len(s.fixtures))

		for location, canConf := range s.fixtures {
			go loopForMessages(s.publisher, location, canConf, ctx, &wg, cl)
		}

		wg.Wait()
		close(done)
	}()

	return done
}

// Destroy the stream dealer
func (s *Stream) Destroy() {
}

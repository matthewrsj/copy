package protostream

import (
	"net/url"
	"time"

	"go.uber.org/zap"
)

const (
	// DefaultWebsocketAddress is used if the WithWebsocketAddress option is not passed to NewStream
	DefaultWebsocketAddress = "localhost:13160"
	// DefaultListenerAddress is the default address for the listener to point to
	DefaultListenerAddress = "localhost:13161"
	// WSEndpoint is the endpoint to which the streamer posts
	WSEndpoint = "/proto"
	// DefaultRecvTimeout is the default timeout to wait for a CAN message
	DefaultRecvTimeout = time.Second * 3
)

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
func WithWebsocketAddress(address, endpoint string) Option {
	return func(s *Stream) error {
		u := url.URL{Scheme: "ws", Host: address, Path: endpoint}
		s.wsAddress = u.String()

		sock, err := NewPublisher(s.wsAddress, "")
		if err != nil {
			return err
		}

		s.publisher = sock

		return nil
	}
}

// WithListenerAddress sets the listener address to this address and endpoint
func WithListenerAddress(address, endpoint string) Option {
	return func(s *Stream) error {
		u := url.URL{Scheme: "ws", Host: address, Path: endpoint}
		s.listenerAddress = u.String()

		return nil
	}
}

// WithRecvTimeout sets the CAN receive timeout
func WithRecvTimeout(d time.Duration) Option {
	return func(s *Stream) error {
		s.recvTimeout = d
		return nil
	}
}

// WithLogDirectory sets the log directory for alert logs
func WithLogDirectory(logDir string) Option {
	return func(s *Stream) error {
		s.logDir = logDir
		return nil
	}
}

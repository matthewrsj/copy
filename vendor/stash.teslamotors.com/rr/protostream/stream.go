// Package protostream provides a stream interface for tower proto messages for all configured towers
package protostream

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/go-socketcan/pkg/socketcan"
	tower "stash.teslamotors.com/rr/towerproto"
)

// Stream sends Messages over a socket for other processes to read and injects proto messages
// onto the CAN bus when published to.
type Stream struct {
	fixtures        map[string]CANConfig
	listenerAddress string
	wsAddress       string
	recvTimeout     time.Duration
	publisher       *Socket
	logger          *zap.SugaredLogger
}

// NewStream starts streaming Messages to the socket
func NewStream(opts ...Option) (*Stream, error) {
	s := &Stream{
		fixtures:    make(map[string]CANConfig),
		logger:      zap.NewExample().Sugar(),
		recvTimeout: DefaultRecvTimeout,
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return s, err
		}
	}

	if s.wsAddress == "" {
		if err := WithWebsocketAddress(DefaultWebsocketAddress, WSEndpoint)(s); err != nil {
			return s, err
		}
	}

	if s.listenerAddress == "" {
		if err := WithListenerAddress(DefaultListenerAddress, WSEndpoint)(s); err != nil {
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

func createDevice(can CANConfig) (socketcan.Interface, error) {
	dev, err := socketcan.NewIsotpInterface(can.Bus, can.RX, can.TX)
	if err != nil {
		return dev, fmt.Errorf("create new socketcan interface (bus: %s, rx: 0x%X, tx: 0x%X): %v", can.Bus, can.RX, can.TX, err)
	}

	if err = dev.SetCANFD(); err != nil {
		return dev, fmt.Errorf("set CANFD flags on socketcan interface (bus: %s, rx: 0x%X, tx: 0x%X): %v", can.Bus, can.RX, can.TX, err)
	}

	if err = dev.SetRecvTimeout(can.RecvTimeout); err != nil {
		return dev, fmt.Errorf("set recv timeout (%s) on socketcan interface (bus: %s, rx: 0x%X, tx: 0x%X): %v", can.RecvTimeout, can.Bus, can.RX, can.TX, err)
	}

	return dev, nil
}

func traceTowerToFixture(cl *zap.SugaredLogger, buf []byte) {
	var protoMsg tower.TowerToFixture
	if err := proto.Unmarshal(buf, &protoMsg); err != nil {
		cl.Debugw("unable to unmarshal injected message for logging", "error", err)
		return
	}

	jb, err := json.Marshal(&protoMsg)
	if err != nil {
		cl.Debugw("unable to marshal injected message to JSON for logging", "error", err)
	}

	cl.Debugw("sent TowerToFixture message", "json_message", string(jb))
}

func traceFixtureToTower(cl *zap.SugaredLogger, buf []byte) {
	var protoMsg tower.FixtureToTower
	if err := proto.Unmarshal(buf, &protoMsg); err != nil {
		cl.Debugw("unable to unmarshal FixtureToTower message for logging", "error", err)
		return
	}

	jb, err := json.Marshal(&protoMsg)
	if err != nil {
		cl.Debugw("unable to marshal FixtureToTower message to JSON for logging", "error", err)
	}

	cl.Debugw("received FixtureToTower message", "json_message", string(jb))
}

func loopForMessages(sock *Socket, inject <-chan *ProtoMessage, can CANConfig, ctx context.Context, wg *sync.WaitGroup, logger *zap.SugaredLogger) {
	defer wg.Done()

	cl := logger.With("fixture", can.NodeID, "can_bus", can.Bus, "can_rx", fmt.Sprintf("0x%X", can.RX), "can_tx", fmt.Sprintf("0x%X", can.TX))

	dev, err := createDevice(can)
	if err != nil {
		cl.Errorw("create new ISOTP interface", "error", err)
		return
	}

	defer func() {
		_ = dev.Close()
	}()

	for {
		select {
		case <-ctx.Done(): // done
			return
		case msg := <-inject: // write the message received from the controller
			cl.Info("proto message received for injection")

			if err := dev.SendBuf(msg.Body); err != nil {
				cl.Errorw("send buffer", "error", err)
				continue
			}

			go traceTowerToFixture(cl, msg.Body)
		default: // by default we read off the bus continuously
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

			go traceFixtureToTower(cl, buf)

			event := ProtoMessage{
				Location: can.NodeID,
				Body:     buf,
			}

			jb, err := json.Marshal(event)
			if err != nil {
				cl.Warnw("marshal event to publish", "error", err)
				continue
			}

			if err := sock.PublishTo(can.NodeID, jb); err != nil {
				cl.Warnw("send event JSON", "error", err)
			}

			cl.Info("published FixtureToTower message")
		}
	}
}

// rxInjectStream listens at listenerAddress for proto messages to inject onto the CAN bus. The listener here is
// is transient and created on every iteration because it is very possible for the publisher to go away
// (TODO: this might not be the best approach)
func rxInjectStream(listenerAddress string, inject chan<- *ProtoMessage, location string, ctx context.Context, wg *sync.WaitGroup, cl *zap.SugaredLogger) {
	defer wg.Done()

	var (
		sub *Socket
		err error
	)

	for {
		if err = backoff.Retry(
			func() error {
				sub, err = NewSubscriberWithSub(listenerAddress, location)
				if err != nil {
					cl.Debugw("create new subscriber", "error", err)
					return err
				}

				return nil
			},
			backoff.NewConstantBackOff(time.Second*5),
		); err != nil {
			cl.Errorw("create new subscriber", "error", err)
			continue
		}

		rxChan := sub.Listen()

		// TODO: determine if we need a timeout on this read in case the publisher went away
		select {
		case <-ctx.Done():
			return
		case msg := <-rxChan:
			var pm ProtoMessage
			if err := json.Unmarshal(msg.Msg.Body, &pm); err != nil {
				cl.Errorw("unmarshal msg body", "error", err)
				continue
			}

			inject <- &pm
		}

		sub.Quit() // transient, close the connection
	}
}

// Start a proto stream to the socket
func (s *Stream) Start(ctx context.Context) chan struct{} {
	done := make(chan struct{})

	cl := s.logger.With("ws_addr", s.wsAddress, "listener_addr", s.listenerAddress)

	go func() {
		defer close(done)

		var wg sync.WaitGroup

		for location, canConf := range s.fixtures {
			canConf.NodeID = location
			canConf.RecvTimeout = s.recvTimeout

			wg.Add(2) // +rxInjectStream, +loopForMessages

			inject := make(chan *ProtoMessage)
			defer close(inject) // defer the close here so it isn't prematurely closed by rxInjectStream

			// receive messages to write over CAN to the device
			go rxInjectStream(s.listenerAddress, inject, location, ctx, &wg, cl)

			// read messages from the CAN bus and publish
			//
			// This loop also does the actual CAN writing of messages received by rxInjectStream.
			// This allows us to serialize reads/writes.
			//
			// There could potentially be some lag between RXing the proto message in rxInjectStream
			// and actually writing because it will have to wait for this loop to complete a read.
			// There is a 3-second timeout on the read so that should be the maximum lag time. This
			// maximum lag time should only be encountered when there is nothing talking (and likely
			// nothing listening) on the bus anyways. Typical maximum lag time will be around 1 second
			// due to the TX rate (1 Hz) of the FXRs on the bus.
			go loopForMessages(s.publisher, inject, canConf, ctx, &wg, cl)
		}

		wg.Wait()
	}()

	return done
}

// Destroy the stream dealer
func (s *Stream) Destroy() {
}

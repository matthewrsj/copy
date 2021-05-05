package protostream

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	tower "stash.teslamotors.com/rr/towerproto"
)

// MockStream sends mock Messages over a socket for other processes to read and injects proto messages
// onto the CAN bus when published to.
type MockStream struct {
	fixtures        map[string]CANConfig
	listenerAddress string
	wsAddress       string
	recvTimeout     time.Duration
	publisher       *Socket
	metricsHandler  *MetricsHandler
	logger          *zap.SugaredLogger
	tcauxCol1       CANConfig
	tcauxCol2       CANConfig
}

// NewMockStream starts streaming Messages to the socket
func NewMockStream(opts ...Option) (*MockStream, error) {
	s := &Stream{
		fixtures: make(map[string]CANConfig),
		logger:   zap.NewExample().Sugar(),
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return &MockStream{}, err
		}
	}

	if s.wsAddress == "" {
		if err := WithWebsocketAddress(DefaultWebsocketAddress, WSEndpoint)(s); err != nil {
			return &MockStream{}, err
		}
	}

	if s.listenerAddress == "" {
		if err := WithListenerAddress(DefaultListenerAddress, WSEndpoint)(s); err != nil {
			return &MockStream{}, err
		}
	}

	ms := MockStream{
		fixtures:        s.fixtures,
		listenerAddress: s.listenerAddress,
		wsAddress:       s.wsAddress,
		recvTimeout:     s.recvTimeout,
		publisher:       s.publisher,
		metricsHandler:  s.metricsHandler,
		logger:          s.logger,
		tcauxCol1:       s.tcauxCol1,
		tcauxCol2:       s.tcauxCol2,
	}

	return &ms, nil
}

func mockReceiveMessagesFromCan(ctx context.Context, rxFromCAN chan<- *ProtoMessage, can CANConfig, cl *zap.SugaredLogger) {
	t := time.NewTicker(time.Second)

	var i int

	for {
		i++
		select {
		case <-ctx.Done(): // done
			return
		case <-t.C:
			var msg tower.FixtureToTower

			// TODO: generate message somehow
			msg.Info = &tower.Info{
				TrayBarcode:     "TRR-80A-00" + strings.ReplaceAll(can.NodeID, "-", "") + "-A",
				FixtureLocation: fmt.Sprintf("CM2-63010-%s", can.NodeID),
				RecipeName:      "test",
				RecipeVersion:   1,
				TransactionId:   "abcdefghijklmnopqrstuvwxyz0123456789",
			}

			msg.Content = &tower.FixtureToTower_Op{
				Op: &tower.FixtureOperational{
					Position:        tower.FixturePosition_FIXTURE_POSITION_OPEN,
					EquipmentStatus: tower.EquipmentStatus_EQUIPMENT_STATUS_IN_OPERATION,
					TrayPresent:     true,
				},
			}

			msg.GetOp().Status = tower.FixtureStatus_FIXTURE_STATUS_IDLE

			pb, err := proto.Marshal(&msg)
			if err != nil {
				cl.Debugw("unmarshal proto message", "error", err)
				continue
			}

			cl.Debugw("publishing mock message", "message", msg.String())

			rxFromCAN <- &ProtoMessage{
				Location:          can.NodeID,
				TimeStampUnixNano: time.Now().UnixNano(),
				Body:              pb,
			}
		}
	}
}

func mockLoopForMessages(ctx context.Context, sock *Socket, inject <-chan *ProtoMessage, can CANConfig, wg *sync.WaitGroup, logger *zap.SugaredLogger, taux bool, mh *MetricsHandler) {
	defer wg.Done()

	cl := logger.With("fixture", can.NodeID, "can_bus", can.Bus)

	rxFromCAN := make(chan *ProtoMessage)

	clOp := cl.With("can_rx", fmt.Sprintf("0x%X", can.RX), "can_tx", fmt.Sprintf("0x%X", can.TX))

	if taux {
		go mockReceiveTAUXMessagesFromCan(ctx, rxFromCAN, can, clOp)
	} else {
		clDiag := cl.With("can_rx", fmt.Sprintf("0x%X", can.RXDiag), "can_tx", fmt.Sprintf("0x%X", can.TXDiag))

		go mockReceiveMessagesFromCan(ctx, rxFromCAN, can, clOp)
		go mockReceiveMessagesFromCan(ctx, rxFromCAN, can, clDiag)
	}

	for {
		select {
		case <-ctx.Done(): // done
			return
		case msg := <-inject: // write the message received from the controller
			cl.Info("proto message received for injection")

			var protoMsg tower.TowerToFixture
			if err := proto.Unmarshal(msg.Body, &protoMsg); err != nil {
				cl.Errorw("unable to unmarshal injected message for logging", "error", err)
				return
			}

			mh.CountTowerToFixture()

			cl.Info("RX INJECT MOCKED", "message", protoMsg.String())
		case event := <-rxFromCAN:
			jb, err := json.Marshal(event)
			if err != nil {
				cl.Warnw("marshal event to publish", "error", err)
				continue
			}

			if err := sock.PublishTo(can.NodeID, jb); err != nil {
				cl.Warnw("send event JSON", "error", err)
			}

			mh.CountFixtureToTower()

			cl.Info("published FixtureToTower message")
		}
	}
}

// mockrxInjectStream listens at listenerAddress for proto messages to inject onto the CAN bus. The listener here is
// is transient and created on every iteration because it is very possible for the publisher to go away
func mockRXInjectStream(ctx context.Context, listenerAddress string, inject chan<- *ProtoMessage, location string, wg *sync.WaitGroup, cl *zap.SugaredLogger) {
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

		break
	}

	rxChan := sub.AlwaysListen()

	defer sub.Quit()

	var rxCount int

	for {
		select {
		case <-ctx.Done():
			return
		case msg := <-rxChan:
			rxCount++

			var pm ProtoMessage
			if err := json.Unmarshal(msg.Msg.Body, &pm); err != nil {
				cl.Errorw("unmarshal msg body", "error", err)
				continue
			}

			cl.Infow("injecting messasge to main stream", "msg", string(msg.Msg.Body), "rx_count", rxCount)
			inject <- &pm
		}
	}
}

// Start a proto stream to the socket
func (s *MockStream) Start(ctx context.Context) chan struct{} {
	done := make(chan struct{})

	cl := s.logger.With("ws_addr", s.wsAddress, "listener_addr", s.listenerAddress)

	go func() {
		defer close(done)

		var wg sync.WaitGroup

		wg.Add(1)

		go mockLoopForMessages(ctx, s.publisher, make(chan *ProtoMessage) /* not used */, s.tcauxCol1, &wg, cl, true, s.metricsHandler)

		for location, canConf := range s.fixtures {
			canConf.NodeID = location
			canConf.RecvTimeout = s.recvTimeout

			wg.Add(2) // +rxInjectStream, +loopForMessages

			inject := make(chan *ProtoMessage)
			defer close(inject) // defer the close here so it isn't prematurely closed by rxInjectStream

			// receive messages to write over CAN to the device
			go mockRXInjectStream(ctx, s.listenerAddress, inject, location, &wg, cl)

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
			go mockLoopForMessages(ctx, s.publisher, inject, canConf, &wg, cl, false, s.metricsHandler)
		}

		wg.Wait()
	}()

	return done
}

// Destroy the stream dealer
func (s *MockStream) Destroy() {
}

func mockReceiveTAUXMessagesFromCan(ctx context.Context, rxFromCAN chan<- *ProtoMessage, can CANConfig, cl *zap.SugaredLogger) {
	t := time.NewTicker(time.Second)

	var i int

	for {
		i++
		select {
		case <-ctx.Done(): // done
			return
		case <-t.C:
			var msg tower.TauxToTower

			msg.Content = &tower.TauxToTower_Op{
				Op: &tower.TauxOperational{
					Status:            tower.TauxStatus_TAUX_STATUS_ACTIVE,
					EnumerationStatus: tower.EnumerationStatus_ENUM_STATUS_OK,
					PowerCapacityW:    10,
					PowerInUseW:       2,
					PowerAvailableW:   8,
				},
			}

			pb, err := proto.Marshal(&msg)
			if err != nil {
				cl.Debugw("unmarshal proto message", "error", err)
				continue
			}

			cl.Debugw("publishing mock message", "message", msg.String())

			rxFromCAN <- &ProtoMessage{
				Location:          can.NodeID,
				TimeStampUnixNano: time.Now().UnixNano(),
				Body:              pb,
			}
		}
	}
}

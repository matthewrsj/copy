package cdcontroller

import (
	"context"
	"fmt"
	"io"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	asrsapi "stash.teslamotors.com/cas/asrs/idl/src"
	terminal "stash.teslamotors.com/cas/asrs/terminal/server"
)

// InputFeeds allows the user to feed operations to the server to be sent to clients
type InputFeeds struct {
	LoadOp   chan *asrsapi.LoadOperation
	UnloadOp chan *asrsapi.UnloadOperation
	ALM      chan *asrsapi.TerminalAlarm
}

// TerminalServer handles incoming client connections
type TerminalServer struct {
	*terminal.Server

	prodAM, testAM *AisleManager
	aisles         map[string]*Aisle

	Feeds InputFeeds
}

// NewTerminalServer returns a new TerminalServer with allocated feeds and pool
func NewTerminalServer(prodAM, testAM *AisleManager, aisles map[string]*Aisle) *TerminalServer {
	return &TerminalServer{
		prodAM: prodAM,
		testAM: testAM,
		aisles: aisles,
		Feeds: InputFeeds{
			LoadOp:   make(chan *asrsapi.LoadOperation),
			UnloadOp: make(chan *asrsapi.UnloadOperation),
			ALM:      make(chan *asrsapi.TerminalAlarm),
		},
	}
}

const _unidentified = "unidentified"

// LoadOperations are the heart of this service; bidirectional streams which convey arrival of trays, confirmation
// of tray positions, transfer requests, and completions.
// nolint:gocognit // not too bad
func (s *TerminalServer) LoadOperations(g asrsapi.Terminal_LoadOperationsServer) error {
	remoteEnd := _unidentified
	ctx := g.Context()

	pr, ok := peer.FromContext(ctx)
	if ok {
		remoteEnd = pr.Addr.String()
	}

	metricsHolder := s.GetMetricsHolder()
	trace := func(loadOp *asrsapi.LoadOperation, msg string, err error, direction string, result string) {
		if metricsHolder != nil {
			metricsHolder.LoadOperationCounter.WithLabelValues(
				direction,
				result,
				remoteEnd,
				fmt.Sprint(loadOp.GetAck()),
				loadOp.GetState().GetState().String(),
				loadOp.GetState().GetStateType().String(),
				loadOp.GetState().GetStatus().String()).Inc()
		}

		if err != nil {
			s.Errorw(msg, "loadOp", loadOp, "remote", remoteEnd, "error", err)
		} else {
			s.Debugw(msg, "loadOp", loadOp, "remote", remoteEnd)
		}
	}

	if s.Feeds.LoadOp != nil {
		go func(ctx context.Context, inputFeed chan *asrsapi.LoadOperation) {
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-inputFeed:
					if !ok {
						return
					}

					if err := g.Send(msg); err != nil {
						s.Error(err)
					}
				}
			}
		}(ctx, s.Feeds.LoadOp)
	}

	// We start with a super simple implementation... i.e. we simply acknowledge and message we receive.
	// We want to add input feeder too.
	for {
		in, err := g.Recv()
		if err != nil {
			st, ok := status.FromError(err)
			if err == io.EOF || (ok && st.Code() == codes.Canceled) {
				s.Debugw("LoadOperation: reader goroutine closing down, stream closed", "remote",
					remoteEnd)
			} else {
				s.Errorw("LoadOperation: reader goroutine closing down, stream closed", "remote",
					remoteEnd, "error", err)
			}

			return err
		}

		trace(in, "LoadOperation rxed from client", nil, asrsapi.DirRx, asrsapi.ResOk)

		if !in.Ack {
			tx := *in
			tx.Ack = true
			tx.Conversation = s.BuildConversationHeader(tx.GetConversation().Id())

			if err := g.Send(&tx); err != nil {
				s.Errorw("unable to acknowledge", "error", err)
				continue
			}

			go func() {
				if err := handleIncomingLoad(g, s.SugaredLogger, s.prodAM, s.testAM, s.aisles, in); err != nil {
					s.Error(err)
				}
			}()

			trace(&tx, "LoadOperation txed to client", nil, asrsapi.DirTx, asrsapi.ResOk)
		}
	}
}

// UnloadOperations are streamed by Conductor to WMS in order to interrupt operations and provide appropriate
// mitigating action at a location (e.g deploy crane with fire suppressing equipment to a tray location, and on a
// subsequent trigger, activate fire extinguisher). Conductor does this by indicating a desired state: Deployed,
// Mitigating, Released to WMS.
//
// UnloadOperation are returned on the stream from WMS to Conductor in order to advise Conductor of
// progress in mitigating an unloadOpent operation e.g. Deploying, Deployed, Mitigating, Released.
// nolint:gocognit,gocyclo // no easy way to simplify this
func (s *TerminalServer) UnloadOperations(g asrsapi.Terminal_UnloadOperationsServer) error {
	remoteEnd := _unidentified
	ctx := g.Context()

	pr, ok := peer.FromContext(ctx)
	if ok {
		remoteEnd = pr.Addr.String()
	}

	metricsHolder := s.GetMetricsHolder()
	trace := func(ulo *asrsapi.UnloadOperation, msg string, err error, direction string, result string) {
		if metricsHolder != nil {
			metricsHolder.UnloadOperationCounter.WithLabelValues(
				direction,
				result,
				remoteEnd,
				fmt.Sprint(ulo.GetAck()),
				ulo.GetState().GetState().String(),
				ulo.GetState().GetStateType().String(),
				ulo.GetState().GetStatus().String()).Inc()
		}

		if err != nil {
			s.Errorw(msg, "ulo", ulo, "remote", remoteEnd, "error", err)
		} else {
			s.Debugw(msg, "ulo", ulo, "remote", remoteEnd)
		}
	}

	if s.Feeds.UnloadOp != nil {
		go func(ctx context.Context, inputFeed chan *asrsapi.UnloadOperation) {
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-inputFeed:
					if !ok {
						return
					}

					if err := g.Send(msg); err != nil {
						s.Error("g.Send", err)
					}
				}
			}
		}(ctx, s.Feeds.UnloadOp)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		in, err := g.Recv()
		if err != nil {
			st, ok := status.FromError(err)
			if err == io.EOF || (ok && st.Code() == codes.Canceled) {
				s.Debugw("UnloadOperation: reader goroutine closing down, stream closed", "remote",
					remoteEnd)
			} else {
				s.Errorw("UnloadOperation: reader goroutine closing down, stream closed", "remote",
					remoteEnd, "error", err)
			}

			return err
		}

		trace(in, "UnloadOperation rxed from client", nil, asrsapi.DirRx, asrsapi.ResOk)

		if in != nil && !in.Ack {
			// beware this is a shallow copy... and deep fields modified will be on structure in dispatch if
			// not explicit new copy is taken
			tx := *in
			tx.Ack = true
			tx.Conversation = s.BuildConversationHeader(tx.GetConversation().Id())

			if err = g.Send(&tx); err != nil {
				trace(&tx, "UnloadOperation txed to client", err, asrsapi.DirTx, asrsapi.ResSendFailed)
			}

			go func() {
				s.Info("received incoming unload, responding to unload")

				if err := handleIncomingUnload(g, in); err != nil {
					s.Error(err)
				}
			}()

			trace(&tx, "UnloadOperation txed to client", nil, asrsapi.DirTx, asrsapi.ResOk)
		}
	}
}

// Alarms are server side streamed from WMS to conductor using this service. The client side will return the alarm
// in the client side stream as shown as an acknowledgement of the fault (but see migration note).
// nolint:gocognit // no simple way to break this up
func (s *TerminalServer) Alarms(g asrsapi.Terminal_AlarmsServer) error {
	remoteEnd := _unidentified
	ctx := g.Context()

	pr, ok := peer.FromContext(ctx)
	if ok {
		remoteEnd = pr.Addr.String()
	}

	metricsHolder := s.GetMetricsHolder()
	trace := func(alarm *asrsapi.TerminalAlarm, msg string, _ error, direction string, result string) {
		loc, err := asrsapi.LocationToString(alarm.GetLocation())
		if err != nil {
			s.Errorw(msg, "LocationToString", err)
			return
		}

		if metricsHolder != nil {
			metricsHolder.AlarmCounter.WithLabelValues(
				direction,
				result,
				remoteEnd,
				alarm.GetStatus().String(),
				alarm.GetLevel().String(),
				loc,
			).Inc()
		}

		s.Debugw(msg, "loadOp", alarm, "remote", remoteEnd)
	}

	if s.Feeds.ALM != nil {
		go func(ctx context.Context, inputFeed chan *asrsapi.TerminalAlarm) {
			for {
				select {
				case <-ctx.Done():
					return
				case msg, ok := <-inputFeed:
					if !ok {
						return
					}

					if err := g.Send(msg); err != nil {
						// log, but keep sending
						s.Error(err)
					}
				}
			}
		}(ctx, s.Feeds.ALM)
	}

	for {
		in, err := g.Recv()
		if err != nil {
			st, ok := status.FromError(err)
			if err == io.EOF || (ok && st.Code() == codes.Canceled) {
				s.Debugw("Alarm: reader goroutine closing down, stream closed", "remote", remoteEnd)
			} else {
				s.Errorw("Alarm: reader goroutine closing down, stream closed", "remote", remoteEnd, "error", err)
			}

			return err
		}

		trace(in, "Alarm ack rxed from client", nil, asrsapi.DirRx, asrsapi.ResOk)
	}
}

// Hellos service allows each end (client and server) to initiate an application level exchange of hello
// messages. Each end can control whether and how often it wishes to validate the roundtrip to the other endpoint.
// When it wishes to do so, it would launch a Hello message in the stream with echo_request set to true, and nonce set
// to a random number. The other end, when receiving a message with echo_request set to true, must reply in a timely
// way with echo_request set to false and nonce copied from the original request.
//
// MIGRATION: This serves the purpose of 4 messages in original spec DateAndTime Req/Data and HeartBeat Req/Ack.
func (s *TerminalServer) Hellos(g asrsapi.Terminal_HellosServer) error {
	ctx := g.Context()
	remoteEnd, p := s.RunHellos(g)
	s.Infow("hellos: server new stream", "remote", remoteEnd)

	// Watch notifications and exit.
outerLoop:
	// nolint:gosimple // need the ok variable to check for close
	for {
		select {
		case _, ok := <-p.Notify:
			if !ok {
				// handler has shut down
				s.Infow("hellos: server child peerTracker closed down", "remote", remoteEnd)
				break outerLoop
			}
			// Notification -server/cmd/implementation.go:64 look at state... just for fun
			st, last, remoteName := p.GetState()
			s.Infow(
				"hellos: server detected state change",
				"since", last,
				"state", st,
				"remote", remoteEnd,
				"remote_name", remoteName,
			)
		}
	}

	s.Infow("hellos: server exiting stream", "remote", remoteEnd)

	return ctx.Err()
}

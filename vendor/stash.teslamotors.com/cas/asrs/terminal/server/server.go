package terminal

import (
	"context"
	"errors"
	"net"
	"sync"
	"time"

	"github.com/cenkalti/backoff"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/grpc-ecosystem/go-grpc-prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/reflection"
	asrsapi "stash.teslamotors.com/cas/asrs/idl/src"
	"stash.teslamotors.com/cas/asrs/terminal/common/metrics"
)

const (
	// For the meaning of constants look at comment where they are used.
	// Note these are overridable when application kicks off package using WithGRPCServerOptions().
	defaultMaxConcurrentStreams           = 10
	defaultInactivityTriggeredPingSeconds = 20
	defaultTimeoutAfterPingSeconds        = 5
	defaultMinTimeEnforcedMilliseconds    = 1000
)

type Server struct {
	implementation asrsapi.TerminalServer
	localListener  net.Listener
	grpcServer     *grpc.Server
	*zap.SugaredLogger
	cfg *ServerConfig
}

// NewServer is used to skeleton set up the handlers for the ASRS server taking care of the basics like logging,
// metrics middleware...
func NewServer(cfg ServerConfig, opts ...ServerConfigOption) (*Server, error) {

	for _, opt := range opts {
		err := opt(&cfg)
		if err != nil {
			// Logging may not be setup yet. Simply return error.
			return nil, asrsErrorf("applied option error, %v", err)
		}
	}

	if cfg.logger == nil {
		logger, err := DefaultZapLoggerConfig().Build()
		if err != nil {
			return nil, asrsErrorf("failed to set up logging")
		}
		cfg.logger = logger.Sugar()
	}

	if cfg.localAddress == "" {
		// Default to a localhost address
		cfg.localAddress = ":13174"
	}
	s := &Server{cfg: &cfg}

	// If no implementation is provided, we register outselves as implementation... a very simple implementation
	// which largely logs messages received and mimics desired state too.
	if cfg.implementation == nil {
		s.implementation = s
	} else {
		s.implementation = cfg.implementation
	}

	s.SugaredLogger = s.cfg.logger.With(zap.String("pkg", "terminal_server"))

	s.Info("hello")

	return s, nil
}

// *Server.Start kicks of handling of services associated with server and returns
// on fatal error, or after context cancellation. It is up to the caller to determine how
// to handle fatal error (i.e. start all over or fail to the orchestrator - latter is
// preferred for real deployments).
func (s *Server) Start(ctx context.Context) {
	var listener net.Listener

	// retry acquiring local port to convey common case e.g. in test where quick
	// cycling of application will not be able to acquire local socket because
	// OS has not yet released it
	err := backoff.Retry(
		func() error {
			var err error
			listener, err = net.Listen("tcp", s.cfg.localAddress)
			return err
		},
		backoff.WithMaxRetries(backoff.NewExponentialBackOff(), 3),
	)

	if err != nil {
		err = asrsErrorf("failed to acquire local endpoint for socket, %v", err)
		s.Errorw("set up listening socket", "error", err)
		return
	}

	s.localListener = listener
	s.Infow("listening", "address", s.cfg.localAddress)

	var wg sync.WaitGroup
	wg.Add(1)
	go s.run(ctx, &wg)
	wg.Wait()
}

// Server.GetMetricsHolder exports the metrics holder to an external
// implementation of WmsServer can access the subset of counters which are
// common and increment them consistently.
func (s *Server) GetMetricsHolder() *metrics.TerminalMetricsHolder {
	return s.cfg.metrics
}

func (s *Server) run(ctx context.Context, wg *sync.WaitGroup) {
	defer func() {
		s.Info("goodbye")
		wg.Done()
	}()

	streamInterceptorChain := []grpc.StreamServerInterceptor{}
	if s.cfg.verboseLogging {
		streamInterceptorChain = append(streamInterceptorChain,
			grpc_zap.StreamServerInterceptor(
				s.SugaredLogger.Named("GRPC_S").Desugar(),
				// All results are forced to debug level
				grpc_zap.WithLevels(func(code codes.Code) zapcore.Level { return zapcore.DebugLevel })))
	}

	if s.cfg.metrics != nil {
		streamInterceptorChain = append(streamInterceptorChain, grpc_prometheus.StreamServerInterceptor)
	}

	// Setup the default server options, all of which can be overwritten. Default server side options
	// are aggressive and assume good connectivity between nodes. These options can be overridden
	// in MakeASRSServer configuration options.
	options := []grpc.ServerOption{
		grpc.MaxConcurrentStreams(defaultMaxConcurrentStreams), // aggressive max concurrent stream per transport
		grpc.KeepaliveParams(keepalive.ServerParameters{ // similarly aggressive attempt to track connection liveness
			Time:    time.Second * defaultInactivityTriggeredPingSeconds, // 10 seconds with no activity, kick client for ping
			Timeout: time.Second * defaultTimeoutAfterPingSeconds,        // no ping after the next 10 seconds, then close connection.
		}),
		// control how often a client can send a keepalive, and whether to allow keepalives with no streams.
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             time.Millisecond * defaultMinTimeEnforcedMilliseconds,
			PermitWithoutStream: true,
		}),
		grpc_middleware.WithStreamServerChain(streamInterceptorChain...),
	}

	//
	// Append configured options so they can overwrite the defaults too.
	options = append(options, s.cfg.grpcServerOptions...)

	s.grpcServer = grpc.NewServer(options...)
	reflection.Register(s.grpcServer)
	asrsapi.RegisterTerminalServer(s.grpcServer, s.implementation)

	s.Infow("gRPCServer_starting_up")

	go func() {
		select {
		case <-ctx.Done():
			s.Debug("gRPCServer_graceful_shutdown_requested")
			s.grpcServer.Stop()
		}
	}()

	for {
		boff := backoff.NewExponentialBackOff()
		boff.MaxElapsedTime = 0 // never give up
		boff.Reset()
		err := backoff.RetryNotify(
			func() error {
				return s.grpcServer.Serve(s.localListener)
			},
			boff,
			func(err error, next time.Duration) {
				err = asrsErrorf("gRPC server stopped serving, %v]", err)
				s.Errorw("gRPCServer_exit", "retry_restart_in", next, "error", err)
			})
		if err != nil {
			s.Errorw("gRPCServer_shut_down_unexpectedly", "error", err)
		} else {
			s.Info("gRPCServer_shut_down_gracefully")
			return
		}
	}
}

func (s *Server) LoadOperations(server asrsapi.Terminal_LoadOperationsServer) error {
	return errors.New("no default implementation")
}

func (s *Server) UnloadOperations(server asrsapi.Terminal_UnloadOperationsServer) error {
	return errors.New("no default implementation")
}

func (s *Server) Alarms(g asrsapi.Terminal_AlarmsServer) error {
	// We do not yet issue alarms. We do nothing until service is cancelled.
	ctx := g.Context()
	<-ctx.Done()
	return nil
}

// Hello service allows each end (client and server) to initiate an application level exchange of hello
// messages. Each end can control whether and how often it wishes to validate the roundtrip to the other endpoint.
// When it wishes to do so, it would launch a Hello message in the stream with echo_request set to true, and nonce set
// to a random number. The other end, when receiving a message with echo_request set to true, must reply in a timely
// way with echo_request set to false and nonce copied from the original request.
func (s *Server) Hellos(g asrsapi.Terminal_HellosServer) error {

	ctx := g.Context()
	remoteEnd, p := s.RunHellos(g)
	s.Debugw("hellos: server new stream", "remote", remoteEnd)

	// Watch notifications and exit.
outerLoop:
	for {
		select {
		case _, ok := <-p.Notify:
			if !ok {
				// handler has shut down
				s.Infow("hellos: server child peerTracker closed down", "remote", remoteEnd)
				break outerLoop
			}
			// Notification - look at state... just for fun
			st, last, discovered := p.GetState()
			s.Infow("hellos: server detected state change", "since", last, "state", st,
				"remote", remoteEnd, "discovered", discovered)
		}
	}

	s.Debugw("hellos: server exiting stream", "remote", remoteEnd)
	return ctx.Err()
}

func (s *Server) RunHellos(g asrsapi.Wms_HellosServer) (string, *asrsapi.PeerTracker) {

	ctx := g.Context()
	remoteEnd, remoteName := asrsapi.HelloRemoteAddressAndNameFromContext(ctx)

	pt := asrsapi.NewPeerTracker(
		g, s.cfg.Line, s.cfg.ServerName,
		s.cfg.helloTimeout, s.cfg.helloPeriodMultiplier,
		s.cfg.metrics.GetHelloMetricsHolder(), s.SugaredLogger)

	go pt.Start(ctx, remoteEnd, remoteName)

	return remoteName, pt
}

func (s *Server) BuildConversationHeader(mid asrsapi.MessageId) *asrsapi.Conversation {
	return asrsapi.BuildConversationHeader(s.cfg.Line, s.cfg.ServerName, mid)
}

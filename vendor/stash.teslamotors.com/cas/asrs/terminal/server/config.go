package terminal

import (
	"time"

	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"google.golang.org/grpc"
	asrsapi "stash.teslamotors.com/cas/asrs/idl/src"
	"stash.teslamotors.com/cas/asrs/terminal/common/metrics"
)

type ServerConfig struct {
	Line       string
	ServerName string
	// hello service: launch hellos on ticker with this period
	helloPeriodMultiplier uint
	helloTimeout          time.Duration
	// overridden implementation of server stream functions
	implementation asrsapi.TerminalServer
	// Local address to listen on.
	localAddress string
	// Logging setup
	logger         *zap.SugaredLogger
	verboseLogging bool
	// Metrics setup
	metrics *metrics.TerminalMetricsHolder
	// grpc server side options
	grpcServerOptions []grpc.ServerOption
}

type ServerConfigOption func(*ServerConfig) error

func WithLocalAddress(address string) ServerConfigOption {
	return func(s *ServerConfig) error {
		s.localAddress = address
		return nil
	}
}

// WithLogger option is invoked by the application to provide a customised zap logger option, or to disable logging.
// The ServerConfigOption returned by WithLogger is passed in to NewServer to control logging; e.g. to provide
// a preconfigured application logger. If logger passed in is nil, logging in the package is disabled.
//
// If WithLogger option is not passed in, package uses its own configured zap logger.
//
// Finally, if application wishes to derive its logger as some variant of the default logger, application can invoke
// DefaultZapLoggerConfig() to fetch a default logger configuration. It can use that configuration (modified as necessary)
// to build a new logger directly through zap library. That new logger can then be passed into WithLogger to generate
// the appropriate node option.
//
// verboseLogging controls whether package redirects underlying gprc middleware logging to zap log and includes
// ultra low level debugging messages including keepalives. This makes debug very noisy, and unless in depth low level
// message troubleshooting is required, verboseLogging should be set to false.
func WithLogger(logger *zap.Logger, verboseLogging bool) ServerConfigOption {
	return func(s *ServerConfig) error {
		if logger != nil {
			s.logger = logger.Sugar()
		} else {
			s.logger = zap.NewNop().Sugar()
			return nil
		}
		s.verboseLogging = verboseLogging

		if s.verboseLogging {
			grpc_zap.ReplaceGrpcLoggerV2(s.logger.Desugar().Named("grpc"))
		}

		return nil
	}
}

// WithMetrics option used with NewServer to specify metrics registry we should count in. Argument namespace specifies
// the namespace for the metrics. This is useful if the application prefixes all its metrics with a prefix. For
// e.g. if namespace is 'foo', then all package metrics will be prefixed with 'foo.asrsserver.'. Argument `detailed`
// controls whether detailed (and more expensive) metrics are tracked (e.g. grpc latency distribution).
// If nil is passed in for the registry, the default registry prometheus.DefaultRegisterer is used. Do note that
// the package does not setup serving metrics; that is up to the application.
func WithMetrics(registry *prometheus.Registry, namespace string, detailed bool) ServerConfigOption {
	return func(s *ServerConfig) error {

		if namespace == "" {
			namespace = "asrs"
		}

		s.metrics = metrics.NewTerminalMetricsHolder(registry, namespace, detailed)
		return nil
	}
}

// WithGRPCServerOptions sets up server side gRPC options. These server options will be merged in with default options,
// with default options overwritten if provided here. Server side options could be used, for example, to set up mutually
// authenticated TLS protection of exchanges with other nodes.
func WithGRPCServerOptions(opts []grpc.ServerOption) ServerConfigOption {
	return func(s *ServerConfig) error {
		s.grpcServerOptions = opts
		return nil
	}
}

// WithImplementation allows the application to provide its own WMSServer implementation. This is particularly
// useful in integration testing in that we can wrap around or completely replace implementation to test specific
// kinds of behaviour.
func WithImplementation(imp asrsapi.TerminalServer) ServerConfigOption {
	return func(s *ServerConfig) error {
		s.implementation = imp
		return nil
	}
}

// WithHellos sets up a server which periodically sends hellos to clients. Timeout dictates how long we are willing to
// wait for response, and multiplier is the period between echo request launches expressed as a multiple of timeouts.
func WithHellos(timeout time.Duration, multiplier uint) ServerConfigOption {
	return func(s *ServerConfig) error {
		s.helloTimeout = timeout
		s.helloPeriodMultiplier = multiplier
		return nil
	}
}

//
// DefaultZapLoggerConfig provides a production logger configuration (logs Info and above, JSON to stderr, with
// stacktrace, caller and sampling disabled) which can be customised by application to produce its own logger based
// on the package configuration. Any logger provided by the application will also have its name extended by the
// package to clearly identify that log message comes from package.
func DefaultZapLoggerConfig() zap.Config {

	lcfg := zap.NewProductionConfig()
	lcfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	lcfg.DisableStacktrace = false
	lcfg.DisableCaller = true
	lcfg.Sampling = nil
	return lcfg
}

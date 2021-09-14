// Package tabitha provides the API for testers to expose to allow for remote management of various options.
package tabitha

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

// Handler is the core of tabitha, it provides the API functionality
type Handler struct {
	l    *zap.Logger
	port string
	mux  sync.RWMutex
	ctx  context.Context

	// tester environment variables
	tester testerEnv

	// features allowed for this instance
	features featureSet

	// shutdown is closed when the server exits
	shutdown chan struct{}
}

// Configuration types
const (
	ConfigTypeDefault = iota
	ConfigTypeJSON
	ConfigTypeYAML
)

type testerEnv struct {
	name       string
	version    string
	gitsummary string

	fwDir              string
	fwdlUser, fwdlPass string
	fwdlWriteFName     string

	configPath string
	configType int
	config     interface{}
}

type featureSet struct {
	updateViaRestart bool
}

// Option sets the configuration on the Handler when creating a new object
type Option func(*Handler)

// WithPort sets the port on the Handler
func WithPort(port string) Option {
	return func(h *Handler) {
		h.port = port
	}
}

// WithContext sets the context on the Handler
func WithContext(ctx context.Context) Option {
	return func(h *Handler) {
		h.ctx = ctx
	}
}

// WithLocalFirmwareDirectory sets the local firmware path
func WithLocalFirmwareDirectory(path string) Option {
	return func(h *Handler) {
		h.tester.fwDir = path
	}
}

// WithLocalConfiguration sets the pointer to the local configuration object
func WithLocalConfiguration(configPath string, configType int, configuration interface{}) Option {
	return func(h *Handler) {
		h.tester.configPath = configPath
		h.tester.config = configuration
		h.tester.configType = configType
	}
}

// WithTesterGitSummary sets the tester gitsummary
func WithTesterGitSummary(githash string) Option {
	return func(h *Handler) {
		h.tester.gitsummary = githash
	}
}

// WithTesterName sets the tester name
func WithTesterName(name string) Option {
	return func(h *Handler) {
		h.tester.name = name
	}
}

// WithTesterVersion sets the tester version
func WithTesterVersion(version string) Option {
	return func(h *Handler) {
		h.tester.version = version
	}
}

// WithLogger sets the logger on the handler
func WithLogger(logger *zap.Logger) Option {
	return func(h *Handler) {
		h.l = logger
	}
}

// WithFeatureUpdateViaRestart sets the update via restart feature
func WithFeatureUpdateViaRestart() Option {
	return func(h *Handler) {
		h.features.updateViaRestart = true
	}
}

// environment variables to be used as defaults for configuration options if no overrides are provided
const (
	ENVTABFWDLUser = "TAB_FWDL_USER"
	ENVTABFWDLPass = "TAB_FWDL_PASS"
)

// WithFirmwareDownloadCredentials sets the credentials to use when downloading firmware
// If not specified tabitha will use "TAB_FWDL_USER" and "TAB_FWDL_PASS" from the environment.
func WithFirmwareDownloadCredentials(user, pass string) Option {
	return func(h *Handler) {
		h.tester.fwdlUser = user
		h.tester.fwdlPass = pass
	}
}

// WithFirmwareDownloadWriteFileName sets the write filename for the downloaded firmware packages.
// This is the filepath the tester expects the firmware to be at once downloaded. After download the
// firmware file is copied to this filepath name under the configured firmware directory. If not set no
// copy is done after downloading the original filename.
func WithFirmwareDownloadWriteFileName(path string) Option {
	return func(h *Handler) {
		h.tester.fwdlWriteFName = path
	}
}

// NewHandler returns a configured *Handler
func NewHandler(opts ...Option) *Handler {
	h := &Handler{
		l:    zap.NewExample(),
		mux:  sync.RWMutex{},
		ctx:  context.Background(),
		port: DefaultPort,
		tester: testerEnv{
			name:       NameUnknown,
			version:    VersionUnknown,
			gitsummary: GitHashUnknown,
			fwdlUser:   os.Getenv(ENVTABFWDLUser),
			fwdlPass:   os.Getenv(ENVTABFWDLPass),
		},
		shutdown: make(chan struct{}),
	}

	for _, opt := range opts {
		opt(h)
	}

	h.l = h.l.Named("tabitha").With(zap.String("tabitha_tester_name", h.tester.name))

	return h
}

// ServeNewHandler creates and serves a new Handler
func ServeNewHandler(opts ...Option) {
	h := NewHandler(opts...)
	h.Serve()
}

// Serve serves the http Server for updating various configuration files and managing the tester
func (h *Handler) Serve() {
	router := mux.NewRouter()

	router.HandleFunc(FirmwareUpdatesEndpoint, h.HandleIncomingFirmwarePackages()).Methods(http.MethodPost, http.MethodGet)
	router.HandleFunc(ConfigurationGetEndpoint, h.HandleIncomingConfigurationRequest()).Methods(http.MethodGet)
	router.HandleFunc(ConfigurationPostEndpoint, h.HandleIncomingConfigurationFiles()).Methods(http.MethodPost)
	router.HandleFunc(SelfUpdateEndpoint, h.HandleUpdateRequests()).Methods(http.MethodPost)
	router.HandleFunc(SelfHealthEndpoint, h.HandleHealthRequests()).Methods(http.MethodGet)

	srv := http.Server{
		Addr:    "0.0.0.0:" + h.port,
		Handler: router,
	}

	go func() {
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			h.l.Error("ListenAndServe", zap.Error(err))
		}
	}()

	h.l.Info("listening", zap.String("addr", srv.Addr))

	defer func() {
		// nolint:govet // don't need to cancel this, as the timeout is respected by Shutdown
		to, _ := context.WithTimeout(context.Background(), time.Second*5)
		if err := srv.Shutdown(to); err != nil {
			h.l.Error("unable to gracefully shut down server", zap.Error(err))
		}

		close(h.shutdown)
	}()

	<-h.ctx.Done()
}

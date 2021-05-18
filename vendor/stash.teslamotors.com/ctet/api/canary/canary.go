// Package canary provides standard canary reporting functionality via a REST endpoint
package canary

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"stash.teslamotors.com/ctet/api/internal/apilog"
)

// CanaryEndpoint is the endpoint at which the canary will be hosted
const CanaryEndpoint = "/canary"

// Versions contains all the version information of towercontroller
type Versions struct {
	GitCommit  string `json:"git_commit"`
	GitBranch  string `json:"git_branch"`
	GitState   string `json:"git_state"`
	GitSummary string `json:"git_summary"`
	BuildDate  string `json:"build_date"`
	Version    string `json:"version"`
}

// handler is responsible for serving the canary endpoint
type handler struct {
	logger      *zap.Logger
	corsAllowed bool
	callback    func() interface{}
	versions    Versions
}

// Option sets a configuration option on the handler
type Option func(*handler)

// WithLogger sets the zap logger for the handler
func WithLogger(lg *zap.Logger) Option {
	return func(h *handler) {
		h.logger = lg
	}
}

// WithCORSAllowed prohibits CORS on all requests. Default is allowed.
func WithCORSAllowed() Option {
	return func(h *handler) {
		h.corsAllowed = false
	}
}

// WithCallbackFunc sets the callback to be called on the data for every request
func WithCallbackFunc(cb func() interface{}) Option {
	return func(h *handler) {
		h.callback = cb
	}
}

// WithVersions sets the versions for the handler
func WithVersions(v Versions) Option {
	return func(h *handler) {
		h.versions = v
	}
}

// newHandler returns a new handler object
func newHandler(opts ...Option) *handler {
	h := handler{}

	for _, opt := range opts {
		opt(&h)
	}

	return &h
}

// CanaryResponse is the response sent to a request to the canary endpoint
type CanaryResponse struct {
	Data        interface{} `json:"data,omitempty"`
	VersionInfo Versions    `json:"versions"`
}

func allowCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, GET, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
}

// NewHandlerFunc returns the HTTP handler function for incoming GET requests
func NewHandlerFunc(opts ...Option) http.HandlerFunc {
	h := newHandler(opts...)

	return func(w http.ResponseWriter, r *http.Request) {
		if h.corsAllowed {
			apilog.Log(h.logger, zap.DebugLevel, "enabling_cors_requests")
			allowCORS(w)
		}

		apilog.Log(h.logger, zap.InfoLevel, "request_to_endpoint", zap.String("endpoint", CanaryEndpoint))

		if r.Method != http.MethodGet {
			apilog.HTTPError(h.logger, w, http.StatusMethodNotAllowed, "unsupported_method", zap.String("method", r.Method))

			return
		}

		w.Header().Add("Content-Type", "application/json")

		var data interface{}

		if h.callback != nil {
			apilog.Log(h.logger, zapcore.InfoLevel, "calling_callback_function")
			data = h.callback()
		}

		cr := CanaryResponse{
			Data:        data,
			VersionInfo: h.versions,
		}

		jb, err := json.Marshal(cr)
		if err != nil {
			apilog.HTTPError(h.logger, w, http.StatusInternalServerError, "unable_to_marshal_canary_response", zap.Error(err))

			return
		}

		if _, err = w.Write(jb); err != nil {
			apilog.HTTPError(h.logger, w, http.StatusInternalServerError, "unable_to_write_canary_response", zap.Error(err))

			return
		}

		apilog.Log(h.logger, zapcore.InfoLevel, "responded_to_request", zap.String("body", string(jb)))
	}
}

package tabitha

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"

	"github.com/go-yaml/yaml"
	"github.com/matthewrsj/copy"
	"go.uber.org/zap"
)

// DefaultPort is the default port at which the Handler listens
const DefaultPort = "49000"

// endpoints exposed for tester management
const (
	FirmwareUpdatesEndpoint   = "/firmware"
	ConfigurationGetEndpoint  = "/configuration"
	ConfigurationPostEndpoint = "/newconfiguration"
	SelfUpdateEndpoint        = "/update"
	SelfHealthEndpoint        = "/health"
)

// FirmwareUpdate specifies the remote from which to pull the update
type FirmwareUpdate struct {
	Remote string `json:"remote"`
}

// HandleIncomingFirmwarePackages handles incoming requests to update local firmware packages
func (h *Handler) HandleIncomingFirmwarePackages() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if HandleCORS(w, r) {
			return
		}

		l := endpointLogger(h.l, r)

		if h.tester.fwDir == "" {
			http.Error(w, "tester firmware updates not allowed for this tester", http.StatusNotImplemented)
			l.Error("tester firmware updates not allowed for this tester")

			return
		}

		rb, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("unable to read request body", zap.Error(err))

			return
		}

		var fwu FirmwareUpdate
		if err = json.Unmarshal(rb, &fwu); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			l.Error("unable to unmarshal request body", zap.Error(err))

			return
		}

		client := &http.Client{}

		req, err := http.NewRequest("GET", fwu.Remote, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("unable to set up request for firmware download", zap.Error(err))

			return
		}

		req.SetBasicAuth(h.tester.fwdlUser, h.tester.fwdlPass)

		resp, err := client.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("unable to download firmware", zap.Error(err), zap.String("status", resp.Status), zap.Int("status_code", resp.StatusCode))

			return
		}

		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			http.Error(w, "firmware download request status NOT OK", http.StatusInternalServerError)
			l.Error("firmware download request status NOT OK", zap.String("status", resp.Status), zap.Int("status_code", resp.StatusCode))

			return
		}

		respBody, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("firmware download read body failed", zap.Error(err))

			return
		}

		dlPath := filepath.Join(h.tester.fwDir, path.Base(resp.Request.URL.String()))
		if err = ioutil.WriteFile(dlPath, respBody, os.ModePerm); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("firmware file write failed", zap.Error(err), zap.String("filepath", dlPath))

			return
		}

		l.Info("new firmware file downloaded", zap.String("filepath", dlPath))

		if h.tester.fwdlWriteFName != "" {
			dest := filepath.Join(h.tester.fwDir, h.tester.fwdlWriteFName)
			l.Debug("duplicating firmware package to configured file name", zap.String("filepath_source", dlPath), zap.String("filepath_dest", dest))

			if err = copy.LinkOrCopy(dlPath, dest); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				l.Error("copy of firmware file failed", zap.Error(err), zap.String("filepath_source", dlPath), zap.String("filepath_dest", dest))

				return
			}

			l.Info("duplicated firmware package to configured file name", zap.String("filepath_source", dlPath), zap.String("filepath_dest", dest))
		}
	}
}

// TesterConfigResponse returns the configuration and type for the tester
type TesterConfigResponse struct {
	Body string `json:"body"`
	Type int    `json:"type"`
}

// HandleIncomingConfigurationRequest handles incoming requests to update a local configuration file
func (h *Handler) HandleIncomingConfigurationRequest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if HandleCORS(w, r) {
			return
		}

		l := endpointLogger(h.l, r)

		if h.tester.config == nil {
			http.Error(w, "tester configuration updates not allowed for this tester", http.StatusNotImplemented)
			l.Error("tester configuration updates not allowed for this tester")

			return
		}

		marshaler, err := marshalerForConfigType(h.tester.configType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("unable to determine marshaler for config type", zap.Int("config_type", h.tester.configType), zap.Error(err))

			return
		}

		cb, err := marshaler(h.tester.config)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("tester configuration marshal failed", zap.Error(err))

			return
		}

		cfg := TesterConfigResponse{
			Body: string(cb),
			Type: h.tester.configType,
		}

		rb, err := json.Marshal(cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("tester configuration marshal failed", zap.Error(err))

			return
		}

		if _, err = w.Write(rb); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("failed to write response body", zap.Error(err))

			return
		}
	}
}

func marshalerForConfigType(configType int) (func(interface{}) ([]byte, error), error) {
	var marshaler func(interface{}) ([]byte, error)

	switch configType {
	case ConfigTypeDefault, ConfigTypeJSON:
		marshaler = json.Marshal
	case ConfigTypeYAML:
		marshaler = yaml.Marshal
	default:
		return nil, errors.New("unsupported configuration type")
	}

	return marshaler, nil
}

func prettyMarshalerForConfigType(configType int) (func(interface{}) ([]byte, error), error) {
	var marshaler func(interface{}) ([]byte, error)

	switch configType {
	case ConfigTypeDefault, ConfigTypeJSON:
		marshaler = func(data interface{}) ([]byte, error) {
			return json.MarshalIndent(data, "", "\t")
		}
	case ConfigTypeYAML:
		marshaler = yaml.Marshal
	default:
		return nil, errors.New("unsupported configuration type")
	}

	return marshaler, nil
}

// HandleIncomingConfigurationFiles saves the configuration file to disk and updates local configuration object
func (h *Handler) HandleIncomingConfigurationFiles() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if HandleCORS(w, r) {
			return
		}

		l := endpointLogger(h.l, r)

		if h.tester.config == nil || h.tester.configPath == "" {
			http.Error(w, "tester configuration updates not allowed for this tester", http.StatusNotImplemented)
			l.Error("tester configuration updates not allowed for this tester")

			return
		}

		rb, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("unable to read request body", zap.Error(err))

			return
		}

		unmarshaler, err := unmarshalerForConfigType(h.tester.configType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("unable to determine unmarshaler for config type", zap.Int("config_type", h.tester.configType), zap.Error(err))

			return
		}

		if err = unmarshaler(rb, &h.tester.config); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("unable to unmarshal config object", zap.Error(err))

			return
		}

		// write to config location
		pmarshaler, err := prettyMarshalerForConfigType(h.tester.configType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("unable to determine marshaler for config type", zap.Int("config_type", h.tester.configType), zap.Error(err))

			return
		}

		bb, err := pmarshaler(h.tester.config)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("unable to marshal config object", zap.Error(err))

			return
		}

		if err = ioutil.WriteFile(h.tester.configPath, bb, os.ModePerm); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("unable to write config object", zap.Error(err))

			return
		}
	}
}

func unmarshalerForConfigType(configType int) (func([]byte, interface{}) error, error) {
	var unmarshaler func([]byte, interface{}) error

	switch configType {
	case ConfigTypeDefault, ConfigTypeJSON:
		unmarshaler = json.Unmarshal
	case ConfigTypeYAML:
		unmarshaler = yaml.Unmarshal
	default:
		return nil, errors.New("unsupported configuration type")
	}

	return unmarshaler, nil
}

// HandleUpdateRequests handles incoming requests to perform a self update
func (h *Handler) HandleUpdateRequests() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if HandleCORS(w, r) {
			return
		}

		l := endpointLogger(h.l, r)

		if !h.features.updateViaRestart {
			http.Error(w, "update via restart feature not allowed for this tester", http.StatusNotImplemented)
			l.Error("update via restart feature not allowed for this tester")

			return
		}
	}
}

const (
	// NameUnknown is the default name when not specified
	NameUnknown = "tester"
	// StatusAlive is the general response that the tester is alive
	StatusAlive = "alive"
	// VersionUnknown is the default version when not specified
	VersionUnknown = "0.0.1"
	// GitHashUnknown is the default gitsummary when not specified
	GitHashUnknown = "00000"
)

// TesterState response
type TesterState struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Version    string `json:"version"`
	GitSummary string `json:"git_summary"`
}

// HandleHealthRequests handles incoming health requests
func (h *Handler) HandleHealthRequests() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if HandleCORS(w, r) {
			return
		}

		l := endpointLogger(h.l, r)

		ts := TesterState{
			Name:       h.tester.name,
			Status:     StatusAlive,
			Version:    h.tester.version,
			GitSummary: h.tester.gitsummary,
		}

		b, err := json.Marshal(ts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("json.Marshal", zap.Error(err))

			return
		}

		w.Header().Add("Content-Type", "application/json")

		if _, err = w.Write(b); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			l.Error("w.Write", zap.Error(err))

			return
		}
	}
}

func endpointLogger(l *zap.Logger, r *http.Request) *zap.Logger {
	lPrime := l.With(zap.String("endpoint", r.URL.Path), zap.String("method", r.Method), zap.String("remote", r.RemoteAddr))
	lPrime.Info("tabitha endpoint hit")

	return lPrime
}

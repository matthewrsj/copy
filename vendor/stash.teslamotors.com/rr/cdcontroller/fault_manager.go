package cdcontroller

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"

	"go.uber.org/zap"
)

/*
  TrayFaultManager keeps track of new faults on the Charge/Discharge system. It is designed to
  allow Tower Controller to resume recipes on a tray after it is routed to a new fixture when a the
  first fixture faults.

  Sequence
  - Fixture faults while running tray
  - TC sends fault record to CDC
  - TrayFaultManager records fault
  - TC requests unload
  - CDC routes tray into new fixture using normal means
  - Before starting recipe TC queries CDC for op snapshot if it exists
  - If op snapshot exists it is added to recipe sent to FXR
  - If tray passes it POSTs to CDC to clear fault record
  - If tray fault count ever exceeds maximum, CDC routes tray out of system
*/

// TrayFaultManager records trays that faulted and are on their way to a new fixture
type TrayFaultManager struct {
	// faultRegistry is a map of tray ID to fault record
	faultRegistry map[string]*FaultRecord
	mx            *sync.Mutex
}

// FaultRecord contains the metadata of the fault and the operational snapshot
type FaultRecord struct {
	OpSnapshot      []byte   `json:"op_snapshot"`
	FixturesFaulted []string `json:"fixtures_faulted"`
}

// TrayFaultManagerOption modifies a *TrayFaultManager
type TrayFaultManagerOption func(t *TrayFaultManager)

// NewTrayFaultManager applies passed options and returns a new *TrayFaultManager
func NewTrayFaultManager(opts ...TrayFaultManagerOption) *TrayFaultManager {
	t := &TrayFaultManager{
		faultRegistry: make(map[string]*FaultRecord),
		mx:            &sync.Mutex{},
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

func idFromTrayID(tid string) string {
	return strings.ReplaceAll(strings.TrimRight(strings.ToLower(tid), "abcd"), "-", "")
}

// NewTrayFault registers a new tray fault on the manager
func (t *TrayFaultManager) NewTrayFault(tid, location string, ops []byte) {
	tid = idFromTrayID(tid)

	t.mx.Lock()
	defer t.mx.Unlock()

	tf, ok := t.faultRegistry[tid]
	if !ok {
		t.faultRegistry[tid] = &FaultRecord{
			OpSnapshot:      ops,
			FixturesFaulted: []string{location},
		}

		return
	}

	tf.OpSnapshot = ops
	tf.FixturesFaulted = append(tf.FixturesFaulted, location)
}

// ClearTrayFault clears an existing tray fault record from the registry
func (t *TrayFaultManager) ClearTrayFault(tid string) {
	tid = idFromTrayID(tid)

	t.mx.Lock()
	defer t.mx.Unlock()

	delete(t.faultRegistry, tid)
}

// FaultRecordForTray returns the FaultRecord for the tid
func (t *TrayFaultManager) FaultRecordForTray(tid string) (*FaultRecord, bool) {
	tid = idFromTrayID(tid)
	v, ok := t.faultRegistry[tid]

	return v, ok
}

const (
	// TrayFaultEndpoint is the endpoint with which the tower controller communicates to post or get snapshots
	TrayFaultEndpoint = "/trayfaulted"
	// TrayIDQueryParameter is the tray being queried
	TrayIDQueryParameter = "tray"
)

// TrayFaultRequest is used to request a new fault to be added to the manager
type TrayFaultRequest struct {
	Tray       string `json:"tray"`
	Location   string `json:"location"`
	Clear      bool   `json:"clear"`
	OpSnapshot []byte `json:"op_snapshot"`
}

// HandleFaultRequest handles incoming requests to the fault request endpoint
func HandleFaultRequest(logger *zap.SugaredLogger, tfm *TrayFaultManager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		cl := logger.With("endpoint", TrayFaultEndpoint)
		cl.Infow("got request to endpoint", "method", r.Method)

		w.Header().Add("Content-Type", "application/json")

		switch r.Method {
		case http.MethodGet:
			tid := r.URL.Query().Get(TrayIDQueryParameter)
			if tid == "" {
				cl.Infow("no tray query parameter, sending all state back")
				handleGetAll(cl, tfm, w)

				return
			}

			cl.Infow("got request for one tray", "tray", tid)
			handleGetOne(cl, tfm, tid, w)
		case http.MethodPost:
			handlePost(cl, tfm, w, r)
		}
	}
}

func handleGetAll(cl *zap.SugaredLogger, tfm *TrayFaultManager, w http.ResponseWriter) {
	jb, err := json.Marshal(tfm.faultRegistry)
	if err != nil {
		cl.Errorw("unable to marshal response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	if _, err = w.Write(jb); err != nil {
		cl.Errorw("unable to write response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handleGetOne(cl *zap.SugaredLogger, tfm *TrayFaultManager, tid string, w http.ResponseWriter) {
	fr, ok := tfm.FaultRecordForTray(tid)
	if !ok {
		cl.Warnw("fault record does not exist for tray", "tray", tid)
		http.Error(w, "fault record does not exist for tray "+tid, http.StatusNotFound)

		return
	}

	jb, err := json.Marshal(fr)
	if err != nil {
		cl.Errorw("unable to marshal response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	// specific tray ID requested
	if _, err := w.Write(jb); err != nil {
		cl.Errorw("unable to write response", "error", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func handlePost(cl *zap.SugaredLogger, tfm *TrayFaultManager, w http.ResponseWriter, r *http.Request) {
	rb, err := ioutil.ReadAll(r.Body)
	if err != nil {
		cl.Errorw("unable to read request", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	defer func() {
		_ = r.Body.Close()
	}()

	var req TrayFaultRequest
	if err = json.Unmarshal(rb, &req); err != nil {
		cl.Errorw("unable to unmarshal request", "error", err)
		http.Error(w, err.Error(), http.StatusBadRequest)

		return
	}

	if req.Tray == "" || req.Location == "" {
		cl.Errorw("request did not include required tray or location fields", "request", req)
		http.Error(w, "request did not include required tray or location fields", http.StatusBadRequest)
	}

	if req.Clear {
		cl.Info("clearing tray fault")
		tfm.ClearTrayFault(req.Tray)

		return
	}

	if req.OpSnapshot == nil {
		cl.Error("op_snapshot is nil but is required when not clearing fault")
		http.Error(w, "op_snapshot is nil but is required when not clearing fault", http.StatusBadRequest)

		return
	}

	cl.Infow("handling tray fault request", "request", req)

	tfm.NewTrayFault(req.Tray, req.Location, req.OpSnapshot)
}

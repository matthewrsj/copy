package towercontroller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/cdcontroller"
)

// LoadEndpoint is the endpoint that handles load requests from the C/D Controller
const LoadEndpoint = "/load"

// HandleLoad handles requests the the load endpoint
func HandleLoad(conf Configuration, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) http.HandlerFunc {
	var mux sync.Mutex

	return func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		cl := logger.With("endpoint", LoadEndpoint, "remote", r.RemoteAddr)

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			cl.Errorw("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var loadRequest cdcontroller.FXRLoad
		if err = json.Unmarshal(b, &loadRequest); err != nil {
			cl.Errorw("unmarshal request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		cl = cl.With("load_request", loadRequest)

		if loadRequest.TransactionID == "" {
			err = errors.New("invalid empty transaction ID")
			cl.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		fxr, err := cdcontroller.NewFixtureBarcode(
			fmt.Sprintf("%s-%s%s-%02d-%02d", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, loadRequest.Column, loadRequest.Level),
		)
		if err != nil {
			cl.Errorw("parse request body for fixture ID", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		fInfo, ok := registry[IDFromFXR(fxr)]
		if !ok {
			err := fmt.Errorf("registry did not contain fixture %s", fxr.Raw)
			cl.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		// internal validation of the TC statemachine begins here. This is not threadsafe. Lock out other threads
		// while we validate the state and whether or not we can accept the load request.
		mux.Lock()
		defer mux.Unlock()

		if fInfo.Avail.Status() == StatusUnknown || fInfo.Avail.Status() > StatusWaitingForLoad {
			err := fmt.Errorf("received load complete for fixture %s, which is already processing a tray", fxr.Raw)
			cl.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		select {
		case fInfo.LDC <- loadRequest:
			cl.Info("sent load request to fixture state machine")
		default:
			// do not fail back to the CDC however as this is probably a duplicate
			cl.Error("received load complete for fixture which is unable to process a tray")
		}

		w.WriteHeader(http.StatusOK)

		for i := 0; fInfo.Avail.Status() <= StatusWaitingForLoad && i < 10; i++ {
			// checking every 10 ms for the status to update before releasing the lock
			// sort of a "debounce", but only wait for 100 ms so we aren't blocking forever,
			// considering this is blocking for all loads on the tower, not just this fixture.
			time.Sleep(time.Millisecond * 10)
		}
	}
}

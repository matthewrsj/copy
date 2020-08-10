package towercontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
)

const _unreserveEndpoint = "/unreserve"

// HandleUnreserveFixture accepts POST requests to manually un-reserve a reserved fixture
func HandleUnreserveFixture(logger *zap.SugaredLogger, registry map[string]*FixtureInfo) {
	http.HandleFunc(_unreserveEndpoint, func(w http.ResponseWriter, r *http.Request) {
		logger.Infow(fmt.Sprintf("got request to %s", _unreserveEndpoint))

		cl := logger.With("endpoint", _unreserveEndpoint)

		if r.Method != "POST" {
			cl.Errorw("received invalid request type", "request_type", r.Method)
			http.Error(w, "this endpoint only accepts POST request", http.StatusBadRequest)
		}

		jb, err := ioutil.ReadAll(r.Body)
		if err != nil {
			cl.Errorw("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		// reuse RequestForm body
		var rf RequestForm

		if err = json.Unmarshal(jb, &rf); err != nil {
			cl.Errorw("unmarshal request body", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		fxrInfo, ok := registry[rf.FixtureID]
		if !ok {
			cl.Errorw("unable to find fixture %s in registry", rf.FixtureID)
			http.Error(w, fmt.Sprintf("unable to find fixture %s in registry", rf.FixtureID), http.StatusBadRequest)

			return
		}

		if fxrInfo.Avail.Status() != StatusWaitingForLoad {
			cl.Errorf("fixture %s status is %s, should be %s", rf.FixtureID, fxrInfo.Avail.Status(), StatusWaitingForLoad)
			http.Error(w, fmt.Sprintf("fixture %s status is %v, should be %v", rf.FixtureID, fxrInfo.Avail.Status(), StatusWaitingForLoad), http.StatusBadRequest)

			return
		}

		select {
		case fxrInfo.Unreserve <- struct{}{}:
		default:
		}

		cl.Infof("reservation removal requested for fixture %s", fxrInfo.Name)

		w.WriteHeader(http.StatusOK)
	})
}

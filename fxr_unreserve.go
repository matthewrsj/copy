package towercontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
)

// UnreserveFixtureEndpoint handles incoming requests to un-reserve fixtures
const UnreserveFixtureEndpoint = "/unreserve"

// HandleUnreserveFixture accepts POST requests to manually un-reserve a reserved fixture
func HandleUnreserveFixture(logger *zap.SugaredLogger, registry map[string]*FixtureInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		allowCORS(w)

		logger = logger.With("endpoint", UnreserveFixtureEndpoint, "remote", r.RemoteAddr)
		logger.Info("got request to endpoint")

		jb, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Errorw("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		// reuse RequestForm body
		var rf RequestForm

		if err = json.Unmarshal(jb, &rf); err != nil {
			logger.Errorw("unmarshal request body", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		fxrInfo, ok := registry[rf.FixtureID]
		if !ok {
			logger.Errorw("unable to find fixture %s in registry", rf.FixtureID)
			http.Error(w, fmt.Sprintf("unable to find fixture %s in registry", rf.FixtureID), http.StatusBadRequest)

			return
		}

		if fxrInfo.Avail.Status() != StatusWaitingForLoad {
			logger.Errorf("fixture %s status is %s, should be %s", rf.FixtureID, fxrInfo.Avail.Status(), StatusWaitingForLoad)
			http.Error(w, fmt.Sprintf("fixture %s status is %v, should be %v", rf.FixtureID, fxrInfo.Avail.Status(), StatusWaitingForLoad), http.StatusBadRequest)

			return
		}

		select {
		case fxrInfo.Unreserve <- struct{}{}:
		default:
		}

		logger.Info("reservation removal requested for fixture", "fixture", fxrInfo.Name)

		w.WriteHeader(http.StatusOK)
	}
}

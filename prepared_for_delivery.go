package towercontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/traycontrollers"
)

const _preparedForDeliveryEndpoint = "/preparedForDelivery"

// HandlePreparedForDelivery handles incoming prepared for delivery messages
func HandlePreparedForDelivery(mux *http.ServeMux, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) {
	mux.HandleFunc(_preparedForDeliveryEndpoint, func(w http.ResponseWriter, r *http.Request) {
		logger.Infow(fmt.Sprintf("got request to %s", _preparedForDeliveryEndpoint))

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Error("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var pfd traycontrollers.PreparedForDelivery
		if err = json.Unmarshal(b, &pfd); err != nil {
			logger.Errorw("unmarshal request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		id, err := IDFromFXRString(pfd.Fixture)
		if err != nil {
			err = fmt.Errorf("ID from FXR string: %v", err)
			logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		fInfo, ok := registry[id]
		if !ok {
			err := fmt.Errorf("registry did not contain fixture %s", id)
			logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		if fInfo.Avail.Status() == StatusUnknown || fInfo.Avail.Status() > StatusWaitingForReservation {
			err := fmt.Errorf("received preparedForDelivery for fixture %s, which is not prepared for delivery", pfd.Fixture)
			logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		fInfo.PFD <- pfd

		w.WriteHeader(http.StatusOK)
	})
}

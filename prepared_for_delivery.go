package towercontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/cdcontroller"
)

// PreparedForDeliveryEndpoint handles incoming POSTs to reserve a fixture
const PreparedForDeliveryEndpoint = "/preparedForDelivery"

// HandlePreparedForDelivery handles incoming prepared for delivery messages
func HandlePreparedForDelivery(logger *zap.SugaredLogger, registry map[string]*FixtureInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cl := logger.With("endpoint", PreparedForDeliveryEndpoint, "remote", r.RemoteAddr)

		cl.Info("got request to endpoint")

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			cl.Error("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var pfd cdcontroller.PreparedForDelivery
		if err = json.Unmarshal(b, &pfd); err != nil {
			cl.Errorw("unmarshal request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		id, err := IDFromFXRString(pfd.Fixture)
		if err != nil {
			err = fmt.Errorf("ID from FXR string: %v", err)
			cl.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		fInfo, ok := registry[id]
		if !ok {
			err := fmt.Errorf("registry did not contain fixture %s", id)
			cl.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		if fInfo.Avail.Status() == StatusUnknown || fInfo.Avail.Status() > StatusWaitingForReservation {
			err := fmt.Errorf("received preparedForDelivery for fixture %s, which is not prepared for delivery", pfd.Fixture)
			cl.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		fInfo.PFD <- pfd

		w.WriteHeader(http.StatusOK)
	}
}

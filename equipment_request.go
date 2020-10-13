package towercontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

// SendEquipmentRequestEndpoint is where equipment requests are sent to be sent to fixtures
const SendEquipmentRequestEndpoint = "/equipment_request"

// RequestEquipment is the request made to send an EquipmentRequest to the specified fixture
type RequestEquipment struct {
	FixtureID        string `json:"fixture_id"`
	EquipmentRequest string `json:"equipment_request"`
}

// HandleSendEquipmentRequest accepts POST requests to send an equipment request to a fixture.
// Common use-case for this is to approve a commission self test on a fixture.
func HandleSendEquipmentRequest(publisher *protostream.Socket, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger = logger.With("endpoint", SendEquipmentRequestEndpoint, "remote", r.RemoteAddr)
		logger.Info("got request to endpoint")

		jb, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Errorw("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var rf RequestEquipment

		if err = json.Unmarshal(jb, &rf); err != nil {
			logger.Errorw("unmarshal request body", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		// confirm the fixture is valid
		fxrInfo, ok := registry[rf.FixtureID]
		if !ok {
			logger.Errorw("unable to find fixture in registry", "fixture", rf.FixtureID)
			http.Error(w, fmt.Sprintf("unable to find fixture %s in registry", rf.FixtureID), http.StatusBadRequest)

			return
		}

		equipReq, ok := tower.EquipmentRequest_value[rf.EquipmentRequest]
		if !ok {
			logger.Errorw("invalid equipment request", "equipment_request", rf.EquipmentRequest)
			http.Error(w, fmt.Sprintf("invalid form request %s", rf.EquipmentRequest), http.StatusBadRequest)

			return
		}

		sendMsg := tower.TowerToFixture{
			EquipmentRequest: tower.EquipmentRequest(equipReq),
		}

		if err := sendProtoMessage(publisher, &sendMsg, fxrInfo.Name); err != nil {
			logger.Errorw("unable to send equipment request", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		logger.Infow("published equipment request", "equipment_request", rf.EquipmentRequest)

		w.WriteHeader(http.StatusOK)
	}
}

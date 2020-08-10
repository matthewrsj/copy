package towercontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
)

const _sendEquipmentRequestEndpoint = "/equipment_request"

// RequestEquipment is the request made to send an EquipmentRequest to the specified fixture
type RequestEquipment struct {
	FixtureID        string `json:"fixture_id"`
	EquipmentRequest string `json:"equipment_request"`
}

// HandleSendEquipmentRequest accepts POST requests to send an equipment request to a fixture.
// Common use-case for this is to approve a commission self test on a fixture.
func HandleSendEquipmentRequest(publisher *protostream.Socket, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) {
	http.HandleFunc(_sendEquipmentRequestEndpoint, func(w http.ResponseWriter, r *http.Request) {
		logger.Infow(fmt.Sprintf("got request to %s", _sendEquipmentRequestEndpoint))

		cl := logger.With("endpoint", _sendEquipmentRequestEndpoint)

		if r.Method != http.MethodPost {
			cl.Errorw("received invalid request type", "request_type", r.Method)
			http.Error(w, "this endpoint only accepts POST requests", http.StatusBadRequest)

			return
		}

		jb, err := ioutil.ReadAll(r.Body)
		if err != nil {
			cl.Errorw("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var rf RequestEquipment

		if err = json.Unmarshal(jb, &rf); err != nil {
			cl.Errorw("unmarshal request body", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		// confirm the fixture is valid
		fxrInfo, ok := registry[rf.FixtureID]
		if !ok {
			cl.Errorw("unable to find fixture in registry", "fixture", rf.FixtureID)
			http.Error(w, fmt.Sprintf("unable to find fixture %s in registry", rf.FixtureID), http.StatusBadRequest)

			return
		}

		equipReq, ok := pb.EquipmentRequest_value[rf.EquipmentRequest]
		if !ok {
			cl.Errorw("invalid equipment request", "equipment_request", rf.EquipmentRequest)
			http.Error(w, fmt.Sprintf("invalid form request %s", rf.EquipmentRequest), http.StatusBadRequest)

			return
		}

		sendMsg := pb.TowerToFixture{
			EquipmentRequest: pb.EquipmentRequest(equipReq),
		}

		rb, err := proto.Marshal(&sendMsg)
		if err != nil {
			cl.Errorw("unable to marshal equipment request", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		sendEvent := protostream.ProtoMessage{
			Location: fxrInfo.Name,
			Body:     rb,
		}

		sendBody, err := json.Marshal(sendEvent)
		if err != nil {
			cl.Errorw("unable to marshal data to send to protostream", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		if err := publisher.PublishTo(fxrInfo.Name, sendBody); err != nil {
			cl.Errorw("unable to send data to protostream", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		cl.Infow("published equipment request", "equipment_request", rf.EquipmentRequest)

		w.WriteHeader(http.StatusOK)
	})
}

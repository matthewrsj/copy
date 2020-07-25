package towercontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
)

const (
	_resetFaultEndpoint = "/resetfault"
	_resetFaultTimeout  = time.Second * 3
)

// ResetFault is the request made to reset a fixture fault to the resetFaultEndpoint
type ResetFault struct {
	FixtureID string `json:"fixture_id"`
}

// HandleResetFixtureFault accepts POST requests to reset a faulted fixture
// nolint:gocognit // no reason to split this out
func HandleResetFixtureFault(publisher *protostream.Socket, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) {
	http.HandleFunc(_resetFaultEndpoint, func(w http.ResponseWriter, r *http.Request) {
		logger.Infow(fmt.Sprintf("got request to %s", _resetFaultEndpoint))

		cl := logger.With("endpoint", _resetFaultEndpoint)

		if r.Method != "POST" {
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

		var rf ResetFault

		if err = json.Unmarshal(jb, &rf); err != nil {
			cl.Errorw("unmarshal request body", "error", err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		fxrInfo, ok := registry[rf.FixtureID]
		if !ok {
			cl.Errorw("unable to find fixture in registry", "fixture", rf.FixtureID)
			http.Error(w, fmt.Sprintf("unable to find fixture %s in registry", rf.FixtureID), http.StatusBadRequest)

			return
		}

	faultLoop:
		for begin := time.Now(); time.Since(begin) < _resetFaultTimeout; {
			select {
			case msg := <-fxrInfo.SC:
				cl.Debug("got message, checking if fixture is available")

				var event protostream.ProtoMessage
				if err = json.Unmarshal(msg.Msg.Body, &event); err != nil {
					cl.Debugw("unmarshal JSON frame", "error", err, "bytes", string(msg.Msg.Body))
					return
				}

				cl.Debug("received message from FXR, unmarshalling to check if it is available")

				var pMsg pb.FixtureToTower

				if err = proto.Unmarshal(event.Body, &pMsg); err != nil {
					cl.Debugw("not the message we were expecting", "error", err)
					break
				}

				if pMsg.GetOp() == nil {
					cl.Debugw("look for the next message, this is diagnostic", "msg", pMsg.String())
					break
				}

				cl.Debugw("fixture status rxd, checking if faulted", "status", pMsg.GetOp().GetStatus().String())

				if pMsg.GetOp().GetStatus() != pb.FixtureStatus_FIXTURE_STATUS_FAULTED {
					cl.Errorw("reset fault request received for fixture that is not faulted", "status", pMsg.GetOp().GetStatus().String())
					http.Error(w, fmt.Sprintf("fixture %s status %s not faulted", fxrInfo.Name, pMsg.GetOp().GetStatus().String()), http.StatusBadRequest)

					return
				}

				break faultLoop
			case <-time.After(_resetFaultTimeout):
				cl.Errorw("unable to read status in timeout", "timeout", _resetFaultTimeout)
				http.Error(w, "unable to read status in timeout", http.StatusInternalServerError)

				return
			}
		}

		cl.Debug("fixture status is faulted, sending fault reset")

		sendMsg := pb.TowerToFixture{
			Recipe: &pb.Recipe{
				Formrequest: pb.FormRequest_FORM_REQUEST_FAULT_RESET,
			},
		}

		rb, err := proto.Marshal(&sendMsg)
		if err != nil {
			cl.Errorw("unable to marshal fault reset request", "error", err)
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

		cl.Debug("published fault reset request")

		w.WriteHeader(http.StatusOK)
	})
}

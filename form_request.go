package towercontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
)

const _sendFormRequestEndpoint = "/form_request"

// RequestForm is the request made to send a FormRequest to a fixture
type RequestForm struct {
	FixtureID   string `json:"fixture_id"`
	FormRequest string `json:"form_request"`
}

// HandleSendFormRequest accepts POST requests to send a form request to a fixture.
// Common use-case for this is to reset a faulted fixture.
// nolint:gocognit // no reason to split this out
func HandleSendFormRequest(publisher *protostream.Socket, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) {
	http.HandleFunc(_sendFormRequestEndpoint, func(w http.ResponseWriter, r *http.Request) {
		logger.Infow(fmt.Sprintf("got request to %s", _sendFormRequestEndpoint))

		cl := logger.With("endpoint", _sendFormRequestEndpoint)

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

		var rf RequestForm

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

		formReq, ok := pb.FormRequest_value[rf.FormRequest]
		if !ok {
			cl.Errorw("invalid form request", "form_request", rf.FormRequest)
			http.Error(w, fmt.Sprintf("invalid form request %s", rf.FormRequest), http.StatusBadRequest)

			return
		}

		sendMsg := pb.TowerToFixture{
			Recipe: &pb.Recipe{
				Formrequest: pb.FormRequest(formReq),
			},
		}

		if err := sendProtoMessage(publisher, &sendMsg, fxrInfo.Name); err != nil {
			cl.Errorw("unable to send data to protostream", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		cl.Infow("published form request", "form_request", rf.FormRequest)

		w.WriteHeader(http.StatusOK)
	})
}

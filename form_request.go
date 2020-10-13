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

// SendFormRequestEndpoint handles incoming requests to send form requests to a fixture
const SendFormRequestEndpoint = "/form_request"

// RequestForm is the request made to send a FormRequest to a fixture
type RequestForm struct {
	FixtureID   string `json:"fixture_id"`
	FormRequest string `json:"form_request"`
}

// HandleSendFormRequest accepts POST requests to send a form request to a fixture.
// Common use-case for this is to reset a faulted fixture.
// nolint:gocognit // no reason to split this out
func HandleSendFormRequest(publisher *protostream.Socket, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		logger = logger.With("endpoint", SendFormRequestEndpoint, "remote", r.RemoteAddr)
		logger.Info("got request to endpoint")

		jb, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Errorw("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var rf RequestForm

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

		formReq, ok := tower.FormRequest_value[rf.FormRequest]
		if !ok {
			logger.Errorw("invalid form request", "form_request", rf.FormRequest)
			http.Error(w, fmt.Sprintf("invalid form request %s", rf.FormRequest), http.StatusBadRequest)

			return
		}

		sendMsg := tower.TowerToFixture{
			Recipe: &tower.Recipe{
				Formrequest: tower.FormRequest(formReq),
			},
		}

		if err := sendProtoMessage(publisher, &sendMsg, fxrInfo.Name); err != nil {
			logger.Errorw("unable to send data to protostream", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		logger.Infow("published form request", "form_request", rf.FormRequest)

		w.WriteHeader(http.StatusOK)
	}
}

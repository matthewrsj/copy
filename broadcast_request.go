package towercontroller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/cdcontroller"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

// HandleBroadcastRequest handles incoming broadcast requests from the CD Controller
func HandleBroadcastRequest(publisher *protostream.Socket, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cl := logger.With("endpoint", cdcontroller.BroadcastEndpoint)
		cl.Infow("got request to endpoint")

		rb, err := ioutil.ReadAll(r.Body)
		if err != nil {
			cl.Errorw("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var broadcastRequest cdcontroller.BroadcastRequest
		if err = json.Unmarshal(rb, &broadcastRequest); err != nil {
			cl.Errorw("unmarshal request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var targets []string

		if broadcastRequest.Scale >= cdcontroller.ScaleTower {
			cl.Debugw("scale is tower or higher, broadcasting to all", "scale", broadcastRequest.Scale.String())
			// we are broadcasting to entire tower, so grab everything in the registry
			for name := range registry {
				if broadcastRequest.ExcludeOrigin && broadcastRequest.Originator.Location == name {
					cl.Debugw("skipping origin", "origin", name)
					// this was the originator and we don't want to re-broadcast
					continue
				}

				targets = append(targets, name)
				cl.Debugw("added target", "target", name)
			}
		} else {
			// we are getting individual requests for each FXR
			fInfo, ok := registry[broadcastRequest.Target]
			if !ok {
				err := fmt.Errorf("registry did not contain fixture %s", broadcastRequest.Target)
				cl.Error(err)
				http.Error(w, err.Error(), http.StatusBadRequest)

				return
			}

			targets = append(targets, fInfo.Name)
			cl.Debugw("added target", "target", fInfo.Name)
		}

		cl.Infow("sending operation to fixture(s)", "operation", broadcastRequest.Operation.String())

		msg := tower.TowerToFixture{
			Recipe: &tower.Recipe{}, // need to instantiate this so it isn't nil
		}

		switch broadcastRequest.Operation {
		case cdcontroller.OperationStopFormation:
			msg.GetRecipe().FormRequest = tower.FormRequest_FORM_REQUEST_STOP
		case cdcontroller.OperationPauseFormation:
			msg.GetRecipe().FormRequest = tower.FormRequest_FORM_REQUEST_PAUSE
		case cdcontroller.OperationResumeFormation:
			msg.GetRecipe().FormRequest = tower.FormRequest_FORM_REQUEST_RESUME
		case cdcontroller.OperationStopIsoCheck:
			// TODO: not implemented in towerproto yet
		case cdcontroller.OperationFaultReset:
			msg.GetRecipe().FormRequest = tower.FormRequest_FORM_REQUEST_FAULT_RESET
		default:
			// nothing to do
			err := errors.New("no operation requested")
			cl.Error(err)
			http.Error(w, err.Error(), http.StatusBadRequest)

			return
		}

		for _, target := range targets {
			if err := sendProtoMessage(publisher, &msg, target); err != nil {
				cl.Errorw("unable to send proto message", "error", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			cl.Infow("sent proto message to fixture", "fixture", target, "request", msg.GetRecipe().GetFormRequest().String())
		}

		cl.Debug("done sending broadcasts")
		w.WriteHeader(http.StatusOK)
	}
}

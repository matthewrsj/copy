package towercontroller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

// HandleBroadcastRequest handles incoming broadcast requests from the CD Controller
func HandleBroadcastRequest(publisher *protostream.Socket, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) {
	http.HandleFunc(traycontrollers.BroadcastEndpoint, func(w http.ResponseWriter, r *http.Request) {
		cl := logger.With("endpoint", traycontrollers.BroadcastEndpoint)
		cl.Infow("got request to endpoint")

		if r.Method != http.MethodPost {
			cl.Errorw("received invalid request type", "request_type", r.Method)
			http.Error(w, "this endpoint only accepts POST requests", http.StatusBadRequest)

			return
		}

		rb, err := ioutil.ReadAll(r.Body)
		if err != nil {
			cl.Errorw("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var broadcastRequest traycontrollers.BroadcastRequest
		if err = json.Unmarshal(rb, &broadcastRequest); err != nil {
			cl.Errorw("unmarshal request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var targets []string

		if broadcastRequest.Scale >= traycontrollers.ScaleTower {
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

		msg := pb.TowerToFixture{
			Recipe: &pb.Recipe{}, // need to instantiate this so it isn't nil
		}

		switch broadcastRequest.Operation {
		case traycontrollers.OperationStopFormation:
			msg.GetRecipe().Formrequest = pb.FormRequest_FORM_REQUEST_STOP
		case traycontrollers.OperationPauseFormation:
			msg.GetRecipe().Formrequest = pb.FormRequest_FORM_REQUEST_PAUSE
		case traycontrollers.OperationResumeFormation:
			msg.GetRecipe().Formrequest = pb.FormRequest_FORM_REQUEST_RESUME
		case traycontrollers.OperationStopIsoCheck:
			// TODO: not implemented in towerproto yet
		case traycontrollers.OperationFaultReset:
			msg.GetRecipe().Formrequest = pb.FormRequest_FORM_REQUEST_FAULT_RESET
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

			cl.Infow("sent proto message to fixture", "fixture", target, "request", msg.GetRecipe().GetFormrequest().String())
		}

		cl.Debug("done sending broadcasts")
		w.WriteHeader(http.StatusOK)
	})
}
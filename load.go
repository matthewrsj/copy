package towercontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
	"stash.teslamotors.com/rr/traycontrollers"
)

const _loadEndpoint = "/load"

// HandleLoad handles requests the the load endpoint
func HandleLoad(conf Configuration, logger *zap.SugaredLogger, registry map[string]*FixtureInfo) {
	http.HandleFunc(_loadEndpoint, func(w http.ResponseWriter, r *http.Request) {
		logger.Infow("got request to /load", "remote", r.RemoteAddr)

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Errorw("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var loadRequest traycontrollers.FXRLoad
		if err = json.Unmarshal(b, &loadRequest); err != nil {
			logger.Errorw("unmarshal request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		if loadRequest.TransactionID == "" {
			err = fmt.Errorf("invalid transaction ID %s, must be greater than 0", loadRequest.TransactionID)
			logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		fxr, err := traycontrollers.NewFixtureBarcode(
			fmt.Sprintf("%s-%s%s-%02d-%02d", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, loadRequest.Column, loadRequest.Level),
		)
		if err != nil {
			logger.Errorw("parse request body for fixture ID", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		fInfo, ok := registry[IDFromFXR(fxr)]
		if !ok {
			err := fmt.Errorf("registry did not contain fixture %s", fxr.Raw)
			logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		if fInfo.Avail.Status() == StatusUnknown || fInfo.Avail.Status() > StatusWaitingForLoad {
			err := fmt.Errorf("received load complete for fixture %s, which is already processing a tray", fxr.Raw)
			logger.Error(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		fInfo.LDC <- loadRequest

		w.WriteHeader(http.StatusOK)
	})
}

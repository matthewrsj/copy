package towercontroller

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/traycontrollers"
)

const _loadEndpoint = "/load"

// FXRLoad is used to post to the TC that a tray is loaded
type FXRLoad struct {
	Column int    `json:"column"`
	Level  int    `json:"level"`
	TrayID string `json:"tray"`
}

// HandleLoad handles requests the the load endpoint
func HandleLoad(conf Configuration, load chan statemachine.Job, logger *zap.SugaredLogger, mockCellAPI bool) {
	http.HandleFunc(_loadEndpoint, func(w http.ResponseWriter, r *http.Request) {
		logger.Infow("got request to /load", "remote", r.RemoteAddr)

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			logger.Errorw("read request body", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		var loadRequest FXRLoad
		if err = json.Unmarshal(b, &loadRequest); err != nil {
			logger.Errorw("unmarshal request body", "error", err)
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

		tbc, err := traycontrollers.NewTrayBarcode(loadRequest.TrayID)
		if err != nil {
			logger.Errorw("parse request body for tray ID", "error", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)

			return
		}

		load <- statemachine.Job{
			Name: IDFromFXR(fxr),
			Work: Barcodes{
				Fixture:     fxr,
				Tray:        tbc,
				MockCellAPI: mockCellAPI,
			},
		}

		w.WriteHeader(http.StatusOK)
	})
}

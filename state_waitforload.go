package towercontroller

import (
	"fmt"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/protostream"
	"stash.teslamotors.com/rr/traycontrollers"
)

// WaitForLoad waits for a loaded message from the C/D controller
type WaitForLoad struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client
	Publisher     *protostream.Socket
	SubscribeChan <-chan *protostream.Message

	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	steps           traycontrollers.StepConfiguration
	processStepName string
	transactID      int64
	recipeVersion   int
	manual          bool
	mockCellAPI     bool
	resetToIdle     bool

	fxrInfo *FixtureInfo

	err error
}

func (w *WaitForLoad) action() {
	w.fxrInfo.Avail.Set(StatusWaitingForLoad)
	w.resetToIdle = false

	w.Logger.Infow("waiting for load complete message from C/D controller", "fixture", w.fxbc)

	var fxrLoad traycontrollers.FXRLoad

	select {
	case <-w.fxrInfo.Unreserve:
		w.Logger.Warn("waitforload: reservation manually removed")
		w.resetToIdle = true

		return
	case fxrLoad = <-w.fxrInfo.LDC:
	}

	fxrID := fmt.Sprintf("%s-%s%s-%02d-%02d", w.Config.Loc.Line, w.Config.Loc.Process, w.Config.Loc.Aisle, fxrLoad.Column, fxrLoad.Level)

	tbc, fxbc, err := newIDs(fxrLoad.TrayID, fxrID)
	if err != nil {
		w.err = fmt.Errorf("parse IDs: %v", err)
		w.Logger.Error(w.err)

		return
	}

	if tbc != w.tbc || fxbc != w.fxbc {
		// they don't match, but always do what the C/D controller tells us
		w.Logger.Warnw("got invalid load complete for this tray, overwriting local with load command",
			"expected_tray", w.tbc.Raw,
			"actual_tray", tbc.Raw,
			"expected_fixture", w.fxbc.Raw,
			"actual_fixture", fxbc.Raw,
		)

		w.tbc = tbc
		w.fxbc = fxbc
	}

	w.processStepName = fxrLoad.RecipeName
	w.recipeVersion = fxrLoad.RecipeVersion
	w.steps = fxrLoad.Steps
	w.transactID = fxrLoad.TransactionID

	if w.processStepName == "" || len(w.steps) == 0 {
		w.err = fmt.Errorf("invalid fixture load message: %v", fxrLoad)
		w.Logger.Error(w.err)

		return
	}
}

// Actions returns the action functions for this state
func (w *WaitForLoad) Actions() []func() {
	return []func(){
		w.action,
	}
}

// Next returns the state to run after this one
func (w *WaitForLoad) Next() statemachine.State {
	var next statemachine.State

	switch {
	case w.err != nil || w.resetToIdle:
		w.Logger.Warnw("going back to idle state", "error", w.err, "reservation_cleared", w.resetToIdle)

		next = &Idle{
			Config:        w.Config,
			Logger:        w.Logger,
			CellAPIClient: w.CellAPIClient,
			Publisher:     w.Publisher,
			SubscribeChan: w.SubscribeChan,
			Manual:        w.manual,
			MockCellAPI:   w.mockCellAPI,
			FXRInfo:       w.fxrInfo,
		}
	default:
		next = &ProcessStep{
			Config:        w.Config,
			Logger:        w.Logger,
			CellAPIClient: w.CellAPIClient,
			Publisher:     w.Publisher,
			SubscribeChan: w.SubscribeChan,
			fxrInfo:       w.fxrInfo,
		}

		next.SetContext(Barcodes{
			Fixture:         w.fxbc,
			Tray:            w.tbc,
			ProcessStepName: w.processStepName,
			MockCellAPI:     w.mockCellAPI,
			RecipeName:      w.processStepName,
			RecipeVersion:   w.recipeVersion,
			StepConf:        w.steps,
			TransactID:      w.transactID,
		})
	}

	w.Logger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

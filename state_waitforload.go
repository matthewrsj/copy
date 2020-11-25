package towercontroller

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cdcontroller"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

// WaitForLoad waits for a loaded message from the C/D controller
type WaitForLoad struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cdcontroller.CellAPIClient
	Publisher     *protostream.Socket

	tbc             cdcontroller.TrayBarcode
	fxbc            cdcontroller.FixtureBarcode
	steps           cdcontroller.StepConfiguration
	stepType        string
	processStepName string
	transactID      string
	recipeVersion   int
	mockCellAPI     bool
	resetToIdle     bool
	unload          bool
	alarmed         tower.FireAlarmStatus

	fxrInfo *FixtureInfo

	err error
}

func (w *WaitForLoad) action() {
	w.fxrInfo.Avail.Set(StatusWaitingForLoad)
	w.resetToIdle = false

	w.Logger.Infow("waiting for load complete message from C/D controller", "fixture", w.fxbc)

	var fxrLoad cdcontroller.FXRLoad

poll:
	for {
		select {
		case <-w.fxrInfo.Unreserve: // unreserved via API
			w.Logger.Warn("waitforload: reservation manually removed")
			w.resetToIdle = true

			return
		case fxrLoad = <-w.fxrInfo.LDC: // load complete
			break poll
		default:
			msg, err := w.fxrInfo.FixtureState.GetOp()
			if err != nil {
				w.Logger.Warnw("get fixture operational message", "error", err)
				time.Sleep(time.Second)
				continue
			}

			if msg.GetOp().GetStatus() == tower.FixtureStatus_FIXTURE_STATUS_FAULTED && msg.GetOp().GetFireAlarmStatus() != tower.FireAlarmStatus_FIRE_ALARM_UNKNOWN_UNSPECIFIED {
				w.Logger.Warnw("fire alarm detected from fixture", "fixture", w.fxrInfo.Name, "alarm", msg.GetOp().GetFireAlarmStatus().String())

				// fire alarm, tell CDC
				// this is in-band because it will try _forever_ until it succeeds,
				w.resetToIdle = true // return to idle whether or not we successfully sounded alarm
				if err := soundTheAlarm(w.Config, msg.GetOp().GetFireAlarmStatus(), w.fxrInfo.Name, w.Logger); err != nil {
					// basically couldn't marshal the request. Return to idle where we will keep trying for as
					// long as the alarm exists
					return
				}

				// successfully alarmed, return to idle but set alarmed to true so we don't keep alarming
				w.alarmed = msg.GetOp().GetFireAlarmStatus()
				return
			}

			time.Sleep(time.Second)
		}
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
	w.stepType = fxrLoad.StepType
	w.transactID = fxrLoad.TransactionID

	if w.processStepName == "" || len(w.steps) == 0 {
		w.Logger.Error(fmt.Errorf("invalid fixture load message: %v", fxrLoad))
		w.unload = true

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
	case w.unload:
		w.Logger.Warn("going to unload state")

		next = &EndProcess{
			Config:          w.Config,
			Logger:          w.Logger,
			CellAPIClient:   w.CellAPIClient,
			Publisher:       w.Publisher,
			childLogger:     w.Logger.With("fixture", w.fxrInfo.Name),
			tbc:             w.tbc,
			fxbc:            w.fxbc,
			processStepName: w.processStepName,
			mockCellAPI:     w.mockCellAPI,
			recipeVersion:   w.recipeVersion,
			fxrInfo:         w.fxrInfo,
			skipClose:       true, // do not close the process step, error here
		}
	case w.err != nil || w.resetToIdle:
		w.Logger.Warnw("going back to idle state", "error", w.err, "reservation_cleared", w.resetToIdle)

		next = &Idle{
			Config:        w.Config,
			Logger:        w.Logger,
			CellAPIClient: w.CellAPIClient,
			Publisher:     w.Publisher,
			MockCellAPI:   w.mockCellAPI,
			FXRInfo:       w.fxrInfo,
			alarmed:       w.alarmed,
		}
	default:
		next = &StartProcess{
			Config:          w.Config,
			Logger:          w.Logger,
			CellAPIClient:   w.CellAPIClient,
			Publisher:       w.Publisher,
			processStepName: w.processStepName,
			transactID:      w.transactID,
			fxbc:            w.fxbc,
			tbc:             w.tbc,
			mockCellAPI:     w.mockCellAPI,
			steps:           w.steps,
			stepType:        w.stepType,
			recipeVersion:   w.recipeVersion,
			fxrInfo:         w.fxrInfo,
		}
	}

	w.Logger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

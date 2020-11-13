package towercontroller

import (
	"time"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cdcontroller"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

// InProcess monitors the FXR proto for the state to change
type InProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cdcontroller.CellAPIClient
	Publisher     *protostream.Socket

	childLogger     *zap.SugaredLogger
	tbc             cdcontroller.TrayBarcode
	fxbc            cdcontroller.FixtureBarcode
	processStepName string
	fixtureFault    bool
	mockCellAPI     bool
	returnToIdle    bool
	cells           map[string]cdcontroller.CellData
	cellResponse    []*tower.Cell
	recipeVersion   int

	fxrInfo *FixtureInfo
}

func (i *InProcess) action() {
	i.childLogger.Info("monitoring fixture to go to complete or fault")
	i.fxrInfo.Avail.Set(StatusActive)

	for { // loop until status updates to COMPLETE/FAULTED
		// TODO: first look for it to go to IN_PROGRESS
		//       then COMPLETE/FAULTED
		//       if it goes back to IDLE it lost the recipe, so unload
		var (
			msg *tower.FixtureToTower
			err error
		)

		if msg, err = i.fxrInfo.FixtureState.GetOp(); err != nil {
			i.childLogger.Warnw("get operational fixture status", "error", err)
			// wait a second for it to update
			// TODO: time out this operation. If fixture status doesn't update in a certain amount of time we should
			//       attempt to unload the tray.
			time.Sleep(time.Second)

			continue
		}

		i.childLogger.Debugw("got FixtureToTower message", "msg", msg.String())

		switch s := msg.GetOp().GetStatus(); s {
		case tower.FixtureStatus_FIXTURE_STATUS_COMPLETE, tower.FixtureStatus_FIXTURE_STATUS_FAULTED:
			statusMsg := "fixture done with tray"

			if i.fixtureFault = s == tower.FixtureStatus_FIXTURE_STATUS_FAULTED; i.fixtureFault {
				statusMsg += "; fixture faulted"
			}

			i.childLogger.Info(statusMsg)

			i.cellResponse = msg.GetOp().GetCells()

			return
		default:
			i.childLogger.Debugw("received fixture_status update", "status", s.String())
			// give it a second to update
			time.Sleep(time.Second)
		}
	}
}

// Actions returns the action functions for this state
func (i *InProcess) Actions() []func() {
	return []func(){
		i.action,
	}
}

// Next returns the next state to run
func (i *InProcess) Next() statemachine.State {
	var next statemachine.State
	if i.returnToIdle {
		next = &Idle{
			Config:        i.Config,
			Logger:        i.Logger,
			CellAPIClient: i.CellAPIClient,
			Publisher:     i.Publisher,
			MockCellAPI:   i.mockCellAPI,
			FXRInfo:       i.fxrInfo,
		}
	} else {
		next = &EndProcess{
			Config:          i.Config,
			Logger:          i.Logger,
			CellAPIClient:   i.CellAPIClient,
			Publisher:       i.Publisher,
			childLogger:     i.childLogger,
			tbc:             i.tbc,
			fxbc:            i.fxbc,
			processStepName: i.processStepName,
			fixtureFault:    i.fixtureFault,
			cellResponse:    i.cellResponse,
			cells:           i.cells,
			mockCellAPI:     i.mockCellAPI,
			recipeVersion:   i.recipeVersion,
			fxrInfo:         i.fxrInfo,
		}
	}

	i.childLogger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

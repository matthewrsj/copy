package towercontroller

import (
	"fmt"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/traycontrollers"
)

// Idle waits for a PreparedForLoad (or loaded to short circuit) from C/D controller
type Idle struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client

	Manual      bool
	MockCellAPI bool

	next statemachine.State
	err  error

	FXRInfo *FixtureInfo
}

func (i *Idle) action() {
	i.FXRInfo.Avail.Set(StatusWaitingForReservation)

	// block until we receive a load complete or a prepared for delivery signal
	select {
	case pfd := <-i.FXRInfo.PFD: // prepared for delivery, next state WaitForLoad
		tbc, fxbc, err := newIDs(pfd.Tray, pfd.Fixture)
		if err != nil {
			i.err = fmt.Errorf("parse IDs: %v", err)
			i.Logger.Error(err)

			return
		}

		i.next = &WaitForLoad{
			Config:        i.Config,
			Logger:        i.Logger,
			CellAPIClient: i.CellAPIClient,
			tbc:           tbc,
			fxbc:          fxbc,
			manual:        i.Manual,
			mockCellAPI:   i.MockCellAPI,
			fxrInfo:       i.FXRInfo,
		}
	case fxrLoad := <-i.FXRInfo.LDC: // load complete (we missed something), next state ProcessStep
		fxrID := fmt.Sprintf("%s-%s%s-%02d-%02d", i.Config.Loc.Line, i.Config.Loc.Process, i.Config.Loc.Aisle, fxrLoad.Column, fxrLoad.Level)

		tbc, fxbc, err := newIDs(fxrLoad.TrayID, fxrID)
		if err != nil {
			i.err = fmt.Errorf("parse IDs: %v", err)
			i.Logger.Error(err)

			return
		}

		// TODO: short circuit to in-progress if fixture status is active (or complete?)
		i.next = &ProcessStep{
			Config:        i.Config,
			Logger:        i.Logger,
			CellAPIClient: i.CellAPIClient,
			mockCellAPI:   i.MockCellAPI,
			fxrInfo:       i.FXRInfo,
		}

		i.next.SetContext(Barcodes{
			Fixture:         fxbc,
			Tray:            tbc,
			ProcessStepName: fxrLoad.RecipeName,
			MockCellAPI:     i.MockCellAPI,
			RecipeName:      fxrLoad.RecipeName,
			RecipeVersion:   fxrLoad.RecipeVersion,
			StepConf:        fxrLoad.Steps,
		})
	}
}

// Actions returns the action functions for this state
func (i *Idle) Actions() []func() {
	return []func(){
		i.action,
	}
}

// Next returns the state to run after this one
func (i *Idle) Next() statemachine.State {
	if i.err != nil {
		i.Logger.Warnw("going back to idle state", "error", i.err)
		i.err = nil
		i.next = i
	}

	i.Logger.Debugw("transitioning to next state", "next", statemachine.NameOf(i.next))

	return i.next
}

func newIDs(tray, fixture string) (tbc traycontrollers.TrayBarcode, fxbc traycontrollers.FixtureBarcode, err error) {
	tbc, err = traycontrollers.NewTrayBarcode(tray)
	if err != nil {
		err = fmt.Errorf("parse tray ID: %v", err)
		return
	}

	fxbc, err = traycontrollers.NewFixtureBarcode(fixture)
	if err != nil {
		err = fmt.Errorf("parse fixture ID: %v", err)
		return
	}

	return
}

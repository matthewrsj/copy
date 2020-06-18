package towercontroller

import (
	"fmt"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/traycontrollers"
)

// ProcessStep exists entirely to type assert the Context() into
// the tray and fixture barcodes and process step name.
type ProcessStep struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client

	processStepName string
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	inProgress      bool
	manual          bool
	mockCellAPI     bool
}

func (p *ProcessStep) action() {
	p.Logger.Debug("setting tbc and fxbc from context")

	bc, ok := p.Context().(Barcodes)
	if !ok {
		fatalError(p, p.Logger, fmt.Errorf("state context %v (%T) was not correct type (Barcodes)", p.Context(), p.Context()))
		return
	}

	p.manual = bc.ManualMode
	p.mockCellAPI = bc.MockCellAPI

	if !p.manual {
		// in manual mode this is handled in the barcode scan step so the operator can confirm
		// the correct process step name
		if !p.mockCellAPI {
			var err error
			if bc.ProcessStepName, err = p.CellAPIClient.GetNextProcessStep(bc.Tray.SN); err != nil {
				fatalError(p, p.Logger, fmt.Errorf("GetNextProcessStep for %s: %v", bc.Tray.SN, err))
				return
			}
		} else {
			p.Logger.Warnf("cell API mocked, skipping GetNextProcessStep and using %s", _mockedFormRequest)
			bc.ProcessStepName = _mockedFormRequest
		}
	}

	p.tbc = bc.Tray
	p.fxbc = bc.Fixture
	p.processStepName = bc.ProcessStepName
	p.inProgress = bc.InProgress

	p.Logger = p.Logger.With(
		"tray", p.tbc.SN,
		"fixture", p.fxbc.Raw,
		"process_step", p.processStepName,
	)
	p.Logger.Info("running process step")
}

// Actions returns the action functions for this state
func (p *ProcessStep) Actions() []func() {
	return []func(){
		p.action,
	}
}

// Next returns the next state to run
func (p *ProcessStep) Next() statemachine.State {
	var next statemachine.State

	if p.inProgress {
		// if this tray was discovered to already be in-progress skip right to monitoring the status
		next = &InProcess{
			Config:          p.Config,
			Logger:          p.Logger,
			CellAPIClient:   p.CellAPIClient,
			processStepName: p.processStepName,
			tbc:             p.tbc,
			fxbc:            p.fxbc,
			manual:          p.manual,
			mockCellAPI:     p.mockCellAPI,
		}
	} else {
		next = &ReadRecipe{
			Config:          p.Config,
			Logger:          p.Logger,
			CellAPIClient:   p.CellAPIClient,
			processStepName: p.processStepName,
			tbc:             p.tbc,
			fxbc:            p.fxbc,
			manual:          p.manual,
			mockCellAPI:     p.mockCellAPI,
		}
	}

	p.Logger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

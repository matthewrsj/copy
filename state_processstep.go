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

	childLogger     *zap.SugaredLogger
	processStepName string
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	steps           traycontrollers.StepConfiguration
	recipeVersion   int
	inProgress      bool
	manual          bool
	mockCellAPI     bool

	fxrInfo *FixtureInfo
}

func (p *ProcessStep) action() {
	p.fxrInfo.Avail.Set(StatusActive)

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
		// the correct process step name. In C-Tower and on this is just the recipe name from CND
		bc.ProcessStepName = bc.RecipeName
		// we also get steps and recipe version from CND
		p.steps = bc.StepConf
		p.recipeVersion = bc.RecipeVersion
	}

	p.tbc = bc.Tray
	p.fxbc = bc.Fixture
	p.processStepName = bc.ProcessStepName
	p.inProgress = bc.InProgress

	p.childLogger = p.Logger.With(
		"tray", p.tbc.SN,
		"fixture", p.fxbc.Raw,
		"process_step", p.processStepName,
	)

	p.childLogger.Info("running process step")
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

	switch {
	case p.inProgress:
		// if this tray was discovered to already be in-progress skip right to monitoring the status
		next = &InProcess{
			Config:          p.Config,
			Logger:          p.Logger,
			CellAPIClient:   p.CellAPIClient,
			childLogger:     p.childLogger,
			processStepName: p.processStepName,
			tbc:             p.tbc,
			fxbc:            p.fxbc,
			manual:          p.manual,
			mockCellAPI:     p.mockCellAPI,
			recipeVersion:   p.recipeVersion,
			fxrInfo:         p.fxrInfo,
		}
	case p.manual:
		// if this is manual we need to load the recipe locally
		next = &ReadRecipe{
			Config:          p.Config,
			Logger:          p.Logger,
			CellAPIClient:   p.CellAPIClient,
			childLogger:     p.childLogger,
			processStepName: p.processStepName,
			tbc:             p.tbc,
			fxbc:            p.fxbc,
			manual:          p.manual,
			mockCellAPI:     p.mockCellAPI,
			fxrInfo:         p.fxrInfo,
		}
	default:
		// not in progress, not in manual mode (don't need to load recipe)
		next = &StartProcess{
			Config:          p.Config,
			Logger:          p.Logger,
			CellAPIClient:   p.CellAPIClient,
			childLogger:     p.childLogger,
			processStepName: p.processStepName,
			fxbc:            p.fxbc,
			tbc:             p.tbc,
			manual:          p.manual,
			mockCellAPI:     p.mockCellAPI,
			steps:           p.steps,
			recipeVersion:   p.recipeVersion,
			fxrInfo:         p.fxrInfo,
		}
	}

	p.Logger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

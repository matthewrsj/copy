package towercontroller

import (
	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/protostream"
	"stash.teslamotors.com/rr/traycontrollers"
)

// ReadRecipe reads the recipe from configuration files
type ReadRecipe struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client
	Publisher     *protostream.Socket

	childLogger     *zap.SugaredLogger
	processStepName string
	transactID      string
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	steps           traycontrollers.StepConfiguration
	recipeVersion   int
	rcpErr          error
	smFatal         bool
	manual          bool
	mockCellAPI     bool
	fxrInfo         *FixtureInfo
}

func (r *ReadRecipe) action() {
	r.childLogger.Info("loading recipe for process step")

	if r.steps, r.rcpErr = LoadRecipe(r.Config.RecipeFile, r.Config.IngredientsFile, r.processStepName); r.rcpErr != nil {
		r.childLogger.Errorw("load recipe", "error", r.rcpErr)
		r.smFatal = true

		return
	}

	r.childLogger.Debug("loaded recipe")
}

// Actions returns the action functions for this state
func (r *ReadRecipe) Actions() []func() {
	return []func(){
		r.action,
	}
}

// Next returns the next state to run after this one
func (r *ReadRecipe) Next() statemachine.State {
	var next statemachine.State

	switch {
	case r.smFatal:
		next = &Idle{
			Config:        r.Config,
			Logger:        r.Logger,
			CellAPIClient: r.CellAPIClient,
			Publisher:     r.Publisher,
			Manual:        r.manual,
			MockCellAPI:   r.mockCellAPI,
			FXRInfo:       r.fxrInfo,
		}
	default:
		next = &StartProcess{
			Config:          r.Config,
			Logger:          r.Logger,
			CellAPIClient:   r.CellAPIClient,
			Publisher:       r.Publisher,
			childLogger:     r.childLogger,
			processStepName: r.processStepName,
			transactID:      r.transactID,
			fxbc:            r.fxbc,
			tbc:             r.tbc,
			steps:           r.steps,
			manual:          r.manual,
			mockCellAPI:     r.mockCellAPI,
			recipeVersion:   r.recipeVersion,
			fxrInfo:         r.fxrInfo,
		}
	}

	r.childLogger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

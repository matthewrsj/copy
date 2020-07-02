package towercontroller

import (
	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/traycontrollers"
)

// ReadRecipe reads the recipe from configuration files
type ReadRecipe struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client

	processStepName string
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	steps           traycontrollers.StepConfiguration
	recipeVersion   int
	rcpErr          error
	manual          bool
	mockCellAPI     bool
	fxrInfo         *FixtureInfo
}

func (r *ReadRecipe) action() {
	r.Logger.Info("loading recipe for process step")

	if r.steps, r.rcpErr = LoadRecipe(r.Config.RecipeFile, r.Config.IngredientsFile, r.processStepName); r.rcpErr != nil {
		fatalError(r, r.Logger, r.rcpErr)
		return
	}

	r.Logger.Debug("loaded recipe")
}

// Actions returns the action functions for this state
func (r *ReadRecipe) Actions() []func() {
	return []func(){
		r.action,
	}
}

// Next returns the next state to run after this one
func (r *ReadRecipe) Next() statemachine.State {
	next := &StartProcess{
		Config:          r.Config,
		Logger:          r.Logger,
		CellAPIClient:   r.CellAPIClient,
		processStepName: r.processStepName,
		fxbc:            r.fxbc,
		tbc:             r.tbc,
		steps:           r.steps,
		manual:          r.manual,
		mockCellAPI:     r.mockCellAPI,
		recipeVersion:   r.recipeVersion,
		fxrInfo:         r.fxrInfo,
	}
	r.Logger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

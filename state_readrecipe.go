package towercontroller

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/traycontrollers"
)

// ReadRecipe reads the recipe from configuration files
type ReadRecipe struct {
	statemachine.Common

	Config        traycontrollers.Configuration
	Logger        *logrus.Logger
	CellAPIClient *cellapi.Client

	processStepName string
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	rcpe            []ingredients
	rcpErr          error
}

func (r *ReadRecipe) action() {
	r.Logger.WithFields(logrus.Fields{
		"tray":         r.tbc.SN,
		"fixture_num":  r.fxbc.Raw,
		"process_step": r.processStepName,
	}).Info("loading recipe for process step")

	if r.rcpe, r.rcpErr = loadRecipe(r.Config.RecipeFile, r.Config.IngredientsFile, r.processStepName); r.rcpErr != nil {
		fatalError(r, r.Logger, r.rcpErr)
		return
	}

	r.Logger.WithFields(logrus.Fields{
		"tray":         r.tbc.SN,
		"fixture_num":  r.fxbc.Raw,
		"process_step": r.processStepName,
		"recipe":       fmt.Sprintf("%#v", r.rcpe),
	}).Debug("loaded recipe")
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
		rcpe:            r.rcpe,
	}
	r.Logger.WithField("tray", r.tbc.SN).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

package towercontroller

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

type ReadRecipe struct {
	statemachine.Common

	Config Configuration
	Logger *logrus.Logger

	processStepName string
	tbc             trayBarcode
	fxbc            fixtureBarcode
	rcpe            []ingredients
	rcpErr          error
}

func (r *ReadRecipe) action() {
	r.Logger.WithFields(logrus.Fields{
		"tray":         r.tbc.sn,
		"fixture_num":  r.fxbc.raw,
		"process_step": r.processStepName,
	}).Info("loading recipe for process step")

	if r.rcpe, r.rcpErr = loadRecipe(r.Config.RecipeFile, r.Config.IngredientsFile, r.processStepName); r.rcpErr != nil {
		r.Logger.Error(r.rcpErr)
		r.SetLast(true)

		return
	}

	r.Logger.WithFields(logrus.Fields{
		"tray":         r.tbc.sn,
		"fixture_num":  r.fxbc.raw,
		"process_step": r.processStepName,
		"recipe":       fmt.Sprintf("%#v", r.rcpe),
	}).Debug("loaded recipe")
}

func (r *ReadRecipe) Actions() []func() {
	return []func(){
		r.action,
	}
}

func (r *ReadRecipe) Next() statemachine.State {
	next := &StartProcess{
		Config:          r.Config,
		Logger:          r.Logger,
		processStepName: r.processStepName,
		fxbc:            r.fxbc,
		tbc:             r.tbc,
		rcpe:            r.rcpe,
	}
	r.Logger.WithField("tray", r.tbc.sn).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

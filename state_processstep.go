package towercontroller

import (
	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

type ProcessStep struct {
	statemachine.Common

	Config Configuration
	Logger *logrus.Logger

	processStepName string
	tbc             trayBarcode
	fxbc            fixtureBarcode
	apiErr          error
}

func (p *ProcessStep) action() {
	p.Logger.WithFields(logrus.Fields{
		"tray":        p.tbc.sn,
		"fixture_num": p.fxbc.raw,
	}).Info("querying Cell API for process step")

	p.processStepName, p.apiErr = getNextProcessStep(p.Config.CellAPI, p.tbc.sn)
	if p.apiErr != nil {
		p.Logger.Error(p.apiErr)
		p.SetLast(true)

		return
	}

	p.Logger.WithFields(logrus.Fields{
		"tray":         p.tbc.sn,
		"fixture_num":  p.fxbc.raw,
		"process_step": p.processStepName,
	}).Info("running process step")
}

func (p *ProcessStep) Actions() []func() {
	return []func(){
		p.action,
	}
}

func (p *ProcessStep) Next() statemachine.State {
	next := &ReadRecipe{
		Config:          p.Config,
		Logger:          p.Logger,
		processStepName: p.processStepName,
		tbc:             p.tbc,
		fxbc:            p.fxbc,
	}
	p.Logger.WithField("tray", p.tbc.sn).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

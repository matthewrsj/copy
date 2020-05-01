package towercontroller

import (
	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine"
)

type ProcessStep struct {
	statemachine.Common

	Logger *logrus.Logger

	tbc  trayBarcode
	fxbc fixtureBarcode
}

func (p *ProcessStep) action() {}

func (p *ProcessStep) Actions() []func() {
	return []func(){
		p.action,
	}
}

func (p *ProcessStep) Next() statemachine.State {
	next := &StartProcess{
		Logger: p.Logger,
		tbc:    p.tbc,
		fxbc:   p.fxbc,
	}
	p.Logger.WithField("tray", p.tbc.sn).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

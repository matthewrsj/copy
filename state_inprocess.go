package towercontroller

import (
	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

type InProcess struct {
	statemachine.Common

	Logger *logrus.Logger

	tbc  trayBarcode
	fxbc fixtureBarcode
}

func (i *InProcess) action() {}

func (i *InProcess) Actions() []func() {
	return []func(){
		i.action,
	}
}

func (i *InProcess) Next() statemachine.State {
	next := &EndProcess{
		Logger: i.Logger,
		tbc:    i.tbc,
		fxbc:   i.fxbc,
	}
	i.Logger.WithField("tray", i.tbc.sn).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

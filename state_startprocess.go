package towercontroller

import (
	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine"
)

type StartProcess struct {
	statemachine.Common

	Logger *logrus.Logger

	tbc  trayBarcode
	fxbc fixtureBarcode
}

func (s *StartProcess) action() {}

func (s *StartProcess) Actions() []func() {
	return []func(){
		s.action,
	}
}

func (s *StartProcess) Next() statemachine.State {
	next := &InProcess{
		Logger: s.Logger,
		tbc:    s.tbc,
		fxbc:   s.fxbc,
	}
	s.Logger.WithField("tray", s.tbc.sn).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

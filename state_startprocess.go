package towercontroller

import (
	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

type StartProcess struct {
	statemachine.Common

	Config Configuration
	Logger *logrus.Logger

	processStepName string
	tbc             trayBarcode
	fxbc            fixtureBarcode
	rcpe            []ingredients
}

func (s *StartProcess) action() {
	s.Logger.WithFields(logrus.Fields{
		"tray":         s.tbc.sn,
		"fixture_num":  s.fxbc.raw,
		"process_step": s.processStepName,
	}).Info("sending recipe and other information to FXR")

	// TODO: send proto to FXR

	s.Logger.WithFields(logrus.Fields{
		"tray":         s.tbc.sn,
		"fixture_num":  s.fxbc.raw,
		"process_step": s.processStepName,
	}).Trace("sent recipe and other information to FXR")
}

func (s *StartProcess) Actions() []func() {
	return []func(){
		s.action,
	}
}

func (s *StartProcess) Next() statemachine.State {
	next := &InProcess{
		Config: s.Config,
		Logger: s.Logger,
		tbc:    s.tbc,
		fxbc:   s.fxbc,
	}
	s.Logger.WithField("tray", s.tbc.sn).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

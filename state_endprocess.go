package towercontroller

import (
	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

type EndProcess struct {
	statemachine.Common

	Logger *logrus.Logger

	tbc  trayBarcode
	fxbc fixtureBarcode
}

func (e *EndProcess) action() {
	e.Logger.WithFields(logrus.Fields{
		"tray":    e.tbc.raw,
		"fixture": e.fxbc.raw,
	}).Infof("tray complete")
}

func (e *EndProcess) Actions() []func() {
	e.SetLast(true)

	return []func(){
		e.action,
	}
}

func (e *EndProcess) Next() statemachine.State {
	e.Logger.WithField("tray", e.tbc.sn).Trace("statemachine exiting")
	return nil
}

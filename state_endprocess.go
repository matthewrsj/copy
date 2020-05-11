package towercontroller

import (
	"log"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

type EndProcess struct {
	statemachine.Common

	Config Configuration
	Logger *logrus.Logger

	tbc             TrayBarcode
	fxbc            FixtureBarcode
	processStepName string
	fixtureFault    bool
}

func (e *EndProcess) action() {
	if err := updateProcessStatus(e.Config.CellAPI, e.tbc.SN, e.processStepName, _statusEnd); err != nil {
		e.Logger.Error(err)
		log.Println(err)
		e.SetLast(true)

		return
	}

	// TODO: determine how to inform cell API of fault
	msg := "tray complete"
	if e.fixtureFault {
		msg += "; fixture faulted"
	}

	e.Logger.WithFields(logrus.Fields{
		"tray":    e.tbc.raw,
		"fixture": e.fxbc.raw,
	}).Infof(msg)
}

func (e *EndProcess) Actions() []func() {
	e.SetLast(true)

	return []func(){
		e.action,
	}
}

func (e *EndProcess) Next() statemachine.State {
	e.Logger.WithField("tray", e.tbc.SN).Trace("statemachine exiting")
	return nil
}

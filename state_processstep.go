package towercontroller

import (
	"fmt"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/traycontrollers"
)

// ProcessStep exists entirely to type assert the Context() into
// the tray and fixture barcodes and process step name.
type ProcessStep struct {
	statemachine.Common

	Config        traycontrollers.Configuration
	Logger        *logrus.Logger
	CellAPIClient *cellapi.Client

	processStepName string
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	inProgress      bool
}

func (p *ProcessStep) action() {
	p.Logger.Trace("setting tbc and fxbc from context")

	bc, ok := p.Context().(Barcodes)
	if !ok {
		fatalError(p, p.Logger, fmt.Errorf("state context %v (%T) was not correct type (Barcodes)", p.Context(), p.Context()))
		return
	}

	p.tbc = bc.Tray
	p.fxbc = bc.Fixture
	p.processStepName = bc.ProcessStepName
	p.inProgress = bc.InProgress

	p.Logger.WithFields(logrus.Fields{
		"tray":         p.tbc.SN,
		"fixture_num":  p.fxbc.Raw,
		"process_step": p.processStepName,
	}).Info("running process step")
}

// Actions returns the action functions for this state
func (p *ProcessStep) Actions() []func() {
	return []func(){
		p.action,
	}
}

// Next returns the next state to run
func (p *ProcessStep) Next() statemachine.State {
	var next statemachine.State

	if p.inProgress {
		// if this tray was discovered to already be in-progress skip right to monitoring the status
		next = &InProcess{
			Config:          p.Config,
			Logger:          p.Logger,
			CellAPIClient:   p.CellAPIClient,
			processStepName: p.processStepName,
			tbc:             p.tbc,
			fxbc:            p.fxbc,
		}
	} else {
		next = &ReadRecipe{
			Config:          p.Config,
			Logger:          p.Logger,
			CellAPIClient:   p.CellAPIClient,
			processStepName: p.processStepName,
			tbc:             p.tbc,
			fxbc:            p.fxbc,
		}
	}

	p.Logger.WithField("tray", p.tbc.SN).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

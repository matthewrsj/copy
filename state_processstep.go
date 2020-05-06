package towercontroller

import (
	"fmt"
	"log"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

// ProcessStep exists entirely to type assert the Context() into
// the tray and fixture barcodes and process step name.
type ProcessStep struct {
	statemachine.Common

	Config Configuration
	Logger *logrus.Logger

	processStepName string
	tbc             TrayBarcode
	fxbc            FixtureBarcode
	apiErr          error
}

func (p *ProcessStep) action() {
	p.Logger.Trace("setting tbc and fxbc from context")

	bc, ok := p.Context().(Barcodes)
	if !ok {
		p.apiErr = fmt.Errorf("state context %v (%T) was not correct type (Barcodes)", p.Context(), p.Context())
		p.Logger.Error(p.apiErr)
		log.Println(p.apiErr)
		p.SetLast(true)

		return
	}

	p.tbc = bc.Tray
	p.fxbc = bc.Fixture
	p.processStepName = bc.ProcessStepName

	p.Logger.WithFields(logrus.Fields{
		"tray":         p.tbc.SN,
		"fixture_num":  p.fxbc.raw,
		"process_step": p.processStepName,
	}).Info("querying Cell API for process step")

	p.Logger.WithFields(logrus.Fields{
		"tray":         p.tbc.SN,
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
	p.Logger.WithField("tray", p.tbc.SN).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

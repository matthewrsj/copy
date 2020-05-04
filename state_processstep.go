package towercontroller

import (
	"fmt"
	"log"
	"time"

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

	msg := fmt.Sprintf("NEXT PROCESS STEP %s; press enter to confirm; any other key + enter to cancel", p.processStepName)

	confirm, err := promptDeadline(msg, time.Now().Add(time.Second*10))
	if err != nil {
		if !isDeadline(err) {
			p.Logger.Error(err)
			p.apiErr = err

			return
		}

		fmt.Println() // clear the prompt line
		log.Println("process step confirmation timed out, continuing")
		p.Logger.Warn("process step confirmation timed out, continuing")
	}

	switch confirm {
	case "":
	default:
		log.Printf("process step canceled with %s\n", confirm)
		p.Logger.Errorf("process step canceled with %s", confirm)
		p.apiErr = err

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

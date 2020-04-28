package towercontroller

import "stash.teslamotors.com/ctet/statemachine"

type ProcessStep struct {
	statemachine.Common
}

func (p *ProcessStep) action() {}

func (p *ProcessStep) Actions() []func() {
	return []func(){
		p.action,
	}
}

func (p *ProcessStep) Next() statemachine.State {
	return &StartProcess{}
}

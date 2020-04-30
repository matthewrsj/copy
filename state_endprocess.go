package towercontroller

import "stash.teslamotors.com/ctet/statemachine"

type EndProcess struct {
	statemachine.Common
}

func (e *EndProcess) action() {}

func (e *EndProcess) Actions() []func() {
	e.SetLast(true)

	return []func(){
		e.action,
	}
}

func (e *EndProcess) Next() statemachine.State {
	return &EndProcess{}
}

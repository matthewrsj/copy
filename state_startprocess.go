package towercontroller

import "stash.teslamotors.com/ctet/statemachine"

type StartProcess struct {
	statemachine.Common
}

func (s *StartProcess) action() {}

func (s *StartProcess) Actions() []func() {
	return []func(){
		s.action,
	}
}

func (s *StartProcess) Next() statemachine.State {
	return &InProcess{}
}

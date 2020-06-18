package towercontroller

import (
	"log"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

func fatalError(s statemachine.State, l *zap.SugaredLogger, err error) {
	s.SetLast(true)
	log.Println(err)
	l.Error(err)
}

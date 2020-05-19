package towercontroller

import (
	"log"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
)

func fatalError(s statemachine.State, l *logrus.Logger, err error) {
	s.SetLast(true)
	log.Println(err)
	l.Error(err)
}

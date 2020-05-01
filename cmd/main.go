package main

import (
	"flag"
	"log"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/cmdlineutils"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/towercontroller"
)

const (
	_logLvlDef  = logrus.InfoLevel
	_logFileDef = "logs/towercontroller/statemachine.log"
)

func main() {
	logLvl := cmdlineutils.LogLevelFlag()
	logFile := flag.String("logf", _logFileDef, "path to the log file")

	flag.Parse()

	lvl, err := cmdlineutils.ParseLogLevelWithDefault(*logLvl, _logLvlDef)
	if err != nil {
		log.Printf("%v; setting log level to default %s", err, _logLvlDef.String())
	}

	logger, err := newLogger(*logFile, lvl)
	if err != nil {
		log.Fatalf("setup logger: %v", err)
	}

	logger.Info("starting state machine")
	statemachine.RunFrom(&towercontroller.TrayBarcode{Logger: logger})
}

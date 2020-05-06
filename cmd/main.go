package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/cmdlineutils"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/towercontroller"
)

const (
	_logLvlDef   = logrus.InfoLevel
	_logFileDef  = "logs/towercontroller/statemachine.log"
	_confFileDef = "../configuration/statemachine/statemachine.yaml"
)

// main is long because of logging and error handling, not complicated logic
// nolint:funlen
func main() {
	logLvl := cmdlineutils.LogLevelFlag()
	logFile := flag.String("logf", _logFileDef, "path to the log file")
	configFile := flag.String("conf", _confFileDef, "path to the configuration file")

	flag.Parse()

	lvl, err := cmdlineutils.ParseLogLevelWithDefault(*logLvl, _logLvlDef)
	if err != nil {
		log.Printf("%v; setting log level to default %s", err, _logLvlDef.String())
	}

	logger, err := newLogger(*logFile, lvl)
	if err != nil {
		log.Fatalf("setup logger: %v", err)
	}

	conf, err := towercontroller.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	s := statemachine.NewScheduler()

	for _, fixture := range conf.Fixtures {
		s.Register(fixture, &towercontroller.ProcessStep{Config: conf, Logger: logger}, nil /* runner (default) */)
	}

	logger.Info("starting state machine scheduler")

	for {
		logger.Info("waiting for tray barcode scan")

		barcodes, err := towercontroller.ScanBarcodes(conf)
		if err != nil {
			if towercontroller.IsInterrupt(err) {
				log.Fatal("received CTRL-C, exiting...")
			}

			err = fmt.Errorf("scan barcodes: %v", err)
			logger.Error(err)
			log.Println(err)

			continue
		}

		logger.WithFields(logrus.Fields{
			"tray_sn":          barcodes.Tray.SN,
			"tray_orientation": barcodes.Tray.O,
			"fixture_location": barcodes.Fixture.Location,
			"fixture_aisle":    barcodes.Fixture.Aisle,
			"fixture_tower":    barcodes.Fixture.Tower,
			"fixture_num":      barcodes.Fixture.Fxn,
		}).Info("starting state machine")

		if err := s.Schedule(statemachine.Job{Name: barcodes.Fixture.Fxn, Work: barcodes}); err != nil {
			err := fmt.Errorf("schedule tray job on fixture: %v", err)
			logger.Error(err)
			log.Println(err)

			continue
		}

		log.Println("starting tray state machine")
	}
}
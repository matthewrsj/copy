//+build !test

// main runs the tower controller application.
package main

import (
	"flag"
	"log"
	"net/http"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/towercontroller"
)

const (
	_logLvlDef   = zapcore.InfoLevel
	_logFileDef  = "logs/towercontroller/statemachine.log"
	_confFileDef = "../configuration/statemachine/statemachine.yaml"
	_localDef    = ":13167"
)

func main() {
	logLvl := zap.LevelFlag("loglvl", _logLvlDef, "log level for zap logger")
	logFile := flag.String("logf", _logFileDef, "path to the log file")
	configFile := flag.String("conf", _confFileDef, "path to the configuration file")
	localAddr := flag.String("local", _localDef, "local address")
	manual := flag.Bool("manual", false, "turn on manual mode (i.e. for SWIFT line)")
	mockCellAPI := flag.Bool("mockapi", false, "mock Cell API interactions")

	flag.Parse()

	logConfig := newLogger(*logFile, *logLvl)

	logger, err := logConfig.Build()
	if err != nil {
		log.Fatalf("build log configuration: %v", err)
	}

	sugar := logger.Sugar()

	conf, err := towercontroller.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("load configuration: %v", err)
	}

	// use normal base URL unless we are running SWIFT (manual) mode
	base := conf.CellAPI.Base
	if *manual {
		base = conf.CellAPI.BaseSWIFT
	}

	caClient := cellapi.NewClient(base,
		cellapi.WithNextProcessStepFmtEndpoint(conf.CellAPI.Endpoints.NextProcStepFmt),
		cellapi.WithProcessStatusFmtEndpoint(conf.CellAPI.Endpoints.ProcessStatusFmt),
		cellapi.WithCellMapFmtEndpoint(conf.CellAPI.Endpoints.CellMapFmt),
		cellapi.WithCellStatusEndpoint(conf.CellAPI.Endpoints.CellStatus),
	)

	s := statemachine.NewScheduler()

	for n := range conf.Fixtures {
		sugar.Infow("registering", "fixture", n)
		s.Register(n,
			&towercontroller.ProcessStep{
				Config:        conf,
				Logger:        sugar,
				CellAPIClient: caClient,
			}, nil, // runner (default)
		)
	}

	sugar.Info("monitoring for in-progress trays")

	jch := make(chan statemachine.Job)

	var wg sync.WaitGroup

	wg.Add(len(conf.Fixtures))

	for _, id := range conf.Fixtures {
		go func(id uint32) {
			defer wg.Done()

			job, err := monitorForInProgress(conf, id, *manual, *mockCellAPI)
			if err != nil {
				sugar.Errorw("monitor for in-progress trays", "error", err)
				return
			}

			if job.Name == "" {
				// no job found within timeout
				return
			}

			jch <- job
		}(id)
	}

	done := make(chan struct{})

	go func() {
		for job := range jch {
			sugar.Infow("found in-progress tray", "fixture", job.Name)

			if err := s.Schedule(job); err != nil {
				sugar.Fatalw("schedule in-progress tray", "error", err)
			}
		}

		close(done)
	}()
	wg.Wait()
	close(jch)
	<-done // wait for all jobs to be read

	if *manual {
		handleManualOperation(s, sugar, caClient, *mockCellAPI)
		return
	} // end of manual operation

	// handle incoming requests on availability
	towercontroller.HandleAvailable(conf, sugar)

	// handle incoming posts to load
	load := make(chan statemachine.Job)
	towercontroller.HandleLoad(conf, load, sugar, *mockCellAPI)

	go func() {
		if err := http.ListenAndServe(*localAddr, nil); err != nil {
			sugar.Errorw("http.ListenAndServe", "error", err)
		}
	}()

	sugar.Info("starting state machine scheduler")

	for job := range load {
		sugar.Infow("got tray load", "fixture", job.Name, "tray_info", job.Work)

		if err := s.Schedule(job); err != nil {
			sugar.Errorw("schedule tray job on fixture", "error", err)
			continue
		}

		sugar.Info("starting tray state machine", "fixture", job.Name)
	}
}

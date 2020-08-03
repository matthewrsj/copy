//+build !test

// main runs the tower controller application.
package main

import (
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/cenkalti/backoff"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/protostream"
	"stash.teslamotors.com/rr/towercontroller"
	"stash.teslamotors.com/rr/traycontrollers"
)

const (
	_logLvlDef   = zapcore.InfoLevel
	_logFileDef  = "logs/towercontroller/statemachine.log"
	_confFileDef = "/etc/towercontroller.d/statemachine.yaml"
	_localDef    = "0.0.0.0:13163"
)

// nolint:funlen // main func
func main() {
	logLvl := zap.LevelFlag("loglvl", _logLvlDef, "log level for zap logger")
	logFile := flag.String("logf", _logFileDef, "path to the log file")
	configFile := flag.String("conf", _confFileDef, "path to the configuration file")
	localAddr := flag.String("local", _localDef, "local address")
	manual := flag.Bool("manual", false, "turn on manual mode (i.e. for SWIFT line)")
	mockCellAPI := flag.Bool("mockapi", false, "mock Cell API interactions")
	wsAddr := flag.String("wsaddr", protostream.DefaultWebsocketAddress, "websocket address for proto")

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

	go towercontroller.MonitorConfig(sugar, *configFile, &conf)

	// use normal base URL unless we are running SWIFT (manual) mode
	base := conf.CellAPI.Base
	if *manual {
		base = conf.CellAPI.BaseSWIFT
	}

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	caClient := cellapi.NewClient(base,
		cellapi.WithNextProcessStepFmtEndpoint(conf.CellAPI.Endpoints.NextProcStepFmt),
		cellapi.WithProcessStatusFmtEndpoint(conf.CellAPI.Endpoints.ProcessStatusFmt),
		cellapi.WithCellMapFmtEndpoint(conf.CellAPI.Endpoints.CellMapFmt),
		cellapi.WithCellStatusEndpoint(conf.CellAPI.Endpoints.CellStatus),
		cellapi.WithCloseProcessFmtEndpoint(conf.CellAPI.Endpoints.CloseProcessFmt),
	)

	s := statemachine.NewScheduler()

	for _, name := range conf.AllFixtures {
		sugar.Infow("registering", "fixture", name)
		s.Register(name,
			&towercontroller.ProcessStep{
				Config:        conf,
				Logger:        sugar,
				CellAPIClient: caClient,
			}, nil, // runner (default)
		)
	}

	if *manual {
		handleManualOperation(s, sugar, caClient, *mockCellAPI)
		return
	} // end of manual operation

	registry := make(map[string]*towercontroller.FixtureInfo)

	for _, name := range conf.AllFixtures {
		u := url.URL{Scheme: "ws", Host: *wsAddr, Path: protostream.WSEndpoint}
		n := name

		var sub *protostream.Socket

		if err = backoff.Retry(
			func() error {
				sub, err = protostream.NewSubscriberWithSub(u.String(), n)
				if err != nil {
					sugar.Errorw("create new subscriber", "error", err)
					return err
				}

				return nil
			},
			backoff.NewExponentialBackOff(),
		); err != nil {
			sugar.Fatalw("create new subscriber", "error", err)
		}

		lc := sub.Listen()

		registry[name] = &towercontroller.FixtureInfo{
			Name: name,
			PFD:  make(chan traycontrollers.PreparedForDelivery),
			LDC:  make(chan traycontrollers.FXRLoad),
			SC:   lc,
		}
	}
	// handle incoming requests on availability
	towercontroller.HandleAvailable(*configFile, sugar, registry)
	// handle incoming posts to load
	towercontroller.HandleLoad(conf, sugar, registry)
	// handle incoming posts to preparedForDelivery
	towercontroller.HandlePreparedForDelivery(conf, sugar, registry)

	go func() {
		if err = http.ListenAndServe(*localAddr, nil); err != nil {
			sugar.Errorw("http.ListenAndServe", "error", err)
		}
	}()

	var publisher *protostream.Socket

	pubU := url.URL{Scheme: "ws", Host: protostream.DefaultListenerAddress, Path: protostream.WSEndpoint}

	// will never return an error because the operation never returns a backoff.PermanentError (tries forever)
	_ = backoff.Retry(
		func() error {
			publisher, err = protostream.NewPublisher(pubU.String(), "")
			if err != nil {
				sugar.Warnw("unable to create new publisher", "error", err)
				return err
			}

			return nil
		},
		backoff.NewConstantBackOff(time.Second*5),
	)

	// handle incoming posts to reset faulted fixtures
	towercontroller.HandleResetFixtureFault(publisher, sugar, registry)
	// handle incoming posts to remove fixture reservation
	towercontroller.HandleUnreserveFixture(sugar, registry)

	u := url.URL{Scheme: "ws", Host: *wsAddr, Path: protostream.WSEndpoint}

	sugar.Info("starting state machine")

	for _, name := range conf.AllFixtures {
		var subscriber *protostream.Socket

		n := name

		if err = backoff.Retry(
			func() error {
				subscriber, err = protostream.NewSubscriberWithSub(u.String(), n)
				if err != nil {
					sugar.Errorw("create new subscriber", "error", err)
					return err
				}

				return nil
			},
			backoff.NewExponentialBackOff(),
		); err != nil {
			sugar.Fatalw("create new subscriber", "error", err)
		}

		lc := subscriber.Listen()

		go func(name string) {
			statemachine.RunFrom(&towercontroller.Idle{
				Config:        conf,
				Logger:        sugar,
				CellAPIClient: caClient,
				Manual:        *manual,
				MockCellAPI:   *mockCellAPI,
				FXRInfo:       registry[name],
				Publisher:     publisher,
				SubscribeChan: lc,
			})
		}(name)
	}

	// TODO: pass a context or done channel to shut down the SM
	select {}
}

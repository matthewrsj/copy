//+build !test

// main runs the tower controller application.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
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
	_logLvlDef    = zapcore.InfoLevel
	_logFileDef   = "logs/towercontroller/statemachine.log"
	_confFileDef  = "/etc/towercontroller.d/statemachine.yaml"
	_localDef     = "0.0.0.0:13163"
	_localUserDef = "0.0.0.0:13173"
)

// nolint:funlen // main func
func main() {
	logLvl := zap.LevelFlag("loglvl", _logLvlDef, "log level for zap logger")
	logFile := flag.String("logf", _logFileDef, "path to the log file")
	configFile := flag.String("conf", _confFileDef, "path to the configuration file")
	localAddr := flag.String("local", _localDef, "local address for operational API")
	localUserAddr := flag.String("local-usr", _localUserDef, "local address for user API")
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

	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	caClient := cellapi.NewClient(conf.CellAPI.Base,
		cellapi.WithNextProcessStepFmtEndpoint(conf.CellAPI.Endpoints.NextProcStepFmt),
		cellapi.WithProcessStatusFmtEndpoint(conf.CellAPI.Endpoints.ProcessStatusFmt),
		cellapi.WithCellMapFmtEndpoint(conf.CellAPI.Endpoints.CellMapFmt),
		cellapi.WithCellStatusEndpoint(conf.CellAPI.Endpoints.CellStatus),
		cellapi.WithCloseProcessFmtEndpoint(conf.CellAPI.Endpoints.CloseProcessFmt),
	)

	registry := make(map[string]*towercontroller.FixtureInfo)

	ctx, cancel := context.WithCancel(context.Background())

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

		registry[name] = &towercontroller.FixtureInfo{
			Name: name,
			PFD:  make(chan traycontrollers.PreparedForDelivery),
			LDC:  make(chan traycontrollers.FXRLoad),
			FixtureState: towercontroller.RunNewFixtureState(
				towercontroller.WithAllDataExpiry(time.Second*5),
				towercontroller.WithContext(ctx),
				towercontroller.WithListener(sub),
				towercontroller.WithLogger(sugar),
			),
		}
	}

	// create publisher socket over which to communicate with FXRs via protostream
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

	// operational API is handled by the opsMux
	opsMux := http.NewServeMux()

	// handle incoming requests on availability
	towercontroller.HandleAvailable(opsMux, *configFile, sugar, registry)
	// handle incoming posts to load
	towercontroller.HandleLoad(opsMux, conf, sugar, registry)
	// handle incoming posts to preparedForDelivery
	towercontroller.HandlePreparedForDelivery(opsMux, sugar, registry)
	// handle incoming posts to broadcast to fixtures
	towercontroller.HandleBroadcastRequest(opsMux, publisher, sugar, registry)

	// user API is handled by the userMux
	userMux := http.NewServeMux()

	// handle incoming posts to send form and equipment requests
	towercontroller.HandleSendFormRequest(userMux, publisher, sugar, registry)
	towercontroller.HandleSendEquipmentRequest(userMux, publisher, sugar, registry)
	// handle incoming posts to remove fixture reservation
	towercontroller.HandleUnreserveFixture(userMux, sugar, registry)

	opsServer := http.Server{
		Addr:    *localAddr,
		Handler: opsMux,
	}

	userServer := http.Server{
		Addr:    *localUserAddr,
		Handler: userMux,
	}

	for _, srv := range []*http.Server{&opsServer, &userServer} {
		go func(srv *http.Server) {
			if err = srv.ListenAndServe(); err != http.ErrServerClosed {
				sugar.Errorw("server ListenAndServe", "error", err)
			}
		}(srv)
	}

	defer func() {
		// nolint:govet // don't need to cancel this, as the timeout is respected by Shtudown
		to, _ := context.WithTimeout(context.Background(), time.Second*5)
		if err = opsServer.Shutdown(to); err != nil {
			sugar.Errorw("unable to gracefully shut down ops server", "error", err)
		}

		if err = userServer.Shutdown(to); err != nil {
			sugar.Errorw("unable to gracefully shut down user server", "error", err)
		}
	}()

	sugar.Info("starting state machine")

	for _, name := range conf.AllFixtures {
		go func(name string) {
			statemachine.RunFrom(&towercontroller.Idle{
				Config:        conf,
				Logger:        sugar,
				CellAPIClient: caClient,
				MockCellAPI:   *mockCellAPI,
				FXRInfo:       registry[name],
				Publisher:     publisher,
			})
		}(name)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs,
		syscall.SIGTERM, // ^C
		syscall.SIGINT,  // kill
		syscall.SIGQUIT, // QUIT
		syscall.SIGABRT, // ABORT
		syscall.SIGHUP,  // parent closing
	)

	// block until a signal is received
	<-sigs // deferred server shutdowns will be called
	cancel()
}

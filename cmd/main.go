//go:build !test
// +build !test

// main runs the tower controller application.
package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"stash.teslamotors.com/ctet/api/canary"
	"stash.teslamotors.com/ctet/api/meter"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/ctet/tabitha"
	"stash.teslamotors.com/rr/cdcontroller"
	"stash.teslamotors.com/rr/protostream"
	"stash.teslamotors.com/rr/towercontroller"
)

const (
	_logLvlDef    = zapcore.InfoLevel
	_logFileDef   = "logs/towercontroller/statemachine.log"
	_confFileDef  = "/etc/towercontroller.d/statemachine.yaml"
	_localDef     = "0.0.0.0:13163"
	_localUserDef = "0.0.0.0:13173"
)

// build variables for govvv
// nolint:golint // don't need its own declaration when it's all just being used by a build tool
// in other words... DON'T CHANGE
var (
	GitCommit  string
	GitBranch  string
	GitState   string
	GitSummary string
	BuildDate  string
	Version    string
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

	caClient := cdcontroller.NewCellAPIClient(conf.CellAPI.Base,
		cdcontroller.WithNextProcessStepFmtEndpoint(conf.CellAPI.Endpoints.NextProcStepFmt),
		cdcontroller.WithProcessStatusFmtEndpoint(conf.CellAPI.Endpoints.ProcessStatusFmt),
		cdcontroller.WithCellMapFmtEndpoint(conf.CellAPI.Endpoints.CellMapFmt),
		cdcontroller.WithCellStatusFmtEndpoint(conf.CellAPI.Endpoints.CellStatusFmt),
		cdcontroller.WithCloseProcessFmtEndpoint(conf.CellAPI.Endpoints.CloseProcessFmt),
	)

	registry := make(map[string]*towercontroller.FixtureInfo)

	ctx, cancel := context.WithCancel(context.Background())

	// tabitha
	if len(conf.AllFixtures) == 0 || len(conf.AllowedFixtures) == 0 {
		log.Fatal("configuration must contain non-empty all_fixtures and allowed_fixtures fields")
	}

	tcName := fmt.Sprintf("%s-TC", strings.Split(conf.AllFixtures[0], "-")[0])
	tcID := fmt.Sprintf("%s-%s%s-%s", conf.Loc.Line, conf.Loc.Process, conf.Loc.Aisle, tcName)

	go tabitha.ServeNewHandler(
		tabitha.WithContext(ctx),
		tabitha.WithTesterName(tcID),
		tabitha.WithTesterVersion(Version),
		tabitha.WithTesterGitSummary(GitSummary),
		tabitha.WithLocalConfiguration(*configFile, tabitha.ConfigTypeYAML, &conf),
	)

	for _, name := range conf.AllFixtures {
		registry[name] = &towercontroller.FixtureInfo{
			Name:      name,
			PFD:       make(chan cdcontroller.PreparedForDelivery),
			LDC:       make(chan cdcontroller.FXRLoad),
			Unreserve: make(chan struct{}),
			FixtureState: towercontroller.RunNewFixtureState(
				towercontroller.WithAllDataExpiry(time.Second*7), // min data rate (5s) + 40% (2s)
				towercontroller.WithContext(ctx),
				towercontroller.WithListener(connectToProtostreamSocket(sugar, *wsAddr, name)),
				towercontroller.WithLogger(sugar),
			),
			Avail: &towercontroller.ReadyStatus{},
		}
	}

	ts := towercontroller.RunNewTCAUXState(
		towercontroller.WithTCAUXDataExpiry(time.Second*7),
		towercontroller.WithTCAUXContext(ctx),
		towercontroller.WithTCAUXListener(connectToProtostreamSocket(sugar, *wsAddr, "TCAUX")),
		towercontroller.WithTCAUXLogger(sugar),
	)

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

	// turn on isolation test on one fixture, make sure it is off on all the rest so the algorithm works
	// due to len check above conf.AllowedFixture[0] is always present.
	go towercontroller.TurnOnIsolationTest(ctx, sugar, registry, publisher, conf.AllowedFixtures, conf.AllFixtures)

	versions := canary.Versions{
		GitCommit:  GitCommit,
		GitBranch:  GitBranch,
		GitState:   GitState,
		GitSummary: GitSummary,
		BuildDate:  BuildDate,
		Version:    Version,
	}

	/*
		The Operational API is the API used by the C/D Controller to control each Tower Controller by
		querying availability, informing the TC a tray has been loaded, reserving fixtures when a
		tray is on the way, broadcasting requests to all fixtures, etc.
	*/

	// operational API is handled by the opsRouter
	opsRouter := mux.NewRouter()
	// handle incoming requests on availability
	opsRouter.HandleFunc(towercontroller.AvailabilityEndpoint, towercontroller.HandleAvailable(*configFile, sugar, registry, ts)).Methods(http.MethodGet)
	// handle incoming posts to load
	opsRouter.HandleFunc(towercontroller.LoadEndpoint, towercontroller.HandleLoad(conf, sugar, registry)).Methods(http.MethodPost)
	// handle incoming posts to preparedForDelivery
	opsRouter.HandleFunc(towercontroller.PreparedForDeliveryEndpoint, towercontroller.HandlePreparedForDelivery(sugar, registry)).Methods(http.MethodPost)
	// handle incoming posts to broadcast to fixtures
	opsRouter.HandleFunc(cdcontroller.BroadcastEndpoint, towercontroller.HandleBroadcastRequest(publisher, sugar, registry)).Methods(http.MethodPost)
	// handle incoming gets to canary
	opsRouter.HandleFunc(
		canary.CanaryEndpoint,
		canary.NewHandlerFunc(
			canary.WithLogger(logger),
			canary.WithCallbackFunc(towercontroller.CanaryCallback(registry)),
			canary.WithVersions(versions),
			canary.WithCORSAllowed(),
		),
	).Methods(http.MethodGet)

	/*
		The User API is the API intended for engineers to send maintenance-type commands to manually exercise
		fixtures or to repair state of the system. The User API allows users to send form and equipment requests
		to fixtures and manually un-reserve a reserved fixture.
	*/

	// user API is handled by the userRouter
	userRouter := mux.NewRouter()

	// handle incoming posts to send form and equipment requests
	userRouter.HandleFunc(towercontroller.SendFormRequestEndpoint, towercontroller.HandleSendFormRequest(publisher, sugar, registry)).Methods(http.MethodPost)
	userRouter.HandleFunc(towercontroller.SendEquipmentRequestEndpoint, towercontroller.HandleSendEquipmentRequest(publisher, sugar, registry)).Methods(http.MethodPost)
	// handle incoming posts to remove fixture reservation
	userRouter.HandleFunc(towercontroller.UnreserveFixtureEndpoint, towercontroller.HandleUnreserveFixture(sugar, registry)).Methods(http.MethodPost)
	// handle incoming GETs to get latest fixture proto messages
	userRouter.HandleFunc(towercontroller.LatestOpEndpoint, towercontroller.HandleLatestOp(sugar, registry)).Methods(http.MethodGet)
	userRouter.HandleFunc(towercontroller.LatestDiagEndpoint, towercontroller.HandleLatestDiag(sugar, registry)).Methods(http.MethodGet)
	userRouter.HandleFunc(towercontroller.LatestAlertEndpoint, towercontroller.HandleLatestAlert(sugar, registry)).Methods(http.MethodGet)
	// handle incoming GETs and POSTs to update the towercontroller
	cc := make(chan struct{}, 1) // single buffer so handle processes doing something else at the time of write
	userRouter.HandleFunc(towercontroller.UpdateEndpoint, towercontroller.HandleUpdate(sugar, cc, registry)).Methods(http.MethodPost, http.MethodGet)
	userRouter.HandleFunc(towercontroller.UpdateCancelEndpoint, towercontroller.HandleUpdateCancel(sugar, cc)).Methods(http.MethodPost)
	meter.EnableMetrics(userRouter)

	opsServer := http.Server{
		Addr:    *localAddr,
		Handler: opsRouter,
	}

	userServer := http.Server{
		Addr:    *localUserAddr,
		Handler: userRouter,
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
	time.Sleep(time.Second) // allow context to shut everything down
}

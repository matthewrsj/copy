package towercontroller

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cdcontroller"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

// Idle waits for a PreparedForLoad (or loaded to short circuit) from C/D controller
type Idle struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cdcontroller.CellAPIClient
	Publisher     *protostream.Socket

	MockCellAPI bool

	next statemachine.State
	err  error

	FXRInfo *FixtureInfo
}

// nolint:funlen // needed to move some actions out of internal functions, as explained in code comments
func (i *Idle) action() {
	i.FXRInfo.Avail.Set(StatusWaitingForReservation)

	done := make(chan struct{})
	active := make(chan inProgressInfo)
	complete := make(chan inProgressInfo)

	go i.monitorForStatus(done, active, complete)

	// block until we receive a load complete or a prepared for delivery signal
waitForUpdate:
	select {
	case pfd := <-i.FXRInfo.PFD: // prepared for delivery, next state WaitForLoad
		tbc, fxbc, err := newIDs(pfd.Tray, pfd.Fixture)
		if err != nil {
			i.err = fmt.Errorf("parse IDs: %v", err)
			i.Logger.Error(err)

			return
		}

		i.next = &WaitForLoad{
			Config:        i.Config,
			Logger:        i.Logger,
			CellAPIClient: i.CellAPIClient,
			Publisher:     i.Publisher,
			tbc:           tbc,
			fxbc:          fxbc,
			mockCellAPI:   i.MockCellAPI,
			fxrInfo:       i.FXRInfo,
		}
	case fxrLoad := <-i.FXRInfo.LDC: // load complete (we missed something), next state ProcessStep
		fxrID := fmt.Sprintf("%s-%s%s-%02d-%02d", i.Config.Loc.Line, i.Config.Loc.Process, i.Config.Loc.Aisle, fxrLoad.Column, fxrLoad.Level)

		tbc, fxbc, err := newIDs(fxrLoad.TrayID, fxrID)
		if err != nil {
			i.err = fmt.Errorf("parse IDs: %v", err)
			i.Logger.Error(err)

			return
		}

		i.next = &StartProcess{
			Config:          i.Config,
			Logger:          i.Logger,
			CellAPIClient:   i.CellAPIClient,
			Publisher:       i.Publisher,
			mockCellAPI:     i.MockCellAPI,
			fxrInfo:         i.FXRInfo,
			fxbc:            fxbc,
			tbc:             tbc,
			processStepName: fxrLoad.RecipeName,
			recipeVersion:   fxrLoad.RecipeVersion,
			steps:           fxrLoad.Steps,
			stepType:        fxrLoad.StepType,
			transactID:      fxrLoad.TransactionID,
		}
	case ip := <-active:
		if ip.transactionID == "" {
			goto waitForUpdate
		}

		childLogger := i.Logger.With(
			"tray", ip.trayBarcode,
			"fixture", ip.fixtureBarcode,
			"process_step", ip.processStep,
			"transaction_id", ip.transactionID,
		)

		tbc, fxbc, err := newIDs(ip.trayBarcode, ip.fixtureBarcode)
		if err != nil {
			i.err = fmt.Errorf("parse IDs: %v", err)
			i.Logger.Error(err)

			return
		}

		i.next = &InProcess{
			Config:          i.Config,
			Logger:          i.Logger,
			CellAPIClient:   i.CellAPIClient,
			Publisher:       i.Publisher,
			tbc:             tbc,
			fxbc:            fxbc,
			processStepName: ip.processStep,
			mockCellAPI:     i.MockCellAPI,
			fxrInfo:         i.FXRInfo,
			childLogger:     childLogger,
		}

	case ip := <-complete:
		if ip.transactionID == "" {
			goto waitForUpdate
		}

		childLogger := i.Logger.With(
			"tray", ip.trayBarcode,
			"fixture", ip.fixtureBarcode,
			"process_step", ip.processStep,
			"transaction_id", ip.transactionID,
		)

		fxbc, err := cdcontroller.NewFixtureBarcode(ip.fixtureBarcode)
		if err != nil {
			i.err = fmt.Errorf("parse IDs: %v", err)
			i.Logger.Error(err)

			return
		}

		i.next = &Unloading{
			Config:        i.Config,
			Logger:        i.Logger,
			CellAPIClient: i.CellAPIClient,
			Publisher:     i.Publisher,
			fxbc:          fxbc,
			mockCellAPI:   i.MockCellAPI,
			fxrInfo:       i.FXRInfo,
			childLogger:   childLogger,
		}
	}
}

// Actions returns the action functions for this state
func (i *Idle) Actions() []func() {
	return []func(){
		i.action,
	}
}

// Next returns the state to run after this one
func (i *Idle) Next() statemachine.State {
	fxrAllowed := fixtureIsAllowed(i.FXRInfo.Name, i.Config.AllowedFixtures)

	if i.err != nil || !fxrAllowed {
		i.Logger.Warnw("going back to idle state", "error", i.err, "fixture_allowed", fxrAllowed)
		i.err = nil
		i.next = i
	}

	i.Logger.Debugw("transitioning to next state", "next", statemachine.NameOf(i.next))

	return i.next
}

type inProgressInfo struct {
	transactionID  string
	processStep    string
	fixtureBarcode string
	trayBarcode    string
}

func fixtureIsAllowed(fixture string, allowedFixtures []string) bool {
	for _, fxr := range allowedFixtures {
		if fxr == fixture {
			return true
		}
	}

	return false
}

// nolint:gocognit // fire suppression adds logic to only sound the alarm once
func (i *Idle) monitorForStatus(done <-chan struct{}, active chan<- inProgressInfo, complete chan<- inProgressInfo) {
	defer close(active)
	defer close(complete)

	splits := strings.Split(i.FXRInfo.Name, "-")
	if len(splits) != 2 { // expect COL-LVL
		i.Logger.Errorw("invalid fixture name", "name", i.FXRInfo.Name)
		return
	}

	col, err := strconv.Atoi(splits[0])
	if err != nil {
		i.Logger.Errorw("unable to parse fixture name", "name", i.FXRInfo.Name)
		return
	}

	lvl, err := strconv.Atoi(splits[1])
	if err != nil {
		i.Logger.Errorw("unable to parse fixture name", "name", i.FXRInfo.Name)
		return
	}

	for {
		// copy over the configuration, in case it has changed
		// this is a very cheap operation so better to just do it
		// every iteration (about once/second/FXR)
		if _globalConfiguration != nil {
			i.Config = *_globalConfiguration
		}

		fxrID := fmt.Sprintf("%s-%s%s-%02d-%02d", i.Config.Loc.Line, i.Config.Loc.Process, i.Config.Loc.Aisle, col, lvl)

		// quick check so we don't loop forever when fixture not allowed
		select {
		case <-done:
			return
		default:
		}

		if !fixtureIsAllowed(i.FXRInfo.Name, i.Config.AllowedFixtures) {
			// fixture not allowed, check again in a second
			time.Sleep(time.Second)
			continue
		}

		// quick check so we don't loop forever when fixture not allowed
		select {
		case <-done:
			return
		default:
		}

		msg, err := i.FXRInfo.FixtureState.GetOp()
		if err != nil {
			// this is just debug, since it happens right at startup
			i.Logger.Debugw("monitor for status; get operational message", "error", err)
			time.Sleep(time.Second) // wait a second for it to update

			continue
		}

		ipInfo := inProgressInfo{
			transactionID:  msg.GetInfo().GetTransactionId(),
			processStep:    msg.GetInfo().GetRecipeName(),
			fixtureBarcode: fxrID,
			trayBarcode:    msg.GetInfo().GetTrayBarcode(),
		}

		i.Logger.Debugw("fixture status", "fixture", i.FXRInfo.Name, "status", msg.GetOp().GetStatus().String())

		switch msg.GetOp().GetStatus() {
		case tower.FixtureStatus_FIXTURE_STATUS_ACTIVE:
			// go to in-progress
			active <- ipInfo

			return
		case tower.FixtureStatus_FIXTURE_STATUS_COMPLETE:
			// go to unload
			complete <- ipInfo

			return
		default:
			// wait a second for it to update before checking again
			time.Sleep(time.Second)
		}
	}
}

func newIDs(tray, fixture string) (tbc cdcontroller.TrayBarcode, fxbc cdcontroller.FixtureBarcode, err error) {
	tbc, err = cdcontroller.NewTrayBarcode(tray)
	if err != nil {
		err = fmt.Errorf("parse tray ID: %v", err)
		return
	}

	fxbc, err = cdcontroller.NewFixtureBarcode(fixture)
	if err != nil {
		err = fmt.Errorf("parse fixture ID: %v", err)
		return
	}

	return
}

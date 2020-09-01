package towercontroller

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

// Idle waits for a PreparedForLoad (or loaded to short circuit) from C/D controller
type Idle struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client
	Publisher     *protostream.Socket
	SubscribeChan <-chan *protostream.Message

	Manual      bool
	MockCellAPI bool

	alarmed pb.FireAlarmStatus
	next    statemachine.State
	err     error

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
			SubscribeChan: i.SubscribeChan,
			tbc:           tbc,
			fxbc:          fxbc,
			manual:        i.Manual,
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

		// TODO: short circuit to in-progress if fixture status is active (or complete?)
		i.next = &ProcessStep{
			Config:        i.Config,
			Logger:        i.Logger,
			CellAPIClient: i.CellAPIClient,
			Publisher:     i.Publisher,
			SubscribeChan: i.SubscribeChan,
			mockCellAPI:   i.MockCellAPI,
			fxrInfo:       i.FXRInfo,
		}

		i.next.SetContext(Barcodes{
			Fixture:         fxbc,
			Tray:            tbc,
			ProcessStepName: fxrLoad.RecipeName,
			MockCellAPI:     i.MockCellAPI,
			RecipeName:      fxrLoad.RecipeName,
			RecipeVersion:   fxrLoad.RecipeVersion,
			StepConf:        fxrLoad.Steps,
			TransactID:      fxrLoad.TransactionID,
		})
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
			SubscribeChan:   i.SubscribeChan,
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

		fxbc, err := traycontrollers.NewFixtureBarcode(ip.fixtureBarcode)
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
			SubscribeChan: i.SubscribeChan,
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

	for {
		// copy over the configuration, in case it has changed
		// this is a very cheap operation so better to just do it
		// every iteration (about once/second/FXR)
		if _globalConfiguration != nil {
			i.Config = *_globalConfiguration
		}

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

		select {
		case <-done:
			return
		case lMsg := <-i.SubscribeChan:
			var event protostream.ProtoMessage
			if err := json.Unmarshal(lMsg.Msg.Body, &event); err != nil {
				i.Logger.Debugw("unmarshal JSON frame", "error", err, "bytes", string(lMsg.Msg.Body))
				continue
			}

			var msg pb.FixtureToTower
			if err := proto.Unmarshal(event.Body, &msg); err != nil {
				i.Logger.Debugw("unable to unmarshal message (may be wrong type)", "error", err)
				continue
			}

			splits := strings.Split(i.FXRInfo.Name, "-")
			if len(splits) != 2 { // expect COL-LVL
				i.Logger.Warnw("invalid fixture name", "name", i.FXRInfo.Name)
				continue
			}

			col, err := strconv.Atoi(splits[0])
			if err != nil {
				i.Logger.Debugw("unable to parse fixture name", "name", i.FXRInfo.Name)
				continue
			}

			lvl, err := strconv.Atoi(splits[1])
			if err != nil {
				i.Logger.Debugw("unable to parse fixture name", "name", i.FXRInfo.Name)
				continue
			}

			fxrID := fmt.Sprintf("%s-%s%s-%02d-%02d", i.Config.Loc.Line, i.Config.Loc.Process, i.Config.Loc.Aisle, col, lvl)

			ipInfo := inProgressInfo{
				transactionID:  msg.GetTransactionId(),
				processStep:    msg.GetProcessStep(),
				fixtureBarcode: fxrID,
				trayBarcode:    msg.GetTraybarcode(),
			}

			i.Logger.Debugw("fixture status", "fixture", i.FXRInfo.Name, "status", msg.GetOp().GetStatus().String())

			switch msg.GetOp().GetStatus() {
			case pb.FixtureStatus_FIXTURE_STATUS_ACTIVE:
				// go to in-progress
				active <- ipInfo

				return
			case pb.FixtureStatus_FIXTURE_STATUS_COMPLETE:
				// go to unload
				complete <- ipInfo

				return
			case pb.FixtureStatus_FIXTURE_STATUS_FAULTED:
				if msg.GetOp().GetFireAlarmStatus() != pb.FireAlarmStatus_FIRE_ALARM_UNKNOWN_UNSPECIFIED {
					i.Logger.Warnw("fire alarm detected from fixture", "fixture", i.FXRInfo.Name, "alarm", msg.GetOp().GetFireAlarmStatus().String())

					// fire alarm, tell CDC
					// this is in-band because it will try _forever_ until it succeeds,
					// but we don't want to go to unload step because it will queue another job for the crane
					// to unload this tray, but we want the next operation on this tray to be a fire
					// suppression activity.
					if i.alarmed < msg.GetOp().GetFireAlarmStatus() { // don't alarm again if we already alarmed in the InProcess state
						i.Logger.Infow("sounding the fire alarm", "fixture", i.FXRInfo.Name, "alarm", msg.GetOp().GetFireAlarmStatus().String())

						if err := soundTheAlarm(i.Config, msg.GetOp().GetFireAlarmStatus(), i.FXRInfo.Name, i.Logger); err != nil {
							i.Logger.Errorw("sound the fire alarm", "error", err)
							continue // try to send the alarm next time
						}

						i.alarmed = msg.GetOp().GetFireAlarmStatus()
					}
				}
			}
		}
	}
}

func newIDs(tray, fixture string) (tbc traycontrollers.TrayBarcode, fxbc traycontrollers.FixtureBarcode, err error) {
	tbc, err = traycontrollers.NewTrayBarcode(tray)
	if err != nil {
		err = fmt.Errorf("parse tray ID: %v", err)
		return
	}

	fxbc, err = traycontrollers.NewFixtureBarcode(fixture)
	if err != nil {
		err = fmt.Errorf("parse fixture ID: %v", err)
		return
	}

	return
}

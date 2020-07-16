package towercontroller

import (
	"encoding/json"
	"fmt"

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
	SubscribeChan <-chan *protostream.Message

	Manual      bool
	MockCellAPI bool

	next statemachine.State
	err  error

	FXRInfo *FixtureInfo
}

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
		if ip.transactionID <= 0 {
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
			SubscribeChan:   i.SubscribeChan,
			tbc:             tbc,
			fxbc:            fxbc,
			processStepName: ip.processStep,
			mockCellAPI:     i.MockCellAPI,
			fxrInfo:         i.FXRInfo,
			childLogger:     childLogger,
		}

	case ip := <-complete:
		if ip.transactionID <= 0 {
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
	if i.err != nil {
		i.Logger.Warnw("going back to idle state", "error", i.err)
		i.err = nil
		i.next = i
	}

	i.Logger.Debugw("transitioning to next state", "next", statemachine.NameOf(i.next))

	return i.next
}

type inProgressInfo struct {
	transactionID  int64
	processStep    string
	fixtureBarcode string
	trayBarcode    string
}

func (i *Idle) monitorForStatus(done <-chan struct{}, active chan<- inProgressInfo, complete chan<- inProgressInfo) {
	defer close(active)
	defer close(complete)

	for {
		select {
		case <-done:
			return
		case lMsg := <-i.SubscribeChan:
			i.Logger.Debugw("got message", "message", lMsg.Msg)

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

			ipInfo := inProgressInfo{
				transactionID:  msg.GetTransactionId(),
				processStep:    msg.GetProcessStep(),
				fixtureBarcode: msg.GetFixturebarcode(),
				trayBarcode:    msg.GetTraybarcode(),
			}

			switch msg.GetOp().GetStatus() {
			case pb.FixtureStatus_FIXTURE_STATUS_ACTIVE:
				// go to in-progress
				i.Logger.Debug("fixture ACTIVE")
				active <- ipInfo

				i.Logger.Debug("returning")

				return
			case pb.FixtureStatus_FIXTURE_STATUS_COMPLETE:
				// go to unload
				i.Logger.Debug("fixture COMPLETE")
				complete <- ipInfo

				i.Logger.Debug("returning")

				return
			default:
				i.Logger.Debugw("fixture", "status", msg.GetOp().GetStatus().String())
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

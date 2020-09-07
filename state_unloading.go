package towercontroller

import (
	"time"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

// Unloading waits for the fixture to go back to IDLE before returning to the idle state
type Unloading struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client
	Publisher     *protostream.Socket

	childLogger *zap.SugaredLogger
	manual      bool
	mockCellAPI bool

	fxbc traycontrollers.FixtureBarcode

	fxrInfo *FixtureInfo
}

func (u *Unloading) action() {
	u.fxrInfo.Avail.Set(StatusUnloading)

	for {
		msg, err := u.fxrInfo.FixtureState.GetOp()
		if err != nil {
			u.childLogger.Warnw("monitoring for unload; get fixture operational message", "error", err)
			time.Sleep(time.Second) // give it time to update

			continue
		}

		status := msg.GetOp().GetStatus()

		u.childLogger.Debugw("received status", "status", status.String())

		// fixture will stay in fault, don't wait for it to go to idle before we go back to idle
		if status == pb.FixtureStatus_FIXTURE_STATUS_IDLE || status == pb.FixtureStatus_FIXTURE_STATUS_FAULTED {
			u.childLogger.Info("tray unloaded")
			break
		}

		// fixture updates anywhere from 1-3 seconds, so delay before checking again
		time.Sleep(time.Second)
	}
}

// Actions returns the action functions for this state
func (u *Unloading) Actions() []func() {
	return []func(){
		u.action,
	}
}

// Next returns the state to run after this one
func (u *Unloading) Next() statemachine.State {
	next := &Idle{
		Config:        u.Config,
		Logger:        u.Logger,
		CellAPIClient: u.CellAPIClient,
		Publisher:     u.Publisher,
		Manual:        u.manual,
		MockCellAPI:   u.mockCellAPI,
		FXRInfo:       u.fxrInfo,
	}
	u.childLogger.Debugw("transitioning back to Idle", "next", statemachine.NameOf(next))

	return next
}

package towercontroller

import (
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
	SubscribeChan <-chan *protostream.Message

	childLogger *zap.SugaredLogger
	manual      bool
	mockCellAPI bool

	fxbc traycontrollers.FixtureBarcode

	fxrInfo *FixtureInfo
}

func (u *Unloading) action() {
	u.fxrInfo.Avail.Set(StatusUnloading)

	for lMsg := range u.SubscribeChan {
		u.childLogger.Debugw("unloading: got message", "message", lMsg.Msg)

		msg, err := unmarshalProtoMessage(lMsg)
		if err != nil {
			u.childLogger.Errorw("unload proto message: %v", err)
			continue
		}

		if msg.GetOp() == nil {
			u.childLogger.Debugw("got non-operational message, checking next one", "msg", msg.String())
			continue
		}

		status := msg.GetOp().GetStatus()

		u.childLogger.Debugw("received status", "status", status.String())

		// fixture will stay in fault, don't wait for it to go to idle before we go back to idle
		if status == pb.FixtureStatus_FIXTURE_STATUS_IDLE || status == pb.FixtureStatus_FIXTURE_STATUS_FAULTED {
			u.childLogger.Info("tray unloaded")
			break
		}
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
		SubscribeChan: u.SubscribeChan,
		Manual:        u.manual,
		MockCellAPI:   u.mockCellAPI,
		FXRInfo:       u.fxrInfo,
	}
	u.childLogger.Debugw("transitioning back to Idle", "next", statemachine.NameOf(next))

	return next
}

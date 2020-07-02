package towercontroller

import (
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/go-socketcan/pkg/socketcan"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

// Unloading waits for the fixture to go back to IDLE before returning to the idle state
type Unloading struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client

	manual      bool
	mockCellAPI bool

	fxbc traycontrollers.FixtureBarcode

	fxrInfo *FixtureInfo
}

func (u *Unloading) action() {
	u.fxrInfo.Avail.Set(StatusUnloading)

	fxrID, ok := u.Config.Fixtures[IDFromFXR(u.fxbc)]
	if !ok {
		fatalError(u, u.Logger, fmt.Errorf("fixture %s not configured for tower controller", IDFromFXR(u.fxbc)))
		return
	}

	u.Logger.Info("creating ISOTP interface to monitor fixture")

	dev, err := socketcan.NewIsotpInterface(u.Config.CAN.Device, fxrID, u.Config.CAN.TXID)
	if err != nil {
		fatalError(u, u.Logger, fmt.Errorf("NewIsotpInterface: %v", err))
		return
	}

	defer func() {
		_ = dev.Close()
	}()

	if err := dev.SetCANFD(); err != nil {
		fatalError(u, u.Logger, fmt.Errorf("SetCANFD: %v", err))
		return
	}

	for {
		data, err := dev.RecvBuf()
		if err != nil {
			fatalError(u, u.Logger, fmt.Errorf("RecvBuf: %v", err))
		}

		msg := &pb.FixtureToTower{}

		if err = proto.Unmarshal(data, msg); err != nil {
			u.Logger.Debug("expecting FixtureToTower message", "error", err)
			continue
		}

		if msg.GetOp().GetStatus() != pb.FixtureStatus_FIXTURE_STATUS_COMPLETE {
			u.Logger.Info("tray unloaded")
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
		Manual:        u.manual,
		MockCellAPI:   u.mockCellAPI,
		FXRInfo:       u.fxrInfo,
	}
	u.Logger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

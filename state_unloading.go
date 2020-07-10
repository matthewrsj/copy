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

	childLogger *zap.SugaredLogger
	manual      bool
	mockCellAPI bool

	fxbc traycontrollers.FixtureBarcode

	fxrInfo *FixtureInfo
}

func (u *Unloading) action() {
	u.fxrInfo.Avail.Set(StatusUnloading)

	fConf, ok := u.Config.Fixtures[IDFromFXR(u.fxbc)]
	if !ok {
		fatalError(u, u.childLogger, fmt.Errorf("fixture %s not configured for tower controller", IDFromFXR(u.fxbc)))
		return
	}

	u.childLogger.Info("creating ISOTP interface to monitor fixture for unload")

	dev, err := socketcan.NewIsotpInterface(fConf.Bus, fConf.RX, fConf.TX)
	if err != nil {
		fatalError(u, u.childLogger, fmt.Errorf("NewIsotpInterface: %v", err))
		return
	}

	defer func() {
		_ = dev.Close()
	}()

	if err := dev.SetCANFD(); err != nil {
		fatalError(u, u.childLogger, fmt.Errorf("SetCANFD: %v", err))
		return
	}

	for {
		data, err := dev.RecvBuf()
		if err != nil {
			fatalError(u, u.childLogger, fmt.Errorf("RecvBuf: %v", err))
		}

		msg := &pb.FixtureToTower{}

		if err = proto.Unmarshal(data, msg); err != nil {
			u.childLogger.Debug("expecting FixtureToTower message", "error", err)
			continue
		}

		if msg.GetOp().GetStatus() != pb.FixtureStatus_FIXTURE_STATUS_COMPLETE {
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
		Manual:        u.manual,
		MockCellAPI:   u.mockCellAPI,
		FXRInfo:       u.fxrInfo,
	}
	u.childLogger.Debugw("transitioning back to Idle", "next", statemachine.NameOf(next))

	return next
}

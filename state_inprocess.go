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

// InProcess monitors the FXR proto for the state to change
type InProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client

	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	processStepName string
	fixtureFault    bool
	manual          bool
	mockCellAPI     bool
	cells           map[string]cellapi.CellData
	cellResponse    []*pb.Cell
	canErr          error
	recipeVersion   int
}

func (i *InProcess) action() {
	fxrID, ok := i.Config.Fixtures[IDFromFXR(i.fxbc)]
	if !ok {
		fatalError(i, i.Logger, fmt.Errorf("fixture %s not configured for tower controller", IDFromFXR(i.fxbc)))
		return
	}

	i.Logger.Info("creating ISOTP interface to monitor fixture")

	var dev socketcan.Interface

	if dev, i.canErr = socketcan.NewIsotpInterface(i.Config.CAN.Device, fxrID, i.Config.CAN.TXID); i.canErr != nil {
		fatalError(i, i.Logger, i.canErr)
		return
	}

	defer func() {
		_ = dev.Close()
	}()

	if err := dev.SetCANFD(); err != nil {
		fatalError(i, i.Logger, err)
		return
	}

	i.Logger.Info("monitoring fixture to go to complete or fault")

	for {
		var data []byte

		data, i.canErr = dev.RecvBuf()
		if i.canErr != nil {
			fatalError(i, i.Logger, i.canErr)
			return
		}

		msg := &pb.FixtureToTower{}

		if i.canErr = proto.Unmarshal(data, msg); i.canErr != nil {
			fatalError(i, i.Logger, i.canErr)
			return
		}

		i.Logger.Debug("got FixtureToTower message")

		fxbcBroadcast, err := traycontrollers.NewFixtureBarcode(msg.GetFixturebarcode())
		if err != nil {
			i.Logger.Warn(fmt.Errorf("parse fixture position: %v", err))
			continue
		}

		if fxbcBroadcast.Fxn != i.fxbc.Fxn {
			i.Logger.Warnf("got fixture status for different fixture %s", fxbcBroadcast.Fxn)
			continue
		}

		op, ok := msg.GetContent().(*pb.FixtureToTower_Op)
		if !ok {
			i.Logger.Debugf("got different message than we are looking for (%T)", msg.GetContent())
			continue
		}

		switch s := op.Op.GetStatus(); s {
		case pb.FixtureStatus_FIXTURE_STATUS_COMPLETE, pb.FixtureStatus_FIXTURE_STATUS_FAULTED:
			msg := "fixture done with tray"

			if i.fixtureFault = s == pb.FixtureStatus_FIXTURE_STATUS_FAULTED; i.fixtureFault {
				msg += "; fixture faulted"
			}

			i.Logger.Info(msg)

			i.cellResponse = op.Op.GetCells()

			return
		default:
			i.Logger.Debugw("received fixture_status update", "status", s.String())
		}
	}
}

// Actions returns the action functions for this state
func (i *InProcess) Actions() []func() {
	return []func(){
		i.action,
	}
}

// Next returns the next state to run
func (i *InProcess) Next() statemachine.State {
	next := &EndProcess{
		Config:          i.Config,
		Logger:          i.Logger,
		CellAPIClient:   i.CellAPIClient,
		tbc:             i.tbc,
		fxbc:            i.fxbc,
		processStepName: i.processStepName,
		fixtureFault:    i.fixtureFault,
		cellResponse:    i.cellResponse,
		cells:           i.cells,
		manual:          i.manual,
		mockCellAPI:     i.mockCellAPI,
		recipeVersion:   i.recipeVersion,
	}
	i.Logger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

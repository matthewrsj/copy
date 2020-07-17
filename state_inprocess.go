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

// InProcess monitors the FXR proto for the state to change
type InProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client
	Publisher     *protostream.Socket
	SubscribeChan <-chan *protostream.Message

	childLogger     *zap.SugaredLogger
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	processStepName string
	fixtureFault    bool
	manual          bool
	mockCellAPI     bool
	cells           map[string]cellapi.CellData
	cellResponse    []*pb.Cell
	recipeVersion   int

	fxrInfo *FixtureInfo
}

func (i *InProcess) action() {
	i.childLogger.Info("monitoring fixture to go to complete or fault")

	for lMsg := range i.SubscribeChan {
		i.childLogger.Debugw("got message", "message", lMsg.Msg)

		var event protostream.ProtoMessage
		if err := json.Unmarshal(lMsg.Msg.Body, &event); err != nil {
			i.Logger.Debugw("unmarshal JSON frame", "error", err, "bytes", string(lMsg.Msg.Body))
			continue
		}

		msg := &pb.FixtureToTower{}

		if err := proto.Unmarshal(event.Body, msg); err != nil {
			i.Logger.Debugw("unmarshal proto", "error", err)
			return
		}

		i.childLogger.Infow("got FixtureToTower message", "msg", msg.String())

		fxbcBroadcast, err := traycontrollers.NewFixtureBarcode(msg.GetFixturebarcode())
		if err != nil {
			i.childLogger.Warn(fmt.Errorf("parse fixture position: %v", err))
			continue
		}

		if fxbcBroadcast.Fxn != i.fxbc.Fxn {
			i.childLogger.Warnf("got fixture status for different fixture %s", fxbcBroadcast.Fxn)
			continue
		}

		op, ok := msg.GetContent().(*pb.FixtureToTower_Op)
		if !ok {
			i.childLogger.Debugf("got different message than we are looking for (%T)", msg.GetContent())
			continue
		}

		switch s := op.Op.GetStatus(); s {
		case pb.FixtureStatus_FIXTURE_STATUS_COMPLETE, pb.FixtureStatus_FIXTURE_STATUS_FAULTED:
			msg := "fixture done with tray"

			if i.fixtureFault = s == pb.FixtureStatus_FIXTURE_STATUS_FAULTED; i.fixtureFault {
				msg += "; fixture faulted"
			}

			i.childLogger.Info(msg)

			i.cellResponse = op.Op.GetCells()

			return
		default:
			i.childLogger.Debugw("received fixture_status update", "status", s.String())
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
		Publisher:       i.Publisher,
		SubscribeChan:   i.SubscribeChan,
		childLogger:     i.childLogger,
		tbc:             i.tbc,
		fxbc:            i.fxbc,
		processStepName: i.processStepName,
		fixtureFault:    i.fixtureFault,
		cellResponse:    i.cellResponse,
		cells:           i.cells,
		manual:          i.manual,
		mockCellAPI:     i.mockCellAPI,
		recipeVersion:   i.recipeVersion,
		fxrInfo:         i.fxrInfo,
	}
	i.childLogger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

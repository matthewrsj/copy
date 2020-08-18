package towercontroller

import (
	"encoding/json"

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
	returnToIdle    bool
	alarmed         pb.FireAlarmStatus
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

		i.childLogger.Debugw("got FixtureToTower message", "msg", msg.String())

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

				if op.Op.GetFireAlarmStatus() != pb.FireAlarmStatus_FIRE_ALARM_UNKNOWN_UNSPECIFIED {
					// fire alarm, tell CDC
					// this is in-band because it will try _forever_ until it succeeds,
					// but we don't want to go to unload step because it will queue another job for the crane
					// to unload this tray, but we want the next operation on this tray to be a fire
					// suppression activity.
					i.returnToIdle = true // return to idle whether or not we successfully sounded alarm
					if err := soundTheAlarm(i.Config, op.Op.GetFireAlarmStatus(), i.fxrInfo.Name, i.childLogger); err != nil {
						// basically couldn't marshal the request. Return to idle where we will keep trying for as
						// long as the alarm exists
						return
					}

					// successfully alarmed, return to idle but set alarmed to true so we don't keep alarming
					i.alarmed = op.Op.GetFireAlarmStatus()
				}
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
	var next statemachine.State
	if i.returnToIdle {
		next = &Idle{
			Config:        i.Config,
			Logger:        i.Logger,
			CellAPIClient: i.CellAPIClient,
			Publisher:     i.Publisher,
			SubscribeChan: i.SubscribeChan,
			Manual:        i.manual,
			MockCellAPI:   i.mockCellAPI,
			FXRInfo:       i.fxrInfo,
			alarmed:       i.alarmed,
		}
	} else {
		next = &EndProcess{
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
	}

	i.childLogger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

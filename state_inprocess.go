package towercontroller

import (
	"fmt"
	"log"

	"github.com/linklayer/go-socketcan/pkg/socketcan"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	pb "stash.teslamotors.com/rr/towercontroller/pb"
)

// InProcess monitors the FXR proto for the state to change
type InProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *logrus.Logger
	CellAPIClient *cellapi.Client

	tbc             TrayBarcode
	fxbc            FixtureBarcode
	processStepName string
	fixtureFault    bool
	cells           map[string]cellapi.CellData
	cellResponse    []*pb.Cell
	canErr          error
}

func (i *InProcess) action() {
	var dev socketcan.Interface

	if dev, i.canErr = socketcan.NewIsotpInterface(
		i.Config.CAN.Device,
		i.Config.CAN.RXID,
		i.Config.CAN.TXID,
	); i.canErr != nil {
		i.Logger.Error(i.canErr)
		log.Println(i.canErr)
		i.SetLast(true)

		return
	}

	defer func() {
		_ = dev.Close()
	}()

	for {
		var data []byte

		data, i.canErr = dev.RecvBuf()
		if i.canErr != nil {
			i.Logger.Error(i.canErr)
			log.Println(i.canErr)
			i.SetLast(true)

			return
		}

		msg := &pb.FixtureToTower{}

		if i.canErr = proto.Unmarshal(data, msg); i.canErr != nil {
			i.Logger.Error(i.canErr)
			log.Println(i.canErr)
			i.SetLast(true)

			return
		}

		fxbcBroadcast, err := NewFixtureBarcode(msg.GetFixtureposition())
		if err != nil {
			err = fmt.Errorf("parse fixture position: %v", err)
			i.Logger.Warn(err)
			log.Println("WARNING:", err)

			continue
		}

		if fxbcBroadcast.Fxn != i.fxbc.Fxn {
			i.Logger.WithFields(logrus.Fields{
				"tray":        i.tbc.SN,
				"fixture_num": i.fxbc.raw,
			}).Tracef("got fixture status for different fixture %s", fxbcBroadcast.Fxn)

			continue
		}

		op, ok := msg.GetContent().(*pb.FixtureToTower_Op)
		if !ok {
			i.Logger.WithFields(logrus.Fields{
				"tray":        i.tbc.SN,
				"fixture_num": i.fxbc.raw,
			}).Tracef("got different message than we are looking for (%T)", msg.GetContent())

			continue
		}

		switch s := op.Op.GetStatus(); s {
		case pb.FixtureStatus_FIXTURE_STATUS_COMPLETE, pb.FixtureStatus_FIXTURE_STATUS_FAULTED:
			msg := "fixture done with tray"

			if i.fixtureFault = s == pb.FixtureStatus_FIXTURE_STATUS_FAULTED; i.fixtureFault {
				msg += "; fixture faulted"
			}

			i.Logger.WithFields(logrus.Fields{
				"tray":        i.tbc.SN,
				"fixture_num": i.fxbc.raw,
			}).Info(msg)

			i.cellResponse = op.Op.GetCells()

			return
		default:
			i.Logger.WithFields(logrus.Fields{
				"tray":           i.tbc.SN,
				"fixture_num":    i.fxbc.raw,
				"fixture_status": s.String(),
			}).Trace("received fixture_status update")
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
	}
	i.Logger.WithField("tray", i.tbc.SN).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

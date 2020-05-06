package towercontroller

import (
	"log"

	"github.com/linklayer/go-socketcan/pkg/socketcan"
	"github.com/sirupsen/logrus"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/statemachine/v2"
	pb "stash.teslamotors.com/rr/towercontroller/pb"
)

type InProcess struct {
	statemachine.Common

	Config Configuration
	Logger *logrus.Logger

	tbc    TrayBarcode
	fxbc   FixtureBarcode
	canErr error
}

// action function does a lot of logging
// nolint:funlen
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

	defer dev.Close()

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

		if msg.GetIdentifier() != i.fxbc.Fxn {
			i.Logger.WithFields(logrus.Fields{
				"tray":        i.tbc.SN,
				"fixture_num": i.fxbc.raw,
			}).Tracef("got fixture status for different fixture %s", msg.GetIdentifier())

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
		case pb.FixtureStatus_FIXTURE_STATUS_COMPLETE:
			i.Logger.WithFields(logrus.Fields{
				"tray":        i.tbc.SN,
				"fixture_num": i.fxbc.raw,
			}).Info("fixture done with tray")

			return
		case pb.FixtureStatus_FIXTURE_STATUS_FAULTED:
			i.Logger.WithFields(logrus.Fields{
				"tray":        i.tbc.SN,
				"fixture_num": i.fxbc.raw,
			}).Error("fixture faulted")
			i.SetLast(true)

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

func (i *InProcess) Actions() []func() {
	return []func(){
		i.action,
	}
}

func (i *InProcess) Next() statemachine.State {
	next := &EndProcess{
		Config: i.Config,
		Logger: i.Logger,
		tbc:    i.tbc,
		fxbc:   i.fxbc,
	}
	i.Logger.WithField("tray", i.tbc.SN).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

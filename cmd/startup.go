package main

import (
	"fmt"
	"time"

	"github.com/linklayer/go-socketcan/pkg/socketcan"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/towercontroller"
	pb "stash.teslamotors.com/rr/towerproto"
)

func monitorForInProgress(c towercontroller.Configuration, fxID uint32) ([]statemachine.Job, error) {
	const waitForMessagesSecs = 5

	dev, err := socketcan.NewIsotpInterface(c.CAN.Device, fxID, c.CAN.TXID)
	if err != nil {
		return nil, fmt.Errorf("create CAN ISOTP interface: %v", err)
	}

	if err = dev.SetRecvTimeout(time.Second); err != nil {
		return nil, fmt.Errorf("set receive timeout: %v", err)
	}

	var jobs []statemachine.Job

	now := time.Now()
	for time.Since(now) < time.Second*waitForMessagesSecs {
		buf, err := dev.RecvBuf()
		if err != nil {
			// timeout returns an error, try again
			continue
		}

		var msg pb.FixtureToTower

		if err := proto.Unmarshal(buf, &msg); err != nil {
			return nil, fmt.Errorf("unmarshal buffer: %v", err)
		}

		op := msg.GetOp()
		if op == nil {
			continue
		}

		if op.GetStatus() == pb.FixtureStatus_FIXTURE_STATUS_ACTIVE {
			fxPos := msg.GetFixtureposition()

			fxBC, err := towercontroller.NewFixtureBarcode(fxPos)
			if err != nil {
				return nil, fmt.Errorf("parse fixture position: %v", err)
			}

			trayBC, err := towercontroller.NewTrayBarcode(msg.GetTraybarcode())
			if err != nil {
				return nil, fmt.Errorf("parse tray barcode: %v", err)
			}

			jobs = append(
				jobs,
				statemachine.Job{
					Name: fxPos,
					Work: towercontroller.Barcodes{
						Fixture:         fxBC,
						Tray:            trayBC,
						ProcessStepName: msg.GetProcessStep(),
						InProgress:      true,
					},
				},
			)
		}
	}

	return jobs, nil
}

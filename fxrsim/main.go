package main

import (
	"log"
	"sync"
	"time"

	"github.com/linklayer/go-socketcan/pkg/socketcan"
	"google.golang.org/protobuf/proto"
	pb "stash.teslamotors.com/rr/towerproto"
)

func main() {
	// rx
	readDev, err := socketcan.NewIsotpInterface("vcan0", 0x200, 0x100)
	if err != nil {
		log.Fatal("create ISOTP listener", err)
	}

	defer readDev.Close()

	// tx
	dev, err := socketcan.NewIsotpInterface("vcan0", 0x100, 0x200)
	if err != nil {
		log.Fatal("create ISOTP interface:", err)
	}

	defer dev.Close()

	// msgDiag contains the diagnostic messages from the FXR. Ignored by the TC SM
	msgDiag := &pb.FixtureToTower{
		Content: &pb.FixtureToTower_Diag{
			Diag: &pb.FixtureDiagnostic{
				Fxr: &pb.Fxr{
					Sensors: &pb.FxrSensors{
						BusVoltage:         15,
						CcEnableInput:      false,
						VBus_24:            23.87,
						PositionSwitchUp:   false,
						PositionSwitchDown: false,
					},
					Outputs: &pb.FxrOutputs{
						StibEnableLine:     true,
						FixtureCloseEnable: true,
						CcEnableOutput:     false,
					},
				},
			},
		},
	}

	var mx sync.Mutex
	go func() {
		// tx to start so no listeners are blocked
		for {
			// send a couple to unblock any listeners
			pkt, err := proto.Marshal(msgDiag)
			if err != nil {
				log.Fatal(err)
			}

			mx.Lock()
			if err = dev.SendBuf(pkt); err != nil {
				log.Fatal(err)
			}
			mx.Unlock()
			time.Sleep(time.Second)
		}
	}()

	// msgOp contains the status of the fixture. This what the TC state machine relies on
	msgOp := &pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Status:   pb.FixtureStatus_FIXTURE_STATUS_IDLE,
				Position: pb.FixturePosition_FIXTURE_POSITION_OPEN,
			},
		},
	}

	for {
		buf, err := readDev.RecvBuf()
		if err != nil {
			log.Fatal("RECV BUF", err)
		}

		var msg pb.TowerToFixture
		err = proto.Unmarshal(buf, &msg)
		if err != nil {
			// just means this isn't the message we are looking for
			continue
		}

		if msg.GetSysinfo().GetProcessStep() == "" {
			// not what we are looking for
			continue
		}

		log.Println("RECEIVED MESSAGE FROM TOWER")
		cells := make([]*pb.Cell, 64)
		cms := msg.Recipe.GetCellMasks()
		for i, cm := range cms {
			for bit := 0; bit < 32; bit++ {
				if cm&(1<<bit) != 0 {
					cells[i+bit] = &pb.Cell{
						Cellstatus: pb.CellStatus_CELL_STATUS_COMPLETE,
						Cellmeasurement: &pb.CellMeasurement{
							Current:             1.23,
							Voltage:             3.47,
							ChargeAh:            94,
							EnergyWh:            74,
							TemperatureEstimate: 28.9,
							PogoResistance:      199,
						},
					}
				}
			}
		}

		msgDiag.Fixtureposition = msg.GetSysinfo().GetFixtureposition()
		msgDiag.Traybarcode = msg.GetSysinfo().GetTraybarcode()
		msgDiag.ProcessStep = msg.GetSysinfo().GetProcessStep()
		msgOp.Fixtureposition = msg.GetSysinfo().GetFixtureposition()
		msgOp.Traybarcode = msg.GetSysinfo().GetTraybarcode()
		msgOp.ProcessStep = msg.GetSysinfo().GetProcessStep()
		msgOp.GetOp().Status = pb.FixtureStatus_FIXTURE_STATUS_IDLE
		msgOp.GetOp().Cells = cells
		for i := 0; i < 10; i++ {
			switch {
			case i > 7:
				msgOp.GetOp().Status = pb.FixtureStatus_FIXTURE_STATUS_COMPLETE
			case i > 2:
				msgOp.GetOp().Status = pb.FixtureStatus_FIXTURE_STATUS_ACTIVE
			}

			log.Println(msgOp.GetOp().Status.String())

			for _, msg := range []proto.Message{msgDiag, msgOp} {
				pkt, err := proto.Marshal(msg)
				if err != nil {
					log.Fatal(err)
				}

				mx.Lock()
				if err := dev.SendBuf(pkt); err != nil {
					log.Fatal(err)
				}
				mx.Unlock()
			}
			time.Sleep(time.Second)
		}
	}
}

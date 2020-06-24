//+build !test

package main

import (
	"flag"
	"log"
	"time"

	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/go-socketcan/pkg/socketcan"
	"stash.teslamotors.com/rr/towercontroller"
	pb "stash.teslamotors.com/rr/towerproto"
)

const _confFileDef = "../../../configuration/statemachine/statemachine.yaml"

// nolint:funlen,gocognit,gocyclo // this is basically just a script
func main() {
	configFile := flag.String("conf", _confFileDef, "path to the configuration file")

	flag.Parse()

	conf, err := towercontroller.LoadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	fxrDevs := make([]socketcan.Interface, len(conf.Fixtures))

	var i int

	for _, id := range conf.Fixtures {
		// tx
		dev, err := socketcan.NewIsotpInterface(conf.CAN.Device, conf.CAN.TXID, id)
		if err != nil {
			log.Println("create ISOTP interface", err)
			return // return so the defer is called
		}

		if err = dev.SetCANFD(); err != nil {
			log.Println("set CANFD", err)
			return
		}

		fxrDevs[i] = dev
		i++

		//nolint:gocritic // they are wrong
		defer func() {
			_ = dev.Close()
		}()
	}

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

	// msgOp contains the status of the fixture. This what the TC state machine relies on
	msgOp := &pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Position: pb.FixturePosition_FIXTURE_POSITION_OPEN,
			},
		},
	}

	for _, devices := range fxrDevs {
		go func(dev socketcan.Interface) {
			log.Println("FIXTURE WAITING FOR MESSAGE FROM TOWER")

			for {
				buf, err := dev.RecvBuf()
				if err != nil {
					log.Println("RECV BUF", err)

					return // return so the defer is called
				}

				var msg pb.TowerToFixture
				if err = proto.Unmarshal(buf, &msg); err != nil {
					// just means this isn't the message we are looking for
					time.Sleep(time.Second)
					continue
				}

				if msg.GetSysinfo().GetProcessStep() == "" {
					// not what we are looking for
					time.Sleep(time.Second)
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

				msgDiag.Fixturebarcode = msg.GetSysinfo().GetFixturebarcode()
				msgDiag.Traybarcode = msg.GetSysinfo().GetTraybarcode()
				msgDiag.ProcessStep = msg.GetSysinfo().GetProcessStep()
				msgOp.Fixturebarcode = msg.GetSysinfo().GetFixturebarcode()
				msgOp.Traybarcode = msg.GetSysinfo().GetTraybarcode()
				msgOp.ProcessStep = msg.GetSysinfo().GetProcessStep()
				msgOp.GetOp().Status = pb.FixtureStatus_FIXTURE_STATUS_READY
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
							log.Println(err)
							return
						}

						if err := dev.SendBuf(pkt); err != nil {
							log.Println(err)
							return
						}
					}

					time.Sleep(time.Second)
				}
			}
		}(devices)
	}

	// allow the above routines to loop forever
	select {}
}

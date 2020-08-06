//+build !test

package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
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

	type devID struct {
		name string
		dev  socketcan.Interface
	}

	fxrDevs := make([]devID, len(conf.AllowedFixtures))

	var i int

	for _, name := range conf.AllowedFixtures {
		bus := "vcan0"
		if strings.HasPrefix(name, "02") {
			bus = "vcan1"
		}

		lvl := strings.Split(name, "-")[1]

		lvlID, err := strconv.Atoi(lvl)
		if err != nil {
			log.Println("generate tx and rx IDs", err)
			return
		}

		rx, tx := uint32(0x240+lvlID), uint32(0x1c0+lvlID)

		log.Printf("fixture: %s, bus: %s, rx: 0x%x, tx: 0x%x", name, bus, rx, tx)

		dev, err := socketcan.NewIsotpInterface(bus, rx, tx)
		if err != nil {
			log.Println("create ISOTP interface", err)
			return // return so the defer is called
		}

		if err = dev.SetCANFD(); err != nil {
			log.Println("set CANFD", err)
			return
		}

		if err = dev.SetRecvTimeout(time.Millisecond * 500); err != nil {
			log.Println("set RECVTIMEOUT", err)
			return
		}

		fxrDevs[i] = devID{
			name: name,
			dev:  dev,
		}
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
						VBusHv:               1,
						VBus_24:              2,
						PositionSwitchClosed: true,
						MicroTemp:            3,
						VRail_5V:             4,
						VRail_3V3:            5,
						IFan_24:              6,
						IStibFib_24:          7,
						VSolenoid_24:         8,
					},
					Outputs: &pb.FxrOutputs{
						StibEnableLine:     true,
						FixtureCloseEnable: true,
					},
				},
			},
		},
	}

	// msgOp contains the status of the fixture. This what the TC state machine relies on
	msgOp := &pb.FixtureToTower{
		Content: &pb.FixtureToTower_Op{
			Op: &pb.FixtureOperational{
				Position:        pb.FixturePosition_FIXTURE_POSITION_OPEN,
				EquipmentStatus: pb.EquipmentStatus_EQUIPMENT_STATUS_IN_OPERATION,
			},
		},
	}

	var mx sync.Mutex

	for _, devices := range fxrDevs {
		go func(did devID) {
			dev := did.dev

			for {
				var (
					buf    []byte
					msgt2f pb.TowerToFixture
				)

				log.Println("FIXTURE WAITING FOR MESSAGE FROM TOWER", did.name)

				for {
					msg := pb.FixtureToTower{
						Content: &pb.FixtureToTower_Op{
							Op: &pb.FixtureOperational{
								Status:          pb.FixtureStatus_FIXTURE_STATUS_IDLE,
								EquipmentStatus: pb.EquipmentStatus_EQUIPMENT_STATUS_IN_OPERATION,
							},
						},
						Fixturebarcode: fmt.Sprintf("CM2-63010-%s", did.name),
					}

					jb, err := proto.Marshal(&msg)
					if err != nil {
						log.Println("MARSHAL", err)
						return
					}

					mx.Lock()
					if err = dev.SendBuf(jb); err != nil {
						mx.Unlock()
						log.Println("SENDBUF", err)

						return
					}
					mx.Unlock()

					buf, err = dev.RecvBuf()
					if err != nil {
						continue
					}

					log.Println("MESSAGE RECEIVED")

					if err = proto.Unmarshal(buf, &msgt2f); err != nil {
						// just means this isn't the message we are looking for
						log.Println("wrong message")
						continue
					}

					if msgt2f.GetSysinfo().GetProcessStep() == "" {
						// not what we are looking for
						log.Println("empty process step")
						continue
					}

					if !strings.HasSuffix(strings.TrimSpace(msgt2f.GetSysinfo().GetFixturebarcode()), strings.TrimSpace(did.name)) {
						// not what we are looking for
						log.Println("wrong fixture")
						continue
					}

					if msgt2f.TransactionId <= 0 {
						log.Println("invalid transaction ID", msgt2f.TransactionId)
						continue
					}

					// received a process to run
					log.Println("RUNNING PROCESS", msgt2f.GetSysinfo().GetProcessStep())

					break
				}

				cells := make([]*pb.Cell, 64)
				cms := msgt2f.Recipe.GetCellMasks()
				transactionID := msgt2f.TransactionId

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

				msgDiag.Fixturebarcode = msgt2f.GetSysinfo().GetFixturebarcode()
				msgDiag.Traybarcode = msgt2f.GetSysinfo().GetTraybarcode()
				msgDiag.ProcessStep = msgt2f.GetSysinfo().GetProcessStep()
				msgDiag.TransactionId = transactionID
				msgOp.Fixturebarcode = msgt2f.GetSysinfo().GetFixturebarcode()
				msgOp.Traybarcode = msgt2f.GetSysinfo().GetTraybarcode()
				msgOp.ProcessStep = msgt2f.GetSysinfo().GetProcessStep()
				msgOp.GetOp().Status = pb.FixtureStatus_FIXTURE_STATUS_READY
				msgOp.GetOp().Cells = cells
				msgOp.TransactionId = transactionID

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

						mx.Lock()
						if err := dev.SendBuf(pkt); err != nil {
							mx.Unlock()
							log.Println(err)

							return
						}
						mx.Unlock()
					}

					time.Sleep(time.Second)
				}

				log.Println("DONE WITH PROCESS", msgt2f.GetSysinfo().GetProcessStep())
			}
		}(devices)
	}

	// allow the above routines to loop forever
	select {}
}

//+build !test

package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/go-socketcan/pkg/socketcan"
	"stash.teslamotors.com/rr/towercontroller"
	pb "stash.teslamotors.com/rr/towerproto"
)

const _confFileDef = "/etc/towercontroller.d/statemachine.yaml"

func main() {
	configFile := flag.String("conf", _confFileDef, "path to the configuration file")
	statName := flag.String("status", "idle", "fixture status to report")

	flag.Parse()

	var status pb.FixtureStatus

	switch *statName {
	case "active":
		status = pb.FixtureStatus_FIXTURE_STATUS_ACTIVE
	case "complete":
		status = pb.FixtureStatus_FIXTURE_STATUS_COMPLETE
	case "idle":
		status = pb.FixtureStatus_FIXTURE_STATUS_IDLE
	case "fault":
		status = pb.FixtureStatus_FIXTURE_STATUS_FAULTED
	case "ready":
		status = pb.FixtureStatus_FIXTURE_STATUS_READY
	default:
		log.Fatal("unknown status", *statName)
	}

	conf, err := towercontroller.LoadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	type devCtx struct {
		writer           socketcan.Interface
		fxbc, tbc, pstep string
	}

	fxDevs := make(map[string]devCtx)

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

		defer func() {
			_ = dev.Close()
		}()

		if err = dev.SetCANFD(); err != nil {
			log.Println("set CANFD", err)
			return
		}

		fxDevs[name] = devCtx{
			writer: dev,
			fxbc:   fmt.Sprintf("CM2-63010-%s", name),
			tbc:    "11223344" + []string{"A", "B", "C", "D"}[i%4],
			pstep:  "FORM_CYCLE",
		}

		i++
	}

	for {
		log.Println("writing", status.String())

		for _, ctx := range fxDevs {
			msgOp := &pb.FixtureToTower{
				Content: &pb.FixtureToTower_Op{
					Op: &pb.FixtureOperational{
						Status: status,
					},
				},
				Fixturebarcode: ctx.fxbc,
				Traybarcode:    ctx.tbc,
				ProcessStep:    ctx.pstep,
				TransactionId:  1,
			}

			pkt, err := proto.Marshal(msgOp)
			if err != nil {
				log.Println("marshal message body:", err)
				return
			}

			if err = ctx.writer.SendBuf(pkt); err != nil {
				log.Println("send buffer:", err)
				return
			}
		}

		time.Sleep(time.Second)
	}
}

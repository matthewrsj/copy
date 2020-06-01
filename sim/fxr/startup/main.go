//+build !test

package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/linklayer/go-socketcan/pkg/socketcan"
	"google.golang.org/protobuf/proto"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

const _confFileDef = "../../configuration/statemachine/statemachine.yaml"

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
	default:
		log.Fatal("unknown status", *statName)
	}

	conf, err := traycontrollers.LoadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	type devCtx struct {
		writer           socketcan.Interface
		fxbc, tbc, pstep string
	}

	fxDevs := make(map[string]devCtx)

	var i int

	for n, id := range conf.Fixtures {
		log.Printf("%d WRITING TO %d", id, conf.CAN.TXID)

		dev, err := socketcan.NewIsotpInterface(conf.CAN.Device, conf.CAN.TXID, id)
		if err != nil {
			log.Println("create ISOTP interface", err)
			return // return so the defer is called
		}

		defer func() {
			_ = dev.Close()
		}()

		fxDevs[n] = devCtx{
			writer: dev,
			fxbc:   fmt.Sprintf("SWIFT-01-A-%s", n),
			tbc:    "11223344" + []string{"A", "B", "C", "D"}[i%4],
			pstep:  "FORM_CYCLE",
		}

		i++
	}

	for {
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

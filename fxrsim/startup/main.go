package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/linklayer/go-socketcan/pkg/socketcan"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/rr/towercontroller"
	pb "stash.teslamotors.com/rr/towerproto"
)

const _confFileDef = "../../configuration/statemachine/statemachine.yaml"

func main() {
	configFile := flag.String("conf", _confFileDef, "path to the configuration file")

	flag.Parse()

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
						Status:   pb.FixtureStatus_FIXTURE_STATUS_ACTIVE,
						Position: pb.FixturePosition_FIXTURE_POSITION_OPEN,
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

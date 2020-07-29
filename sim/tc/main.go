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
	"stash.teslamotors.com/rr/traycontrollers"
)

const _confFileDef = "../../configuration/statemachine/statemachine.yaml"

// nolint:funlen,gocognit // this is basically just a script
func main() {
	configFile := flag.String("conf", _confFileDef, "path to the configuration file")

	flag.Parse()

	conf, err := traycontrollers.LoadConfig(*configFile)
	if err != nil {
		log.Fatal(err)
	}

	tcDev, err := socketcan.NewIsotpInterface(conf.CAN.Device, conf.CAN.RXID, conf.CAN.TXID)
	if err != nil {
		log.Fatal("create ISOTP listener", err)
	}

	defer func() {
		_ = tcDev.Close()
	}()

	if err = tcDev.SetCANFD(); err != nil {
		log.Println("set CANFD", err)
		return
	}

	if err = tcDev.SetRecvTimeout(time.Second * 2); err != nil {
		log.Println("set recv timeout:", err)
		return
	}

	recipe, err := towercontroller.LoadRecipe(conf.RecipeFile, conf.IngredientsFile, "FORM_REQ")
	if err != nil {
		log.Println("read recipe from configuration:", err)
		return
	}

	r := pb.Recipe{
		CellMasks:   []uint32{0xffff, 0xffff}, // fully-populated tray
		Formrequest: pb.FormRequest_FORM_REQUEST_START,
	}

	for _, ing := range recipe {
		var mode pb.RecipeStep_FormMode

		switch ing.Mode {
		case "FORM_REQ_CC":
			mode = pb.RecipeStep_FORM_MODE_CC
		case "FORM_REQ_CV":
			mode = pb.RecipeStep_FORM_MODE_CV
		default:
			mode = pb.RecipeStep_FORM_MODE_UNKNOWN_UNSPECIFIED
		}

		r.Steps = append(r.Steps, &pb.RecipeStep{
			Mode:          mode,
			ChargeCurrent: ing.ChargeCurrentAmps,
			MaxCurrent:    ing.MaxCurrentAmps,
			CutoffVoltage: ing.CutOffVoltage,
			CutoffCurrent: ing.CutOffCurrent,
			CutoffDv:      ing.CutOffDV,
			StepTimeout:   ing.StepTimeoutSeconds,
		})
	}

	t2f := pb.TowerToFixture{
		Recipe: &r,
		// arbitrary Sysinfo values
		Sysinfo: &pb.SystemInfo{
			Traybarcode:    "11223344A",
			Fixturebarcode: "SWIFT-01-A-01",
			ProcessStep:    "FORM_CYCLE",
		},
	}

	buf, err := proto.Marshal(&t2f)
	if err != nil {
		log.Println("marshal proto message:", err)
		return
	}

	for {
		if err := tcDev.SendBuf(buf); err != nil {
			log.Println("send buffer:", err)
			return
		}

		time.Sleep(time.Second * 10)
	}
}

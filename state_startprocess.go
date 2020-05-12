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

type StartProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *logrus.Logger
	CellAPIClient *cellapi.Client

	processStepName string
	tbc             TrayBarcode
	fxbc            FixtureBarcode
	rcpe            []ingredients
	cells           map[string]cellapi.CellData
	canErr, apiErr  error
}

// this is a simple function made long by logging
// nolint: funlen
func (s *StartProcess) action() {
	s.Logger.WithFields(logrus.Fields{
		"tray":         s.tbc.SN,
		"fixture_num":  s.fxbc.raw,
		"process_step": s.processStepName,
	}).Info("sending recipe and other information to FXR")

	recipe := pb.Recipe{Formrequest: pb.FormRequest_FORM_REQUEST_START}

	for _, ingredient := range s.rcpe {
		recipe.Steps = append(recipe.Steps, &pb.RecipeStep{
			Mode:          modeStringToEnum(ingredient.Mode),
			ChargeCurrent: ingredient.ChargeCurrentAmps,
			MaxCurrent:    ingredient.MaxCurrentAmps,
			CutOffVoltage: ingredient.CutOffVoltage,
			CutOffCurrent: ingredient.CutOffCurrent,
			CellDropOutV:  ingredient.CellDropOutVoltage,
			StepTimeout:   ingredient.StepTimeoutSeconds,
		})
	}

	if s.cells, s.apiErr = s.CellAPIClient.GetCellMap(s.tbc.SN); s.apiErr != nil {
		s.Logger.Error(s.apiErr)
		log.Println(s.apiErr)
		s.SetLast(true)

		return
	}

	cellMapConf, ok := s.Config.CellMap[s.tbc.O.String()]
	if !ok {
		err := fmt.Errorf("could not find orientation %s in configuration", s.tbc.O)
		s.Logger.Error(err)
		log.Println(err)
		s.SetLast(true)

		return
	}

	present := make([]bool, len(cellMapConf))

	for i, cell := range cellMapConf {
		_, ok := s.cells[cell]
		present[i] = ok
	}

	recipe.CellMasks = newCellMask(present)

	var dev socketcan.Interface

	if dev, s.canErr = socketcan.NewIsotpInterface(
		s.Config.CAN.Device, s.Config.CAN.TXID, s.Config.CAN.RXID,
	); s.canErr != nil {
		s.Logger.Error(s.canErr)
		log.Println(s.canErr)
		s.SetLast(true)

		return
	}

	defer dev.Close()

	var data []byte

	if data, s.canErr = proto.Marshal(&recipe); s.canErr != nil {
		s.Logger.Error(s.canErr)
		log.Println(s.canErr)
		s.SetLast(true)

		return
	}

	if s.canErr = dev.SendBuf(data); s.canErr != nil {
		s.Logger.Error(s.canErr)
		log.Println(s.canErr)
		s.SetLast(true)

		return
	}

	if err := s.CellAPIClient.UpdateProcessStatus(s.tbc.SN, s.processStepName, cellapi.StatusStart); err != nil {
		s.Logger.Error(err)
		log.Println(err)
		s.SetLast(true)

		return
	}

	s.Logger.WithFields(logrus.Fields{
		"tray":         s.tbc.SN,
		"fixture_num":  s.fxbc.raw,
		"process_step": s.processStepName,
	}).Trace("sent recipe and other information to FXR")
}

func (s *StartProcess) Actions() []func() {
	return []func(){
		s.action,
	}
}

func (s *StartProcess) Next() statemachine.State {
	next := &InProcess{
		Config:          s.Config,
		Logger:          s.Logger,
		CellAPIClient:   s.CellAPIClient,
		tbc:             s.tbc,
		fxbc:            s.fxbc,
		cells:           s.cells,
		processStepName: s.processStepName,
	}
	s.Logger.WithField("tray", s.tbc.SN).Tracef("next state: %s", statemachine.NameOf(next))

	return next
}

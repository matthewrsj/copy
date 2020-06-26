// Package towercontroller implements the state machine (statemachine.State) for the RR formation tower controller.
package towercontroller

import (
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/go-socketcan/pkg/socketcan"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

// StartProcess sends the recipe information to the FXR
type StartProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client

	processStepName string
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	steps           traycontrollers.StepConfiguration
	cells           map[string]cellapi.CellData
	canErr, apiErr  error
	manual          bool
	mockCellAPI     bool
	recipeVersion   int
}

func (s *StartProcess) action() {
	s.Logger.Info("sending recipe and other information to FXR")

	twr2Fxr := pb.TowerToFixture{
		Recipe: &pb.Recipe{Formrequest: pb.FormRequest_FORM_REQUEST_START},
		Sysinfo: &pb.SystemInfo{
			Traybarcode:    s.tbc.Raw,
			Fixturebarcode: s.fxbc.Raw,
			ProcessStep:    s.processStepName,
		},
	}

	for _, step := range s.steps {
		twr2Fxr.Recipe.Steps = append(twr2Fxr.Recipe.Steps, &pb.RecipeStep{
			Mode:          modeStringToEnum(step.Mode),
			ChargeCurrent: step.ChargeCurrentAmps,
			MaxCurrent:    step.MaxCurrentAmps,
			CutOffVoltage: step.CutOffVoltage,
			CutOffCurrent: step.CutOffCurrent,
			CellDropOutV:  step.CellDropOutVoltage,
			StepTimeout:   step.StepTimeoutSeconds,
		})
	}

	if !s.mockCellAPI {
		if s.cells, s.apiErr = s.CellAPIClient.GetCellMap(s.tbc.SN); s.apiErr != nil {
			fatalError(s, s.Logger, s.apiErr)
			return
		}
	} else {
		s.Logger.Warn("cell API mocked, skipping GetCellMap and populating a few cells")
		s.cells = map[string]cellapi.CellData{
			"A01": {
				Position: "A01",
				Serial:   "TESTA01",
				IsEmpty:  false,
			},
			"A02": {
				Position: "A02",
				Serial:   "TESTA02",
				IsEmpty:  false,
			},
			"A03": {
				Position: "A03",
				Serial:   "TESTA03",
				IsEmpty:  false,
			},
			"A04": {
				Position: "A04",
				Serial:   "TESTA04",
				IsEmpty:  false,
			},
		}
	}

	cellMapConf, ok := s.Config.CellMap[s.tbc.O.String()]
	if !ok {
		fatalError(s, s.Logger, fmt.Errorf("could not find orientation %s in configuration", s.tbc.O))
		return
	}

	present := make([]bool, len(cellMapConf))

	for i, cell := range cellMapConf {
		_, ok = s.cells[cell]
		present[i] = ok
	}

	twr2Fxr.Recipe.CellMasks = newCellMask(present)

	var fxrID uint32

	if fxrID, ok = s.Config.Fixtures[IDFromFXR(s.fxbc)]; !ok {
		fatalError(s, s.Logger, fmt.Errorf("fixture %s not configured for tower controller", IDFromFXR(s.fxbc)))
		return
	}

	var dev socketcan.Interface

	if dev, s.canErr = socketcan.NewIsotpInterface(
		s.Config.CAN.Device, fxrID, s.Config.CAN.TXID,
	); s.canErr != nil {
		fatalError(s, s.Logger, s.canErr)
		return
	}

	defer func() {
		_ = dev.Close()
	}()

	if err := dev.SetCANFD(); err != nil {
		fatalError(s, s.Logger, err)
		return
	}

	var data []byte

	if data, s.canErr = proto.Marshal(&twr2Fxr); s.canErr != nil {
		fatalError(s, s.Logger, s.canErr)
		return
	}

	if s.canErr = dev.SendBuf(data); s.canErr != nil {
		fatalError(s, s.Logger, s.canErr)
		return
	}

	if s.manual {
		if !s.mockCellAPI {
			if err := s.CellAPIClient.UpdateProcessStatus(s.tbc.SN, s.processStepName, cellapi.StatusStart); err != nil {
				fatalError(s, s.Logger, fmt.Errorf("UpdateProcessStatus: %v", err))
				return
			}
		} else {
			s.Logger.Warn("cell API mocked, skipping UpdateProcessStatus")
		}
	}

	s.Logger.Debug("sent recipe and other information to FXR")
}

// Actions returns the action functions for this state
func (s *StartProcess) Actions() []func() {
	return []func(){
		s.action,
	}
}

// Next returns the next state to run after this one
func (s *StartProcess) Next() statemachine.State {
	next := &InProcess{
		Config:          s.Config,
		Logger:          s.Logger,
		CellAPIClient:   s.CellAPIClient,
		tbc:             s.tbc,
		fxbc:            s.fxbc,
		cells:           s.cells,
		processStepName: s.processStepName,
		manual:          s.manual,
		mockCellAPI:     s.mockCellAPI,
		recipeVersion:   s.recipeVersion,
	}
	s.Logger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

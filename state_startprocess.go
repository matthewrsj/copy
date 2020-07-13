// Package towercontroller implements the state machine (statemachine.State) for the RR formation tower controller.
package towercontroller

import (
	"fmt"
	"time"

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

	childLogger     *zap.SugaredLogger
	processStepName string
	transactID      int64
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	steps           traycontrollers.StepConfiguration
	cells           map[string]cellapi.CellData
	canErr          error
	manual          bool
	mockCellAPI     bool
	recipeVersion   int

	fxrInfo *FixtureInfo
}

func (s *StartProcess) action() {
	s.childLogger.Info("sending recipe and other information to FXR")

	twr2Fxr := pb.TowerToFixture{
		Recipe: &pb.Recipe{Formrequest: pb.FormRequest_FORM_REQUEST_START},
		Sysinfo: &pb.SystemInfo{
			Traybarcode:    s.tbc.Raw,
			Fixturebarcode: s.fxbc.Raw,
			ProcessStep:    s.processStepName,
		},
		TransactionId: s.transactID,
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

	var err error

	s.cells, err = getCellMap(s.mockCellAPI, s.childLogger, s.CellAPIClient, s.tbc.SN)
	if err != nil {
		fatalError(s, s.childLogger, err)
		return
	}

	s.childLogger.Infow("GetCellMap complete", "cells", s.cells)

	cellMapConf, ok := s.Config.CellMap[s.tbc.O.String()]
	if !ok {
		fatalError(s, s.childLogger, fmt.Errorf("could not find orientation %s in configuration", s.tbc.O))
		return
	}

	present := make([]bool, len(cellMapConf))

	for i, cell := range cellMapConf {
		_, ok = s.cells[cell]
		present[i] = ok
	}

	twr2Fxr.Recipe.CellMasks = newCellMask(present)

	var fConf fixtureConf

	if fConf, ok = s.Config.Fixtures[IDFromFXR(s.fxbc)]; !ok {
		fatalError(s, s.childLogger, fmt.Errorf("fixture %s not configured for tower controller", IDFromFXR(s.fxbc)))
		return
	}

	var dev socketcan.Interface

	if dev, s.canErr = socketcan.NewIsotpInterface(fConf.Bus, fConf.RX, fConf.TX); s.canErr != nil {
		fatalError(s, s.childLogger, s.canErr)
		return
	}

	defer func() {
		_ = dev.Close()
	}()

	if err := dev.SetCANFD(); err != nil {
		fatalError(s, s.childLogger, err)
		return
	}

	if err := dev.SetSendTimeout(time.Second * 3); err != nil {
		s.childLogger.Warnw("unable to set send timeout", "error", err)
	}

	if err := dev.SetRecvTimeout(time.Second * 3); err != nil {
		s.childLogger.Warnw("unable to set recv timeout", "error", err)
	}

	var data []byte

	if data, s.canErr = proto.Marshal(&twr2Fxr); s.canErr != nil {
		fatalError(s, s.childLogger, s.canErr)
		return
	}

	// performHandshake blocks until the FXR acknowledges receipt of recipe
	s.performHandshake(dev, data)

	if s.manual {
		if !s.mockCellAPI {
			if err := s.CellAPIClient.UpdateProcessStatus(s.tbc.SN, s.processStepName, cellapi.StatusStart); err != nil {
				fatalError(s, s.childLogger, fmt.Errorf("UpdateProcessStatus: %v", err))
				return
			}
		} else {
			s.childLogger.Warn("cell API mocked, skipping UpdateProcessStatus")
		}
	}

	s.childLogger.Debug("sent recipe and other information to FXR")
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
		childLogger:     s.childLogger,
		tbc:             s.tbc,
		fxbc:            s.fxbc,
		cells:           s.cells,
		processStepName: s.processStepName,
		manual:          s.manual,
		mockCellAPI:     s.mockCellAPI,
		recipeVersion:   s.recipeVersion,
		fxrInfo:         s.fxrInfo,
	}
	s.childLogger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

func (s *StartProcess) performHandshake(dev socketcan.Interface, data []byte) {
	for {
		if err := dev.SendBuf(data); s.canErr != nil {
			s.childLogger.Warnw("unable to send data to FXR", "error", err)
			continue
		}

		buf, err := dev.RecvBuf()
		if err != nil {
			s.childLogger.Warnw("unable to receive data from FXR", "error", err)
			continue
		}

		var msg pb.FixtureToTower
		if err := proto.Unmarshal(buf, &msg); err != nil {
			s.childLogger.Warnw("unable to unmarshal data from FXR", "error", err)
			continue
		}

		if msg.TransactionId != s.transactID {
			s.childLogger.Warnw(
				"transaction ID from FXR did not match transaction ID sent",
				"fxr_transaction_id", msg.TransactionId, "sent_transaction_id", s.transactID,
			)

			continue
		}

		break
	}
}

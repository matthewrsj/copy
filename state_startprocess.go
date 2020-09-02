// Package towercontroller implements the state machine (statemachine.State) for the RR formation tower controller.
package towercontroller

import (
	"encoding/json"
	"fmt"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

// StartProcess sends the recipe information to the FXR
type StartProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client
	Publisher     *protostream.Socket
	SubscribeChan <-chan *protostream.Message

	childLogger     *zap.SugaredLogger
	processStepName string
	transactID      string
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	steps           traycontrollers.StepConfiguration
	cells           map[string]cellapi.CellData
	smFatal         bool
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
			Mode:            modeStringToEnum(step.Mode),
			ChargeCurrent:   step.ChargeCurrentAmps,
			MaxCurrent:      step.MaxCurrentAmps,
			CutoffVoltage:   step.CutOffVoltage,
			CutoffCurrent:   step.CutOffCurrent,
			CutoffDv:        step.CutOffDV,
			StepTimeout:     step.StepTimeoutSeconds,
			ChargePower:     step.ChargePower,
			CutoffAh:        step.CutOffAH,
			EndingStyle:     endingStyleStringToEnum(step.EndingStyle),
			VCellMinQuality: step.VCellMinQuality,
			VCellMaxQuality: step.VCellMaxQuality,
		})
	}

	var err error

	s.cells, err = getCellMap(s.mockCellAPI, s.childLogger, s.CellAPIClient, s.tbc.SN)
	if err != nil {
		s.childLogger.Errorw("get cell map", "error", err)
		s.smFatal = true

		return
	}

	s.childLogger.Infow("GetCellMap complete", "cells", s.cells)

	cellMapConf, ok := s.Config.CellMap[s.tbc.O.String()]
	if !ok {
		s.childLogger.Error(fmt.Errorf("could not find orientation %s in configuration", s.tbc.O))
		s.smFatal = true

		return
	}

	present := make([]bool, len(cellMapConf))

	for i, cell := range cellMapConf {
		_, ok = s.cells[cell]
		present[i] = ok
	}

	twr2Fxr.Recipe.CellMasks = newCellMask(present)

	// performHandshake blocks until the FXR acknowledges receipt of recipe
	s.performHandshake(&twr2Fxr)

	if s.manual {
		if !s.mockCellAPI {
			if err := s.CellAPIClient.UpdateProcessStatus(s.tbc.SN, s.processStepName, cellapi.StatusStart); err != nil {
				s.childLogger.Errorw("UpdateProcessStatus", "error", err)
				s.smFatal = true

				return
			}
		} else {
			s.childLogger.Warn("cell API mocked, skipping UpdateProcessStatus")
		}
	}

	s.childLogger.Info("sent recipe and other information to FXR")
}

// Actions returns the action functions for this state
func (s *StartProcess) Actions() []func() {
	return []func(){
		s.action,
	}
}

// Next returns the next state to run after this one
func (s *StartProcess) Next() statemachine.State {
	var next statemachine.State

	switch {
	case s.smFatal:
		next = &Idle{
			Config:        s.Config,
			Logger:        s.Logger,
			CellAPIClient: s.CellAPIClient,
			Publisher:     s.Publisher,
			SubscribeChan: s.SubscribeChan,
			Manual:        s.manual,
			MockCellAPI:   s.mockCellAPI,
			FXRInfo:       s.fxrInfo,
		}
	default:
		next = &InProcess{
			Config:          s.Config,
			Logger:          s.Logger,
			CellAPIClient:   s.CellAPIClient,
			Publisher:       s.Publisher,
			SubscribeChan:   s.SubscribeChan,
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
	}

	s.childLogger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

func (s *StartProcess) performHandshake(msg proto.Message) {
	for {
		s.childLogger.Info("attempting handshake with FXR")

		if err := sendProtoMessage(s.Publisher, msg, IDFromFXR(s.fxbc)); err != nil {
			s.childLogger.Warnw("failed to send proto message, retrying", "error", err)
			continue
		}

		s.childLogger.Debug("reading from subscribeChan")
		lMsg := <-s.SubscribeChan
		s.childLogger.Debug("received message from subscribeChan")

		var event protostream.ProtoMessage

		if err := json.Unmarshal(lMsg.Msg.Body, &event); err != nil {
			s.childLogger.Debugw("unmarshal JSON frame", "error", err, "bytes", string(lMsg.Msg.Body))
			continue
		}

		var msg pb.FixtureToTower
		if err := proto.Unmarshal(event.Body, &msg); err != nil {
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

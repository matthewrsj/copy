// Package towercontroller implements the state machine (statemachine.State) for the RR formation tower controller.
package towercontroller

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cenkalti/backoff"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cdcontroller"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

// StartProcess sends the recipe information to the FXR
type StartProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cdcontroller.CellAPIClient
	Publisher     *protostream.Socket

	childLogger     *zap.SugaredLogger
	processStepName string
	transactID      string
	tbc             cdcontroller.TrayBarcode
	fxbc            cdcontroller.FixtureBarcode
	steps           cdcontroller.StepConfiguration
	stepType        string
	cells           map[string]cdcontroller.CellData
	smFatal         bool
	mockCellAPI     bool
	unload          bool
	recipeVersion   int

	fxrInfo *FixtureInfo
}

const _readinessTimeoutLen = time.Minute

func (s *StartProcess) action() {
	s.childLogger = s.Logger.With(
		"tray", s.tbc.SN,
		"fixture", s.fxbc.Raw,
		"process_step", s.processStepName,
		"transaction_id", s.transactID,
	)

	if s.stepType != cdcontroller.AllowedStepType {
		s.childLogger.Errorw("incorrect step type for charge/discharge", "step_type", s.stepType)
		s.unload = true

		return
	}

	bo := backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = 0 // infinite

	if !s.mockCellAPI {
		_ = backoff.Retry(func() error {
			var err error
			if s.steps, err = s.CellAPIClient.GetStepConfiguration(s.tbc.Raw); err != nil {
				s.Logger.Errorw("get step configuration", "error", err)
				return err
			}

			s.Logger.Infow("retrieved steps from cell API", "num_steps", len(s.steps))

			if len(s.steps) == 0 {
				s.Logger.Error("invalid empty step configuration retrieved from Cell API")
				return errors.New("empty step configuration retrieved from Cell API")
			}

			return nil
		}, bo)
	}

	s.childLogger.Info("sending recipe and other information to FXR")

	twr2Fxr := tower.TowerToFixture{
		Recipe: &tower.Recipe{FormRequest: tower.FormRequest_FORM_REQUEST_START},
		Info: &tower.Info{
			TrayBarcode:     s.tbc.Raw,
			FixtureLocation: s.fxbc.Raw,
			RecipeName:      s.processStepName,
			RecipeVersion:   uint32(s.recipeVersion),
			TransactionId:   s.transactID,
			Orientation:     tower.TrayOrientation(s.tbc.O),
		},
	}

	for _, step := range s.steps {
		twr2Fxr.Recipe.Steps = append(twr2Fxr.Recipe.Steps, &tower.RecipeStep{
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

	bo = backoff.NewExponentialBackOff()
	bo.MaxElapsedTime = 0 // infinite retry
	bo.MaxInterval = time.Minute

	// no error from Retry as this is infinite
	_ = backoff.Retry(func() error {
		s.cells, err = getCellMap(s.mockCellAPI, s.childLogger, s.CellAPIClient, s.tbc.SN, s.processStepName)
		if err != nil {
			s.childLogger.Errorw("get cell map", "error", err)
			return err
		}

		return nil
	}, bo)

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

	if !isCommissionRecipe(s.processStepName) {
		twr2Fxr.Op = getOpSnapshot(s.childLogger, s.Config, s.tbc.Raw)
	}

	// performHandshake blocks until the FXR acknowledges receipt of recipe
	s.performHandshake(&twr2Fxr)

	if !s.mockCellAPI {
		// out of band and ignoring all errors update Cell API that we started
		// does not affect any process just makes it easier to find data
		go func() {
			s.childLogger.Debug("updating process status", "status", "end")

			err := s.CellAPIClient.UpdateProcessStatus(s.tbc.SN, s.fxbc.Raw, cdcontroller.StatusStart)
			if err != nil {
				s.childLogger.Warnw("unable to update Cell API of recipe start", "error", err)
			}
		}()
	} else {
		s.childLogger.Warn("cell API mocked, skipping UpdateProcessStatus")
	}
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
			MockCellAPI:   s.mockCellAPI,
			FXRInfo:       s.fxrInfo,
		}
	case s.unload:
		next = &EndProcess{
			Config:          s.Config,
			Logger:          s.Logger,
			CellAPIClient:   s.CellAPIClient,
			Publisher:       s.Publisher,
			childLogger:     s.childLogger,
			tbc:             s.tbc,
			fxbc:            s.fxbc,
			cells:           s.cells,
			processStepName: s.processStepName,
			mockCellAPI:     s.mockCellAPI,
			recipeVersion:   s.recipeVersion,
			fxrInfo:         s.fxrInfo,
			skipClose:       true, // do not close the process step, error here
		}
	default:
		next = &InProcess{
			Config:          s.Config,
			Logger:          s.Logger,
			CellAPIClient:   s.CellAPIClient,
			Publisher:       s.Publisher,
			childLogger:     s.childLogger,
			tbc:             s.tbc,
			fxbc:            s.fxbc,
			cells:           s.cells,
			processStepName: s.processStepName,
			mockCellAPI:     s.mockCellAPI,
			recipeVersion:   s.recipeVersion,
			fxrInfo:         s.fxrInfo,
		}
	}

	s.childLogger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

func (s *StartProcess) performHandshake(msg proto.Message) {
	start := time.Now()

	s.childLogger.Info("checking that FXR is ready to handshake")

	s.unload = true

	for time.Since(start) < _readinessTimeoutLen {
		rMsg, err := s.fxrInfo.FixtureState.GetOp()
		if err != nil {
			s.childLogger.Errorw("get fixture operational message", "error", err)
			time.Sleep(time.Second) // give it a chance to update

			continue
		}

		if rMsg.GetOp().GetStatus() != tower.FixtureStatus_FIXTURE_STATUS_READY {
			s.childLogger.Infow("FXR not yet ready for recipe", "status", rMsg.GetOp().GetStatus())
			time.Sleep(time.Second) // give it a chance to update

			continue
		}

		// if we got this far no timeout
		s.unload = false

		break
	}

	if s.unload {
		s.childLogger.Warn("timeout trying to send recipe to FXR")
		return
	}

	// reset timeout to true, make below loop turn it off again
	s.unload = true
	start = time.Now()

	for time.Since(start) < _readinessTimeoutLen {
		s.childLogger.Info("attempting handshake with FXR")

		if err := sendProtoMessage(s.Publisher, msg, IDFromFXR(s.fxbc)); err != nil {
			s.childLogger.Warnw("failed to send proto message, retrying", "error", err)
			continue
		}

		time.Sleep(time.Second) // give fixture a chance to update. cycle rate is anywhere from 1-3 seconds

		rMsg, err := s.fxrInfo.FixtureState.GetOp()
		if err != nil {
			s.childLogger.Warnw("check transaction ID; get fixture operational message", "error", err)
			time.Sleep(time.Second) // give it a chance to update

			continue
		}

		if rMsg.GetInfo().GetTransactionId() != s.transactID {
			s.childLogger.Warnw(
				"transaction ID from FXR did not match transaction ID sent",
				"fxr_transaction_id", rMsg.GetInfo().GetTransactionId(), "sent_transaction_id", s.transactID,
			)
			time.Sleep(time.Second) // give it a chance to update

			continue
		}

		// no timeout if we got here
		s.unload = false

		break
	}

	if s.unload {
		s.childLogger.Warn("timeout trying to send recipe to FXR")
	} else {
		s.childLogger.Info("sent recipe and other information to FXR")
	}
}

func getFaultRecord(logger *zap.SugaredLogger, conf Configuration, tid string) (cdcontroller.FaultRecord, error) {
	c := http.Client{Timeout: time.Second * 5}

	resp, err := c.Get(fmt.Sprintf("%s%s?%s=%s", conf.Remote, cdcontroller.TrayFaultEndpoint, cdcontroller.TrayIDQueryParameter, tid))
	if err != nil {
		logger.Errorw("unable to query latest fault record for tray", "error", err)
		return cdcontroller.FaultRecord{}, err
	}

	if resp.StatusCode != http.StatusOK {
		logger.Warnw("latest fault record query response NOT OK", "status", resp.Status, "status_code", resp.StatusCode)
		return cdcontroller.FaultRecord{}, err
	}

	rb, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		logger.Errorw("unable to read fault record query response", "error", err)
		return cdcontroller.FaultRecord{}, err
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	var fr cdcontroller.FaultRecord
	if err = json.Unmarshal(rb, &fr); err != nil {
		logger.Errorw("unable to unmarshal fault record query response", "error", err)
		return cdcontroller.FaultRecord{}, err
	}

	return fr, nil
}

func getOpSnapshot(logger *zap.SugaredLogger, conf Configuration, tid string) *tower.FixtureOperational {
	fr, err := getFaultRecord(logger, conf, tid)
	if err != nil {
		return nil
	}

	var ops tower.FixtureOperational
	if err = proto.Unmarshal(fr.OpSnapshot, &ops); err != nil {
		logger.Errorw("unable to unmarshal operational snapshot in fault record query response", "error", err)
		return nil
	}

	logger.Info("ops snapshot received from C/D Controller", "snapshot", ops.String())

	return &ops
}

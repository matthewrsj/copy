package towercontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cdcontroller"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

// InProcess monitors the FXR proto for the state to change
type InProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cdcontroller.CellAPIClient
	Publisher     *protostream.Socket

	childLogger     *zap.SugaredLogger
	tbc             cdcontroller.TrayBarcode
	fxbc            cdcontroller.FixtureBarcode
	processStepName string
	fixtureFault    bool
	mockCellAPI     bool
	returnToIdle    bool
	cells           map[string]cdcontroller.CellData
	cellResponse    []*tower.Cell
	recipeVersion   int

	fxrInfo *FixtureInfo
}

func (i *InProcess) action() {
	i.childLogger.Info("monitoring fixture to go to complete or fault")
	i.fxrInfo.Avail.Set(StatusActive)

	for { // loop until status updates to COMPLETE/FAULTED
		// TODO: first look for it to go to IN_PROGRESS
		//       then COMPLETE/FAULTED
		//       if it goes back to IDLE it lost the recipe, so unload
		var (
			msg *tower.FixtureToTower
			err error
		)

		if msg, err = i.fxrInfo.FixtureState.GetOp(); err != nil {
			i.childLogger.Warnw("get operational fixture status", "error", err)
			// wait a second for it to update
			// TODO: time out this operation. If fixture status doesn't update in a certain amount of time we should
			//       attempt to unload the tray.
			time.Sleep(time.Second)

			continue
		}

		i.childLogger.Debugw("got FixtureToTower message", "msg", msg.String())

		var statusMsg string

		switch s := msg.GetOp().GetStatus(); s {
		case tower.FixtureStatus_FIXTURE_STATUS_COMPLETE:
			statusMsg = "fixture done with tray"

			i.childLogger.Info(statusMsg)

			i.cellResponse = msg.GetOp().GetCells()

			// post this clear even if this is a commissioning tray since
			// 1) it returns 200 even if there is nothing to clear
			// 2) if there is a fault for this tray somehow we don't want to resume
			//    with it when commissioning a new fixture
			postClearFaultToRemote(i.childLogger, i.Config, i.tbc.Raw, i.fxbc.Raw)

			return
		case tower.FixtureStatus_FIXTURE_STATUS_FAULTED:
			statusMsg = "fixture done with tray; fixture faulted"

			if msg.GetOp().GetFireAlarmStatus() > tower.FireAlarmStatus_FIRE_ALARM_UNKNOWN_UNSPECIFIED {
				statusMsg += fmt.Sprintf("; fire alarm %s triggered, not requesting unload", msg.GetOp().GetFireAlarmStatus().String())
				i.returnToIdle = true
			}

			i.fixtureFault = true

			i.childLogger.Infow("cell statuses when faulted", "cells", msg.GetOp().GetCells())
			i.childLogger.Info(statusMsg)

			if !isCommissionRecipe(i.processStepName) {
				postOpSnapshotToRemote(i.childLogger, i.Config, i.tbc.Raw, i.fxbc.Raw, msg.GetOp())
				holdTrayIfFaultsExceedLimit(i.childLogger, i.CellAPIClient, i.Config, i.mockCellAPI, i.tbc.Raw)
			}

			return
		default:
			i.childLogger.Debugw("received fixture_status update", "status", s.String())
			// give it a second to update
			time.Sleep(time.Second)
		}
	}
}

// Actions returns the action functions for this state
func (i *InProcess) Actions() []func() {
	return []func(){
		i.action,
	}
}

// Next returns the next state to run
func (i *InProcess) Next() statemachine.State {
	var next statemachine.State
	if i.returnToIdle {
		next = &Idle{
			Config:        i.Config,
			Logger:        i.Logger,
			CellAPIClient: i.CellAPIClient,
			Publisher:     i.Publisher,
			MockCellAPI:   i.mockCellAPI,
			FXRInfo:       i.fxrInfo,
		}
	} else {
		next = &EndProcess{
			Config:          i.Config,
			Logger:          i.Logger,
			CellAPIClient:   i.CellAPIClient,
			Publisher:       i.Publisher,
			childLogger:     i.childLogger,
			tbc:             i.tbc,
			fxbc:            i.fxbc,
			processStepName: i.processStepName,
			fixtureFault:    i.fixtureFault,
			cellResponse:    i.cellResponse,
			cells:           i.cells,
			mockCellAPI:     i.mockCellAPI,
			recipeVersion:   i.recipeVersion,
			fxrInfo:         i.fxrInfo,
		}
	}

	i.childLogger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

func postOpSnapshotToRemote(logger *zap.SugaredLogger, conf Configuration, tray, location string, op *tower.FixtureOperational) {
	opData, err := proto.Marshal(op)
	if err != nil {
		logger.Errorw("unable to marshal operational message for snapshot", "error", err)
		return
	}

	tr := cdcontroller.TrayFaultRequest{
		OpSnapshot: opData,
		Tray:       tray,
		Location:   location,
	}

	jb, err := json.Marshal(tr)
	if err != nil {
		logger.Errorw("unable to marshal tray fault request", "error", err)
		return
	}

	c := http.Client{Timeout: time.Second * 5}

	resp, err := c.Post(fmt.Sprintf("%s%s", conf.Remote, cdcontroller.TrayFaultEndpoint), "application/json", bytes.NewReader(jb))
	if err != nil {
		logger.Errorw("unable to write tray fault request", "error", err)
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		logger.Errorw("status NOT OK when writing tray fault request", "status", resp.Status, "status_code", resp.StatusCode)
	}
}

func postClearFaultToRemote(logger *zap.SugaredLogger, conf Configuration, tray, location string) {
	tr := cdcontroller.TrayFaultRequest{
		Clear:    true,
		Tray:     tray,
		Location: location,
	}

	jb, err := json.Marshal(tr)
	if err != nil {
		logger.Errorw("unable to marshal tray fault request", "error", err)
		return
	}

	c := http.Client{Timeout: time.Second * 5}

	resp, err := c.Post(fmt.Sprintf("%s%s", conf.Remote, cdcontroller.TrayFaultEndpoint), "application/json", bytes.NewReader(jb))
	if err != nil {
		logger.Errorw("unable to write tray fault request", "error", err)
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		logger.Errorw("status NOT OK when writing tray fault request", "status", resp.Status, "status_code", resp.StatusCode)
	}
}

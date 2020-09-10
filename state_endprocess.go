package towercontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	"stash.teslamotors.com/rr/protostream"
	pb "stash.teslamotors.com/rr/towerproto"
	"stash.teslamotors.com/rr/traycontrollers"
)

const _unloadEndpoint = "/unload"

// EndProcess informs the cell API of process completion. This is the last state.
type EndProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cellapi.Client
	Publisher     *protostream.Socket

	childLogger     *zap.SugaredLogger
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	cells           map[string]cellapi.CellData
	cellResponse    []*pb.Cell
	processStepName string
	smFatal         bool
	fixtureFault    bool
	manual          bool
	mockCellAPI     bool
	recipeVersion   int

	fxrInfo *FixtureInfo
}

func (e *EndProcess) action() {
	// only update cell API on SWIFT. On C Tower and beyond this is done by CND
	if e.manual {
		if !e.mockCellAPI {
			e.childLogger.Debugw("UpdateProcessStatus", "process_name", e.processStepName)

			// only try to close the process step if this is a normal recipe
			if e.processStepName != traycontrollers.CommissionSelfTestRecipeName {
				if err := e.CellAPIClient.UpdateProcessStatus(e.tbc.SN, e.processStepName, cellapi.StatusEnd); err != nil {
					// keep trying the other transactions
					e.childLogger.Warn(err)
				}
			}
		} else {
			e.childLogger.Warn("cell API mocked, skipping UpdateProcessStatus")
		}
	}

	if len(e.cells) == 0 { // we short-circuited here or something went wrong, just re-get the map
		e.childLogger.Info("empty cell map, querying API for new map")

		var err error
		if e.cells, err = getCellMap(e.mockCellAPI, e.childLogger, e.CellAPIClient, e.tbc.SN); err != nil {
			e.childLogger.Errorw("get cell map", "error", err)
			e.smFatal = true

			return
		}
	}

	if e.manual {
		e.setCellStatusesSWIFT()
	} else if e.processStepName != traycontrollers.CommissionSelfTestRecipeName {
		e.setCellStatuses()
	}

	if !e.manual && !e.mockCellAPI {
		e.childLogger.Infow("closing process step", "recipe_name", e.processStepName, "recipe_version", e.recipeVersion)

		if e.processStepName != traycontrollers.CommissionSelfTestRecipeName {
			if err := e.CellAPIClient.CloseProcessStep(e.tbc.SN, e.processStepName, e.recipeVersion); err != nil {
				e.childLogger.Error("close process status", "error", err)
			}
		}
	}

	// TODO: determine how to inform cell API of fault
	msg := "tray complete"
	if e.fixtureFault {
		msg += "; fixture faulted"
	}

	e.childLogger.Info(msg)

	// if this is manual we are done
	if e.manual {
		e.childLogger.Info("done with tray")
		return
	}

	tc := trayComplete{
		ID:     e.tbc.Raw,
		Aisle:  e.Config.Loc.Aisle,
		Column: e.fxbc.Tower,
		Level:  e.fxbc.Fxn,
	}

	b, err := json.Marshal(tc)
	if err != nil {
		e.childLogger.Errorw("json marshal tray complete", "error", err)
		e.smFatal = true

		return
	}

	// wait for FXR to report open before calling for unload
	e.childLogger.Info("waiting for fixture to report open position")
	e.waitForOpen()
	e.childLogger.Info("sending unload request")

	resp, err := http.Post(e.Config.Remote+_unloadEndpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		e.childLogger.Errorw("post unload request", "error", err)
		e.smFatal = true

		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != 200 {
		e.childLogger.Errorw("http post", "response", fmt.Errorf("response NOT OK: %v, %v", resp.StatusCode, resp.Status))
		e.smFatal = true

		return
	}

	e.childLogger.Info("done with tray")
}

// Actions returns the action functions for this state
func (e *EndProcess) Actions() []func() {
	if e.manual {
		// this is the last state for manual
		e.SetLast(true)
	}

	return []func(){
		e.action,
	}
}

// Next returns the next state to run
func (e *EndProcess) Next() statemachine.State {
	if e.manual {
		// all done
		e.childLogger.Debug("statemachine exiting")
		return nil
	}

	var next statemachine.State

	switch {
	case e.smFatal:
		next = &Idle{
			Config:        e.Config,
			Logger:        e.Logger,
			CellAPIClient: e.CellAPIClient,
			Publisher:     e.Publisher,
			Manual:        e.manual,
			MockCellAPI:   e.mockCellAPI,
			FXRInfo:       e.fxrInfo,
		}
	default:
		next = &Unloading{
			Config:        e.Config,
			Logger:        e.Logger,
			CellAPIClient: e.CellAPIClient,
			Publisher:     e.Publisher,
			childLogger:   e.childLogger,
			mockCellAPI:   e.mockCellAPI,
			fxbc:          e.fxbc,
			fxrInfo:       e.fxrInfo,
		}
	}

	e.childLogger.Debugw("transitioning to next state", "next", statemachine.NameOf(next))

	return next
}

type trayComplete struct {
	ID     string `json:"id"`
	Aisle  string `json:"aisle"`
	Column string `json:"column"`
	Level  string `json:"level"`
}

func (e *EndProcess) setCellStatusesSWIFT() {
	// nolint:prealloc // we don't know how long this will be, depends on what the FXR Cells' content is
	cpf := []cellapi.CellPFDataSWIFT{}

	var failed []string

	for i, cell := range e.cellResponse {
		// no cell present
		if cell.GetCellstatus() == pb.CellStatus_CELL_STATUS_NONE_UNSPECIFIED {
			continue
		}

		status := cellapi.StatusPassed
		if cell.GetCellstatus() != pb.CellStatus_CELL_STATUS_COMPLETE {
			status = cellapi.StatusFailed
		}

		m, ok := e.Config.CellMap[e.tbc.O.String()]
		if !ok {
			e.childLogger.Error(fmt.Errorf("invalid tray position: %s", e.tbc.O.String()))
			return
		}

		if i > len(m) || len(m) == 0 {
			e.childLogger.Error(fmt.Errorf("invalid cell position index, cell list too large: %d > %d", i, len(m)))
			return
		}

		position := m[i]

		if status == cellapi.StatusFailed {
			failed = append(failed, position)
		}

		cell, ok := e.cells[position]
		if !ok {
			e.childLogger.Warn(fmt.Errorf("invalid cell position %s, unable to find cell serial", position))
			continue
		}

		psn, err := cellapi.RecipeToProcess(e.processStepName)
		if err != nil {
			e.childLogger.Warn(fmt.Errorf("invalid recipe name %s, unable to find process name", e.processStepName))
			continue
		}

		cpf = append(cpf, cellapi.CellPFDataSWIFT{
			Serial:  cell.Serial,
			Status:  status,
			Process: psn,
		})
	}

	if len(failed) > 0 {
		e.childLogger.Info(fmt.Sprintf("failed cells: %s", strings.Join(failed, ", ")))
	}

	if !e.mockCellAPI {
		if err := e.CellAPIClient.SetCellStatusesSWIFT(cpf); err != nil {
			e.childLogger.Errorw("SetCellStatuses", "error", err)
			return
		}
	} else {
		e.childLogger.Warn("cell API mocked, skipping SetCellStatuses")
	}
}

func (e *EndProcess) setCellStatuses() {
	// nolint:prealloc // we don't know how long this will be, depends on what the FXR Cells' content is
	cpf := []cellapi.CellPFData{}

	type cellStats struct {
		Serial      string `json:"cell_serial"`
		CellPFToAPI string `json:"cell_pf_to_api"`
		Status      string `json:"proto_status"`
	}

	stats := make(map[string]cellStats)

	var failed []string

	for i, cell := range e.cellResponse {
		status := cellapi.StatusPassed
		if cell.GetCellstatus() != pb.CellStatus_CELL_STATUS_COMPLETE {
			status = cellapi.StatusFailed
		}

		m, ok := e.Config.CellMap[e.tbc.O.String()]
		if !ok {
			e.childLogger.Error(fmt.Errorf("invalid tray position: %s", e.tbc.O.String()))
			return
		}

		if i >= len(m) || len(m) == 0 {
			e.childLogger.Error(fmt.Errorf("invalid cell position index, cell list too large: %d > %d", i, len(m)-1))
			return
		}

		position := m[i]

		if status == cellapi.StatusFailed {
			failed = append(failed, position)
		}

		cellInfo, ok := e.cells[position]
		if !ok || cellInfo.IsEmpty {
			// if it is empty it will be skipped here
			e.childLogger.Debug("cell position is empty", "position", position)
			continue
		}

		stats[position] = cellStats{
			Serial:      cellInfo.Serial,
			CellPFToAPI: status,
			Status:      cell.GetCellstatus().String(),
		}

		cpf = append(cpf, cellapi.CellPFData{
			Serial:  cellInfo.Serial,
			Status:  status,
			Recipe:  e.processStepName,
			Version: e.recipeVersion,
		})
	}

	statsBuf, err := json.Marshal(stats)
	if err != nil {
		statsBuf = []byte(fmt.Sprintf("%v", statsBuf))
	}

	e.childLogger.Infow("cell data collected from recipe run", "cell_data", string(statsBuf))

	if len(failed) > 0 {
		e.childLogger.Infow("failed cells", "positions", failed)
	}

	if !e.mockCellAPI {
		if err := e.CellAPIClient.SetCellStatuses(cpf); err != nil {
			e.childLogger.Errorw("SetCellStatuses", "error", err)
			return
		}
	} else {
		e.childLogger.Warn("cell API mocked, skipping SetCellStatuses")
	}
}

// waitForOpen reads from fixture until fixture position is open
func (e *EndProcess) waitForOpen() {
	for {
		msg, err := e.fxrInfo.FixtureState.GetOp()
		if err != nil {
			e.childLogger.Warnw("wait for open; get fixture operational message", "error", err)
			time.Sleep(time.Second)

			continue
		}

		if msg.GetOp().GetPosition() == pb.FixturePosition_FIXTURE_POSITION_OPEN {
			break
		}

		// allow it to update
		time.Sleep(time.Second)
	}
}

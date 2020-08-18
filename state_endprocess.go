package towercontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

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
	SubscribeChan <-chan *protostream.Message

	childLogger     *zap.SugaredLogger
	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	cells           map[string]cellapi.CellData
	cellResponse    []*pb.Cell
	processStepName string
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
			fatalError(e, e.childLogger, err)
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
		fatalError(e, e.childLogger, err)
		return
	}

	resp, err := http.Post(e.Config.Remote+_unloadEndpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		fatalError(e, e.childLogger, err)
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != 200 {
		fatalError(e, e.childLogger, fmt.Errorf("response NOT OK: %v, %v", resp.StatusCode, resp.Status))
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

	next := &Unloading{
		Config:        e.Config,
		Logger:        e.Logger,
		CellAPIClient: e.CellAPIClient,
		Publisher:     e.Publisher,
		SubscribeChan: e.SubscribeChan,
		childLogger:   e.childLogger,
		mockCellAPI:   e.mockCellAPI,
		fxbc:          e.fxbc,
		fxrInfo:       e.fxrInfo,
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
	var cpf []cellapi.CellPFDataSWIFT

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
	var cpf []cellapi.CellPFData

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

		cpf = append(cpf, cellapi.CellPFData{
			Serial:  cell.Serial,
			Status:  status,
			Recipe:  e.processStepName,
			Version: e.recipeVersion,
		})
	}

	if len(failed) > 0 {
		e.childLogger.Info(fmt.Sprintf("failed cells: %s", strings.Join(failed, ", ")))
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

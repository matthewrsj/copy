package towercontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
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

	tbc             traycontrollers.TrayBarcode
	fxbc            traycontrollers.FixtureBarcode
	cells           map[string]cellapi.CellData
	cellResponse    []*pb.Cell
	processStepName string
	fixtureFault    bool
	manual          bool
	mockCellAPI     bool
}

func (e *EndProcess) action() {
	if !e.mockCellAPI {
		if err := e.CellAPIClient.UpdateProcessStatus(e.tbc.SN, e.processStepName, cellapi.StatusEnd); err != nil {
			// keep trying the other transactions
			e.Logger.Warn(err)
		}
	} else {
		e.Logger.Warn("cell API mocked, skipping UpdateProcessStatus")
	}

	// nolint:prealloc // we don't know how long this will be, depends on what the FXR Cells' content is
	var cpf []cellapi.CellPFData

	var failed []string

	for i, cell := range e.cellResponse {
		// no cell present
		if cell.GetCellstatus() == pb.CellStatus_CELL_STATUS_NONE_UNSPECIFIED {
			continue
		}

		status := "pass"
		if cell.GetCellstatus() != pb.CellStatus_CELL_STATUS_COMPLETE {
			status = "fail"
		}

		// TODO: making assumption we get all cells back but only report the ones that are not empty. Confirm this.

		m, ok := e.Config.CellMap[e.tbc.O.String()]
		if !ok {
			err := fmt.Errorf("invalid tray position: %s", e.tbc.O.String())
			e.Logger.Warn(err)
			log.Println("WARNING:", err)

			return
		}

		if i > len(m) || len(m) == 0 {
			err := fmt.Errorf("invalid cell position index, cell list too large: %d > %d", i, len(m))
			e.Logger.Warn(err)
			log.Println("WARNING:", err)

			// i will only increase, don't keep trying
			return
		}

		position := m[i]

		if status == "fail" {
			failed = append(failed, position)
		}

		cell, ok := e.cells[position]
		if !ok {
			err := fmt.Errorf("invalid cell position %s, unable to find cell serial", position)
			e.Logger.Warn(err)
			log.Println("WARNING:", err)

			continue
		}

		cpf = append(cpf, cellapi.CellPFData{
			Serial:  cell.Serial,
			Process: e.processStepName,
			Status:  status,
		})
	}

	if len(failed) > 0 {
		e.Logger.Info(fmt.Sprintf("failed cells: %s", strings.Join(failed, ", ")))
	}

	if !e.mockCellAPI {
		if err := e.CellAPIClient.SetCellStatuses(cpf); err != nil {
			e.Logger.Errorw("SetCellStatuses", "error", err)
			return
		}
	} else {
		e.Logger.Warn("cell API mocked, skipping SetCellStatuses")
	}

	// TODO: determine how to inform cell API of fault
	msg := "tray complete"
	if e.fixtureFault {
		msg += "; fixture faulted"
	}

	e.Logger.Info(msg)

	// if this is manual we are done
	if e.manual {
		e.Logger.Info("done with tray")
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
		fatalError(e, e.Logger, err)
		return
	}

	resp, err := http.Post(e.Config.Remote+_unloadEndpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		fatalError(e, e.Logger, err)
		return
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != 200 {
		fatalError(e, e.Logger, fmt.Errorf("response NOT OK: %v, %v", resp.StatusCode, resp.Status))
		return
	}

	e.Logger.Info("done with tray")
}

// Actions returns the action functions for this state
func (e *EndProcess) Actions() []func() {
	e.SetLast(true)

	return []func(){
		e.action,
	}
}

// Next returns the next state to run
func (e *EndProcess) Next() statemachine.State {
	e.Logger.Debug("statemachine exiting")
	return nil
}

type trayComplete struct {
	ID     string `json:"id"`
	Aisle  string `json:"aisle"`
	Column string `json:"column"`
	Level  string `json:"level"`
}

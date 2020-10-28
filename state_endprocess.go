package towercontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cdcontroller"
	"stash.teslamotors.com/rr/protostream"
	tower "stash.teslamotors.com/rr/towerproto"
)

const _unloadEndpoint = "/unload"

// EndProcess informs the cell API of process completion. This is the last state.
type EndProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *zap.SugaredLogger
	CellAPIClient *cdcontroller.CellAPIClient
	Publisher     *protostream.Socket

	childLogger     *zap.SugaredLogger
	tbc             cdcontroller.TrayBarcode
	fxbc            cdcontroller.FixtureBarcode
	cells           map[string]cdcontroller.CellData
	cellResponse    []*tower.Cell
	processStepName string
	smFatal         bool
	fixtureFault    bool
	mockCellAPI     bool
	skipClose       bool
	recipeVersion   int

	fxrInfo *FixtureInfo
}

// nolint:gocognit // TODO: simplify reporting logic. Need to work with cell api team to eliminate the two-step logic
func (e *EndProcess) action() {
	if len(e.cells) == 0 { // we short-circuited here or something went wrong, just re-get the map
		e.childLogger.Info("empty cell map, querying API for new map")

		var err error
		if e.cells, err = getCellMap(e.mockCellAPI, e.childLogger, e.CellAPIClient, e.tbc.SN); err != nil {
			e.childLogger.Errorw("get cell map", "error", err)
			e.smFatal = true

			return
		}
	}

	if !e.fixtureFault {
		if e.skipClose {
			e.childLogger.Info("skipClose set, not closing process step or setting cell statuses")
		}

		if !e.skipClose && e.processStepName != cdcontroller.CommissionSelfTestRecipeName {
			e.childLogger.Info("setting cell statuses")
			e.setCellStatuses()
		}

		if !e.mockCellAPI {
			// out of band and ignoring all errors update Cell API that we finished
			// does not affect any process just makes it easier to find data
			// this is different from ending it with the process name as it just leaves a marker on the fixture itself instead
			// of closing the actual process step.
			go func() {
				e.childLogger.Debug("updating process status", "status", "end")

				err := e.CellAPIClient.UpdateProcessStatus(e.tbc.SN, fmt.Sprintf("CM2-%s%s-%s", e.Config.Loc.Process, e.Config.Loc.Aisle, e.fxrInfo.Name), cdcontroller.StatusEnd)
				if err != nil {
					e.childLogger.Warnw("unable to update Cell API of recipe end", "error", err)
				}
			}()

			if !e.skipClose && e.processStepName != cdcontroller.CommissionSelfTestRecipeName {
				e.childLogger.Infow("closing process step", "recipe_name", e.processStepName, "recipe_version", e.recipeVersion)

				if err := e.CellAPIClient.CloseProcessStep(e.tbc.SN, e.processStepName, e.recipeVersion); err != nil {
					e.childLogger.Error("close process status", "error", err)
				}
			} else {
				e.childLogger.Info("not closing process step for recipe", "recipe_name", e.processStepName)
			}
		} else {
			e.childLogger.Warn("Cell API mocked, not closing process step")
		}
	}

	msg := "tray complete"

	if e.fixtureFault {
		if err := logFaultToFile(msg, e.fxrInfo.Name, "logs/towercontroller/faults.log"); err != nil {
			e.childLogger.Errorw("log fault to file", "error", err)
		}
	}

	e.childLogger.Infow(msg, "fixture_faulted", e.fixtureFault)

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
	return []func(){
		e.action,
	}
}

// Next returns the next state to run
func (e *EndProcess) Next() statemachine.State {
	var next statemachine.State

	switch {
	case e.smFatal:
		next = &Idle{
			Config:        e.Config,
			Logger:        e.Logger,
			CellAPIClient: e.CellAPIClient,
			Publisher:     e.Publisher,
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

func (e *EndProcess) setCellStatuses() {
	// nolint:prealloc // we don't know how long this will be, depends on what the FXR Cells' content is
	cpf := []cdcontroller.CellPFData{}

	type cellStats struct {
		Serial      string `json:"cell_serial"`
		CellPFToAPI string `json:"cell_pf_to_api"`
		Status      string `json:"proto_status"`
	}

	stats := make(map[string]cellStats)

	var failed []string

	for i, cell := range e.cellResponse {
		status := cdcontroller.StatusPassed
		if cell.GetStatus() != tower.CellStatus_CELL_STATUS_COMPLETE {
			status = cdcontroller.StatusFailed
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

		if status == cdcontroller.StatusFailed {
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
			Status:      cell.GetStatus().String(),
		}

		cpf = append(cpf, cdcontroller.CellPFData{
			Serial: cellInfo.Serial,
			Status: status,
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
		if err := e.CellAPIClient.SetCellStatuses(e.tbc.SN, e.fxbc.Raw, e.processStepName, e.recipeVersion, cpf); err != nil {
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

		if msg.GetOp().GetPosition() == tower.FixturePosition_FIXTURE_POSITION_OPEN {
			break
		}

		// allow it to update
		time.Sleep(time.Second)
	}
}

func logFaultToFile(msg, fxName, fPath string) error {
	msg += fmt.Sprintf("; fixture %s faulted; time: %v\n", fxName, time.Now())

	f, err := os.OpenFile(fPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return fmt.Errorf("open fault log: %v", err)
	}

	defer func() {
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}()

	if _, err = f.Write([]byte(msg)); err != nil {
		return fmt.Errorf("write to fault log: %v", err)
	}

	return err
}

package towercontroller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/cenkalti/backoff"
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

func (e *EndProcess) action() {
	if e.recipeVersion == 0 { // we short-circuited here or something went wrong, just re-get the version
		e.childLogger.Info("version is 0, querying API for new recipe version")

		e.processStepName, e.recipeVersion = getRecipeAndVersion(e.mockCellAPI, e.childLogger, e.CellAPIClient, e.tbc.SN)
	}

	if len(e.cells) == 0 { // we short-circuited here or something went wrong, just re-get the map
		e.childLogger.Info("empty cell map, querying API for new map")

		bo := backoff.NewExponentialBackOff()
		bo.MaxInterval = time.Minute
		bo.MaxElapsedTime = 0 // try forever

		_ = backoff.Retry(func() error {
			var err error
			if e.cells, err = getCellMap(e.mockCellAPI, e.childLogger, e.CellAPIClient, e.tbc.SN, e.processStepName); err != nil {
				e.childLogger.Errorw("get cell map", "error", err)

				return err
			}

			return nil
		}, bo)
	}

	if !e.skipClose {
		e.childLogger.Info("setting cell statuses")
		e.setCellStatuses()
	} else {
		// skipClose at this point is a bit of a misnomer, but essentially means we were unable to even start a recipe
		// on these cells either due to internal error or fixture not being responsive. In this case we have no cell
		// statuses to even post.
		e.childLogger.Info("skipClose set or commissioning tray, not setting cell statuses")
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

	// use mockCellAPI as a flag to indicate we don't want to do network things
	if !e.mockCellAPI {
		e.childLogger.Info("sending unload request")

		bo := backoff.NewExponentialBackOff()
		bo.MaxInterval = time.Minute
		bo.MaxElapsedTime = 0 // try forever

		// will never return a backoff.PermanentError (tries forever)
		_ = backoff.Retry(func() error {
			resp, err := http.Post(e.Config.Remote+_unloadEndpoint, "application/json", bytes.NewReader(b))
			if err != nil {
				e.childLogger.Errorw("post unload request", "error", err)
				return err
			}

			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode != 200 {
				e.childLogger.Errorw("http post", "response", fmt.Errorf("response NOT OK: %v, %v", resp.StatusCode, resp.Status))
				return fmt.Errorf("unload request response %d ('%s'), expected 200", resp.StatusCode, resp.Status)
			}

			return nil
		}, bo)
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
	cpf := make(map[string]string)

	type cellStats struct {
		Serial string `json:"cell_serial"`
		Status string `json:"proto_status"`
	}

	stats := make(map[string]cellStats)

	var failed []string

	for i, cell := range e.cellResponse {
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

		cellInfo, ok := e.cells[position]
		if !ok || cellInfo.IsEmpty {
			// if it is empty it will be skipped here
			e.childLogger.Debug("cell position is empty", "position", position)
			continue
		}

		if cell.GetStatus() != tower.CellStatus_CELL_STATUS_COMPLETE {
			failed = append(failed, position)
		}

		stats[position] = cellStats{
			Serial: cellInfo.Serial,
			Status: cell.GetStatus().String(),
		}

		cpf[position] = cell.GetStatus().String()
	}

	statsBuf, err := json.Marshal(stats)
	if err != nil {
		statsBuf = []byte(fmt.Sprintf("%v", statsBuf))
	}

	e.childLogger.Infow("cell data collected from recipe run", "cell_data", string(statsBuf))

	if len(failed) > 0 {
		e.childLogger.Infow("failed cells", "positions", failed)
	}

	statusSetter := e.CellAPIClient.SetCellStatuses
	if e.fixtureFault || isCommissionRecipe(e.processStepName) {
		statusSetter = e.CellAPIClient.SetCellStatusesNoClose
	}

	if !e.mockCellAPI {
		bo := backoff.NewExponentialBackOff()
		bo.MaxInterval = time.Minute
		bo.MaxElapsedTime = 0 // try forever

		// will never return a backoff.PermanentError (tries forever)
		_ = backoff.Retry(func() error {
			if err := statusSetter(e.tbc.SN, e.fxbc.Raw, e.processStepName, e.recipeVersion, cpf); err != nil {
				e.childLogger.Errorw("SetCellStatuses", "error", err)
				return err
			}

			return nil
		}, bo)
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

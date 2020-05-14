package towercontroller

import (
	"fmt"
	"log"
	"strings"

	"github.com/sirupsen/logrus"
	"stash.teslamotors.com/ctet/statemachine/v2"
	"stash.teslamotors.com/rr/cellapi"
	pb "stash.teslamotors.com/rr/towercontroller/pb"
)

type EndProcess struct {
	statemachine.Common

	Config        Configuration
	Logger        *logrus.Logger
	CellAPIClient *cellapi.Client

	tbc             TrayBarcode
	fxbc            FixtureBarcode
	cells           map[string]cellapi.CellData
	cellResponse    []*pb.Cell
	processStepName string
	fixtureFault    bool
}

// error handling lengthens action
// nolint: funlen
func (e *EndProcess) action() {
	if err := e.CellAPIClient.UpdateProcessStatus(e.tbc.SN, e.processStepName, cellapi.StatusEnd); err != nil {
		// keep trying the other transactions
		e.Logger.Error(err)
		log.Println(err)
	}

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

		if i > len(m) {
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

	failMsg := fmt.Sprintf("failed cells: %s", strings.Join(failed, ", "))
	e.Logger.WithFields(logrus.Fields{
		"tray":    e.tbc.raw,
		"fixture": e.fxbc.raw,
	}).Infof(failMsg)

	log.Printf("tray %s (fixture %s) %s", e.tbc.SN, e.fxbc.raw, failMsg)

	if err := e.CellAPIClient.SetCellStatuses(cpf); err != nil {
		e.Logger.Error(err)
		log.Println(err)

		return
	}
	// TODO: determine how to inform cell API of fault
	msg := "tray complete"
	if e.fixtureFault {
		msg += "; fixture faulted"
	}

	e.Logger.WithFields(logrus.Fields{
		"tray":    e.tbc.raw,
		"fixture": e.fxbc.raw,
	}).Infof(msg)
}

func (e *EndProcess) Actions() []func() {
	e.SetLast(true)

	return []func(){
		e.action,
	}
}

func (e *EndProcess) Next() statemachine.State {
	e.Logger.WithField("tray", e.tbc.SN).Trace("statemachine exiting")
	return nil
}

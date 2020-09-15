//+build !test

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	asrsapi "stash.teslamotors.com/cas/asrs/idl/src"
)

type intermediateLoad struct {
	Tray     *asrsapi.Tray                        `json:"tray"`
	State    *asrsapi.LoadOperationStateAndStatus `json:"state"`
	Location *imLocation                          `json:"location"`
	Recipe   *asrsapi.Recipe                      `json:"recipe"`
}

type intermediateUnload struct {
	Tray     *asrsapi.Tray                          `json:"tray"`
	State    *asrsapi.UnloadOperationStateAndStatus `json:"state"`
	Location *imLocation                            `json:"location"`
}

type imLocation struct {
	CmFormat *cmLocation `json:"cmFormat"`
}

type cmLocation struct {
	EquipmentID         string `json:"equipmentId"`
	ManufacturingSystem string `json:"manufacturingSystem"`
	WorkCenter          string `json:"workcenter"`
	Equipment           string `json:"equipment"`
	WorkStation         string `json:"workstation"`
	SubIdentifier       string `json:"subIdentifier"`
}

// nolint:funlen // just a script
func main() {
	operation := flag.String("op", "preparedForDelivery", "operation to run [preparedForDelivery|loaded|preparedToUnload]")
	trays := flag.String("trays", "11223344A,11223345A", "trays to operate on")
	col := flag.String("column", "01", "column location in aisle")
	lvl := flag.String("lvl", "01", "level in column")

	flag.Parse()

	sc := []byte(`
{
	"STEP00": {
		"mode": "FORM_REQ_CC",
		"charge_current": 8.67,
		"max_current": 9.0,
		"cutoff_voltage": 4.1,
		"cutoff_current": 0.0,
		"cutoff_dv": 0.0,
		"charge_power": 4.5,
		"cutoff_ah": 0.0,
		"ending_style": "ENDING_STYLE_CELL_BYPASS_ENABLE",
		"v_cell_min_quality": 0.1,
		"v_cell_max_quality": 4.0,
		"step_timeout": 10800
	},
	"STEP01": {
		"mode": "FORM_REQ_CV",
		"charge_current": 8.7,
		"max_current": 9.0,
		"cutoff_voltage": 4.1,
		"cutoff_current": 0.0,
		"cutoff_dv": 0.0,
		"charge_power": 4.5,
		"cutoff_ah": 0.0,
		"ending_style": "ENDING_STYLE_CELL_BYPASS_ENABLE",
		"v_cell_min_quality": 0.1,
		"v_cell_max_quality": 4.0,
		"step_timeout": 10800
	},
	"STEP02": {
		"mode": "FORM_REQ_CC",
		"charge_current": -8.67,
		"max_current": 8.67,
		"cutoff_voltage": 4.1,
		"cutoff_current": 0.0,
		"cutoff_dv": 0.0,
		"charge_power": 4.5,
		"cutoff_ah": 0.0,
		"ending_style": "ENDING_STYLE_CELL_BYPASS_ENABLE",
		"v_cell_min_quality": 0.1,
		"v_cell_max_quality": 4.0,
		"step_timeout": 9000
	}
}`)

	trayIDs := strings.Split(*trays, ",")

	tray := &asrsapi.Tray{
		TrayId: trayIDs,
	}
	loadState := &asrsapi.LoadOperationStateAndStatus{
		State:     asrsapi.LoadOperationState_PreparedForDelivery,
		StateType: asrsapi.StateType_Desired,
		Status: &asrsapi.Status{
			Status: asrsapi.Status_Complete,
		},
	}
	unloadState := &asrsapi.UnloadOperationStateAndStatus{
		State:     asrsapi.UnloadOperationState_PreparedToUnload,
		StateType: asrsapi.StateType_Desired,
		Status: &asrsapi.Status{
			Status: asrsapi.Status_Complete,
		},
	}
	location := &imLocation{
		CmFormat: &cmLocation{
			EquipmentID:         "CM2-63010-00-00",
			ManufacturingSystem: "CM2",
			WorkCenter:          "63",
			Equipment:           "010",
			WorkStation:         "00",
			SubIdentifier:       "00",
		},
	}
	recipe := &asrsapi.Recipe{
		Name:              "CBuildRecipe",
		Step:              "form_precharge - 1",
		StepType:          "CMChargeDischarge",
		StepConfiguration: sc,
	}

	var (
		jb       []byte
		endPoint string
		err      error
	)

	switch *operation {
	case "preparedToUnload":
		actual := &intermediateUnload{
			Tray:     tray,
			State:    unloadState,
			Location: location,
		}

		actual.Location.CmFormat.WorkStation = *col
		actual.Location.CmFormat.SubIdentifier = *lvl
		actual.Location.CmFormat.EquipmentID = fmt.Sprintf("CM2-63010-%s-%s", *col, *lvl)

		jb, err = json.Marshal(actual)
		if err != nil {
			log.Fatal(err)
		}

		endPoint = "/unloadOperations"
	case "preparedForDelivery":
		actual := &intermediateLoad{
			Tray:     tray,
			State:    loadState,
			Location: location,
			Recipe:   recipe,
		}

		actual.State.State = asrsapi.LoadOperationState_PreparedForDelivery
		actual.State.StateType = asrsapi.StateType_Desired

		jb, err = json.Marshal(actual)
		if err != nil {
			log.Fatal(err)
		}

		endPoint = "/loadOperations"
	case "loaded":
		actual := &intermediateLoad{
			Tray:     tray,
			State:    loadState,
			Location: location,
			Recipe:   recipe,
		}

		actual.State.State = asrsapi.LoadOperationState_Loaded
		actual.State.StateType = asrsapi.StateType_Current
		actual.Location.CmFormat.WorkStation = *col
		actual.Location.CmFormat.SubIdentifier = *lvl
		actual.Location.CmFormat.EquipmentID = fmt.Sprintf("CM2-63010-%s-%s", *col, *lvl)

		jb, err = json.Marshal(actual)
		if err != nil {
			log.Fatal(err)
		}

		endPoint = "/loadOperations"
	default:
		log.Fatal("invalid operation")
	}

	resp, err := http.Post("http://localhost:13174/asrs"+endPoint, "application/json", bytes.NewReader(jb))
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		log.Printf("bad status %d %s", resp.StatusCode, resp.Status)
	}
}

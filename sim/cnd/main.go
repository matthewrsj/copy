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
		"cut_off_voltage": 4.1,
		"cut_off_current": 0.0,
		"cell_drop_out_v": 0.0,
		"step_timeout": 10800
	},
	"STEP01": {
		"mode": "FORM_REQ_CV",
		"charge_current": 8.7,
		"max_current": 9.0,
		"cut_off_voltage": 4.1,
		"cut_off_current": 1.3,
		"cell_drop_out_v": 0.0,
		"step_timeout": 10800
	},
	"STEP02": {
		"mode": "FORM_REQ_CC",
		"charge_current": -8.67,
		"max_current": 8.67,
		"cut_off_voltage": 3.3,
		"cut_off_current": 0.0,
		"cell_drop_out_v": 0.0,
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

	resp, err := http.Post("http://localhost:13173/asrs"+endPoint, "application/json", bytes.NewReader(jb))
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

package traycontrollers

import (
	"encoding/json"
	"sort"

	pb "stash.teslamotors.com/rr/towerproto"
)

// Step defines an individual recipe step
type Step struct {
	Mode               string                    `json:"mode"`
	ChargeCurrentAmps  float32                   `json:"charge_current"`
	MaxCurrentAmps     float32                   `json:"max_current"` // amps limited to this value charge/discharge
	CutOffVoltage      float32                   `json:"cutoff_voltage"`
	CutOffCurrent      float32                   `json:"cutoff_current"`
	CutOffDV           float32                   `json:"cutoff_dv"`
	ChargePower        float32                   `json:"charge_power"`
	CutOffAH           float32                   `json:"cutoff_ah"`
	EndingStyle        pb.RecipeStep_EndingStyle `json:"ending_style"`
	VCellMinQuality    float32                   `json:"v_cell_min_quality"`
	VCellMaxQuality    float32                   `json:"v_cell_max_quality"`
	StepTimeoutSeconds float32                   `json:"step_timeout"`
}

/*
StepConfiguration from CND (cell api) looks like this, a map with the keys increasing from STEP00 onward

TC/FXRs need a slice of steps in step order, so drop the keys and just make a slice here
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
		"ending_style: 1,
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
		"ending_style: 1,
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
		"ending_style: 1,
		"v_cell_min_quality": 0.1,
		"v_cell_max_quality": 4.0,
		"step_timeout": 9000
	}
}
*/
// StepConfiguration is the slice of steps that defines a recipe
type StepConfiguration []Step

// NewStepConfiguration parses a new StepConfiguration out of a byte slice
func NewStepConfiguration(steps []byte) (StepConfiguration, error) {
	scm := make(map[string]Step)
	if err := json.Unmarshal(steps, &scm); err != nil {
		return nil, err
	}

	keys := make([]string, len(scm))

	var i int

	for k := range scm {
		keys[i] = k
		i++
	}

	sort.Strings(keys)

	sc := make(StepConfiguration, len(keys))
	for i, key := range keys {
		sc[i] = scm[key]
	}

	return sc, nil
}

package cdcontroller

import (
	"sort"

	tower "stash.teslamotors.com/rr/towerproto"
)

// StepConfiguration contains a recipe list
type StepConfiguration struct {
	RecipeList   map[string]Step       `json:"recipe_list"`
	StepOrdering []uint32              `json:"step_ordering"`
	ParamLims    []ParameterizedLimits `json:"parameterized_limits"`
}

// Step defines an individual recipe step
type Step struct {
	Name   string      `json:"name"`
	Values Ingredients `json:"values"`
}

// Ingredients contains the various parameters for a recipe step
type Ingredients struct {
	Mode               string  `json:"mode"`
	EndingStyle        string  `json:"ending_style"`
	ChargeCurrentAmps  float32 `json:"charge_current"`
	MaxCurrentAmps     float32 `json:"max_current"` // amps limited to this value charge/discharge
	CutOffVoltage      float32 `json:"cutoff_voltage"`
	CutOffCurrent      float32 `json:"cutoff_current"`
	CutOffDV           float32 `json:"cutoff_dv"`
	ChargePower        float32 `json:"charge_power"`
	CutOffAH           float32 `json:"cutoff_ah"`
	VCellMinQuality    float32 `json:"v_cell_min_quality"`
	VCellMaxQuality    float32 `json:"v_cell_max_quality"`
	StepTimeoutSeconds float32 `json:"step_timeout"`
}

/*
FormationStep from cell api looks like this
{
    "name": "swift_baseline_14 - 1",
    "step": "swift_quality_cycling - 1",
    "step_type": "cm_cd",
    "step_configuration": {
        "recipe_list": {
            "1": {
                "name": "swift_cycle_charge_cc/1",
                "values": {
                    "charge_current": 10.0,
                    "cutoff_ah": 0.0,
                    "cutoff_current": 0.0,
                    "cutoff_dv": 0.01,
                    "cutoff_voltage": 4.1,
                    "ending_style": "ENDING_STYLE_CELL_BYPASS_ENABLE",
                    "max_current": 10.5,
                    "mode": "FORM_MODE_CC",
                    "step_timeout": 9000,
                    "v_cell_max_quality": 4.2,
                    "v_cell_min_quality": 2.85
                }
            },
            "2": {
                "name": "swift_cycle_charge_cv/1",
                "values": {
                    "charge_current": 10.0,
                    "cutoff_ah": 0.0,
                    "cutoff_current": 1.3,
                    "cutoff_dv": 0.01,
                    "cutoff_voltage": 4.1,
                    "ending_style": "ENDING_STYLE_CELL_BYPASS_ENABLE",
                    "max_current": 10.5,
                    "mode": "FORM_MODE_CV",
                    "step_timeout": 10800,
                    "v_cell_max_quality": 4.21,
                    "v_cell_min_quality": 3.8
                }
            },
            "3": {
                "name": "swift_cycle_wait_charge/1",
                "values": {
                    "charge_current": 0.0,
                    "cutoff_ah": 0.0,
                    "cutoff_current": 0.0,
                    "cutoff_dv": 0.0,
                    "cutoff_voltage": 0.0,
                    "ending_style": "ENDING_STYLE_UNKNOWN_UNSPECIFIED",
                    "max_current": 0.5,
                    "mode": "FORM_MODE_DELAY",
                    "step_timeout": 900,
                    "v_cell_max_quality": 4.21,
                    "v_cell_min_quality": -0.1
                }
            }
        },
        "step_ordering": [1, 2, 3, 2, 1]
    }
}
*/
// FormationStep is the slice of steps that defines a recipe
type FormationStep struct {
	Name        string            `json:"name"`
	Step        string            `json:"step"`
	StepType    string            `json:"step_type"`
	StepConfMap StepConfiguration `json:"step_configuration"`
}

// StepList contains a list of steps to be sent to the firmware
type StepList struct {
	Steps        []Ingredients
	StepOrdering []uint32
	ParamLims    []*tower.ParameterizedLimit
}

// ParameterizedLimits contains configurable limits per recipe step
type ParameterizedLimits struct {
	StepNumber       uint32  `json:"step_number"`
	LimitType        string  `json:"limit_type"`
	LimitActive      string  `json:"limit_active"`
	LimitLowerBound  float32 `json:"limit_lower_bound"`
	LimitUpperBound  float32 `json:"limit_upper_bound"`
	CellStatusResult string  `json:"cell_status_result"`
}

// NewStepList parses a new FormationStep out of a byte slice
func NewStepList(sc FormationStep) (StepList, error) {
	var sl StepList

	keys := make([]string, len(sc.StepConfMap.RecipeList))

	var i int

	for k := range sc.StepConfMap.RecipeList {
		keys[i] = k
		i++
	}

	sort.Strings(keys)

	sl.Steps = make([]Ingredients, len(keys))

	for i, key := range keys {
		sl.Steps[i] = sc.StepConfMap.RecipeList[key].Values
	}

	sl.StepOrdering = sc.StepConfMap.StepOrdering

	sl.ParamLims = make([]*tower.ParameterizedLimit, len(sc.StepConfMap.ParamLims))
	for i, lims := range sc.StepConfMap.ParamLims {
		sl.ParamLims[i] = &tower.ParameterizedLimit{
			StepNumber:       lims.StepNumber,
			LimitType:        tower.ParamLimitType(tower.ParamLimitType_value[lims.LimitType]),
			LimitActive:      tower.ParamLimitActive(tower.ParamLimitActive_value[lims.LimitActive]),
			CellStatusResult: tower.CellStatus(tower.CellStatus_value[lims.CellStatusResult]),
			LimitUpperBound:  lims.LimitUpperBound,
			LimitLowerBound:  lims.LimitLowerBound,
		}
	}

	return sl, nil
}

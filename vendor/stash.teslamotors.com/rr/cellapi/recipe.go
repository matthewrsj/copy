package cellapi

import "fmt"

func recipeToProcess(step string) (string, error) {
	rps := map[string]string{
		"FORM_PRECHARGE":                "precharge",
		"FORM_PRECHARGE_SKIP_CHECK":     "precharge",
		"FORM_FIRST_CHARGE":             "first_charge",
		"FORM_FIRST_CHARGE_SKIP_CHECK":  "first_charge",
		"FORM_SECOND_CHARGE":            "final_cd",
		"FORM_SECOND_CHARGE_SKIP_CHECK": "final_cd",
		"FORM_DISCHARGE":                "final_cd",
		"FORM_CYCLE":                    "quality_cycling",
		"FORM_CYCLE_MOD":                "quality_cycling",
		"DOE_FORM_PRECHARGE":            "precharge",
		"DOE_FORM_FIRST_CHARGE":         "first_charge",
		"DOE_FORM_SECOND_CHARGE":        "final_cd",
	}

	process, ok := rps[step]
	if !ok {
		return process, fmt.Errorf("step %s is not valid", step)
	}

	return process, nil
}

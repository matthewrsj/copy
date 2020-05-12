package towercontroller

import (
	"fmt"
	"io/ioutil"

	"gopkg.in/yaml.v2"
	pb "stash.teslamotors.com/rr/towercontroller/pb"
)

/*
ingredients contains all the parameters for a specific step in a recipe

	PRECHARGE:
	  mode: FORM_REQ_CC
	  charge_current: 2.6  # Amps
	  max_current: 3       # Amps limited to this value chg/dchg
	  cut_off_voltage: 3.2 # Voltage where cells are dropped out
	  cut_off_current: 0   # NA
	  cell_drop_out_v: 0   # NA
	  step_timeout: 3600   # 1 hour - Seconds before step timeout
*/
type ingredients struct {
	Mode               string  `yaml:"mode"`
	ChargeCurrentAmps  float32 `yaml:"charge_current"`
	MaxCurrentAmps     float32 `yaml:"max_current"` // amps limited to this value charge/discharge
	CutOffVoltage      float32 `yaml:"cut_off_voltage"`
	CutOffCurrent      float32 `yaml:"cut_off_current"`
	CellDropOutVoltage float32 `yaml:"cell_drop_out_v"`
	StepTimeoutSeconds float32 `yaml:"step_timeout"`
}

type cookbook map[string][]ingredients
type ingredientsbook map[string]ingredients
type stepsbook map[string][]string

func loadIngredients(path string) (ingredientsbook, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read recipe file %s: %v", path, err)
	}

	ingredientsBook := make(ingredientsbook)

	if err := yaml.Unmarshal(content, ingredientsBook); err != nil {
		return nil, fmt.Errorf("unmarshal ingredient yaml: %v", err)
	}

	return ingredientsBook, nil
}

func loadRecipes(recipePath, ingredientsPath string) (cookbook, error) {
	content, err := ioutil.ReadFile(recipePath)
	if err != nil {
		return nil, fmt.Errorf("read recipe file %s: %v", recipePath, err)
	}

	// stepsBook contains the list of ingredient names in the order they should
	// be performed by the recipe
	stepsBook := make(stepsbook)

	if err = yaml.Unmarshal(content, stepsBook); err != nil {
		return nil, fmt.Errorf("unmarshal steps yaml: %v", err)
	}

	// ingredientsBook contains the actual parameters for each step
	ingredientsBook, err := loadIngredients(ingredientsPath)
	if err != nil {
		return nil, fmt.Errorf("load ingredients from %s: %v", ingredientsPath, err)
	}

	// cookBook contains all the recipes mapped to their respective lists of ingredient
	// steps to run.
	cookBook := make(cookbook)

	for name, steps := range stepsBook {
		for _, step := range steps {
			i, ok := ingredientsBook[step]
			if !ok {
				return nil, fmt.Errorf("ingredients file %s did not contain ingredients for step %s", ingredientsPath, step)
			}

			// add the ingredients for this step to the list of steps in the cookbook
			cookBook[name] = append(cookBook[name], i)
		}
	}

	return cookBook, nil
}

func loadRecipe(recipePath, ingredientsPath, recipe string) ([]ingredients, error) {
	cookBook, err := loadRecipes(recipePath, ingredientsPath)
	if err != nil {
		return nil, fmt.Errorf("load recipes from %s and %s: %v", recipePath, ingredientsPath, err)
	}

	ings, ok := cookBook[recipe]
	if !ok {
		return nil, fmt.Errorf("recipe files at %s and %s did not contain %s", recipePath, ingredientsPath, recipe)
	}

	return ings, nil
}

func modeStringToEnum(input string) pb.RecipeStep_FormMode {
	switch input {
	case "FORM_REQ_CC":
		return pb.RecipeStep_FORM_MODE_CC
	case "FORM_REQ_CV":
		return pb.RecipeStep_FORM_MODE_CV
	default:
		return pb.RecipeStep_FORM_MODE_UNKNOWN_UNSPECIFIED
	}
}

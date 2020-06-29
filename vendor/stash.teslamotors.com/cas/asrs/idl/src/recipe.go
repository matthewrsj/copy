package asrs

import (
	"encoding/json"
	"errors"
)

// We do this to make it possible for the user sitting in conductor to pass in one json string,
// while supporting the ability in asrs to carry more than just json if we we wish to.
type JsonEncodedRecipe struct {
	Name              string          `json:"name,omitempty"`
	Step              string          `json:"step,omitempty"`
	StepType          string          `json:"step_type,omitempty"`
	StepConfiguration json.RawMessage `json:"step_configuration,omitempty"`
}

func RecipeToString(recipe *Recipe) (string, error) {
	if recipe == nil {
		return "", errors.New("empty Recipe is invalid")
	}

	// We go through intermediate structure to control marshalling of step_configuration
	// rather than through jsonpb.
	jr := &JsonEncodedRecipe{
		Name:              recipe.Name,
		Step:              recipe.Step,
		StepType:          recipe.StepType,
		StepConfiguration: recipe.StepConfiguration,
	}

	out, err := json.Marshal(jr)
	return string(out), err
}

func RecipeFromString(in string) (*Recipe, error) {
	var jr JsonEncodedRecipe
	err := json.Unmarshal([]byte(in), &jr)
	if err != nil {
		return nil, err
	}

	// We go through intermediate structure to control unmarshalling of step_configuration
	// rather than through jsonpb.
	if jr.Name != "" {
		return &Recipe{
			Name:              jr.Name,
			Step:              jr.Step,
			StepType:          jr.StepType,
			StepConfiguration: jr.StepConfiguration,
		}, nil
	}

	return nil, nil
}

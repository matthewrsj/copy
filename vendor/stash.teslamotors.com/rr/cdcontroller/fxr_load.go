package cdcontroller

// FXRLoad is used to post to the TC that a tray is loaded
type FXRLoad struct {
	TransactionID string `json:"transaction_id"`
	Column        int    `json:"column"`
	Level         int    `json:"level"`
	TrayID        string `json:"tray"`
	RecipeName    string `json:"recipe_name"`
	RecipeVersion int    `json:"recipe_ver"`
	StepType      string `json:"step_type"`
}

// AllowedStepType is the step type allowed for charge/discharge
const AllowedStepType = "cm_cd"

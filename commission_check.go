package towercontroller

import (
	"strings"

	"stash.teslamotors.com/rr/cdcontroller"
)

func isCommissionRecipe(stepName string) bool {
	return strings.Contains(strings.ToLower(stepName), cdcontroller.CommissionSelfTestRecipeName)
}

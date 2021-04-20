package towercontroller

import (
	"testing"

	"github.com/stretchr/testify/assert"
	tower "stash.teslamotors.com/rr/towerproto"
)

func Test_modeStringToEnum(t *testing.T) {
	testCases := []struct {
		in  string
		exp tower.RecipeStep_FormMode
	}{
		{"FORM_MODE_CC", tower.RecipeStep_FORM_MODE_CC},
		{"FORM_MODE_CV", tower.RecipeStep_FORM_MODE_CV},
		{"FORM_MODE", tower.RecipeStep_FORM_MODE_UNKNOWN_UNSPECIFIED},
		{"", tower.RecipeStep_FORM_MODE_UNKNOWN_UNSPECIFIED},
	}

	for _, tc := range testCases {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.exp, modeStringToEnum(tc.in))
		})
	}
}

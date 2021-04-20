package towercontroller

import tower "stash.teslamotors.com/rr/towerproto"

func modeStringToEnum(input string) tower.RecipeStep_FormMode {
	fm, ok := tower.RecipeStep_FormMode_value[input]
	if !ok {
		return tower.RecipeStep_FORM_MODE_UNKNOWN_UNSPECIFIED
	}

	return tower.RecipeStep_FormMode(fm)
}

func endingStyleStringToEnum(input string) tower.RecipeStep_EndingStyle {
	fm, ok := tower.RecipeStep_EndingStyle_value[input]
	if !ok {
		return tower.RecipeStep_ENDING_STYLE_UNKNOWN_UNSPECIFIED
	}

	return tower.RecipeStep_EndingStyle(fm)
}

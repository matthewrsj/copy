package cdcontroller

import (
	"fmt"

	tower "stash.teslamotors.com/rr/towerproto"
)

// CommissionSelfTestRecipeName is the recipe name TC and CDC use to special-case loading instructions
const CommissionSelfTestRecipeName = "commission-self-test"

// TowerAvailability is a map of fixtures to their corresponding statuses and power availability for a tower
type TowerAvailability struct {
	FXRs  map[string]FXRAvailable `json:"fixtures"`
	Power PowerAvailable          `json:"power"`
}

// PowerAvailable contains power information on the tower
type PowerAvailable struct {
	CapacityW  int32 `json:"power_capacity_w"`
	InUseW     int32 `json:"power_in_use_w"`
	AvailableW int32 `json:"power_available_w"`
}

// FXRAvailable contains availability information for one FXR
type FXRAvailable struct {
	Status          string `json:"status"`
	EquipmentStatus string `json:"equipment_status"`
	Reserved        bool   `json:"reserved"`
	Allowed         bool   `json:"allowed"`
}

// ToFXRLayout converts the availability info to a FXRLayout
func (ta TowerAvailability) ToFXRLayout() (*FXRLayout, error) {
	return fxrLayoutForValidator(ta, fxrReadyForNormalOperation)
}

// ToFXRLayoutForCommissioning converts the availability info to a FXRLayout
func (ta TowerAvailability) ToFXRLayoutForCommissioning() (*FXRLayout, error) {
	return fxrLayoutForValidator(ta, fxrReadyForCommissioning)
}

func fxrLayoutForValidator(ta TowerAvailability, validator func(FXRAvailable) bool) (*FXRLayout, error) {
	f := NewFXRLayout()

	for loc, a := range ta.FXRs {
		c, err := ColFromLoc(loc)
		if err != nil {
			return nil, fmt.Errorf("column from location (%s): %v", loc, err)
		}

		l, err := LvlFromLoc(loc)
		if err != nil {
			return nil, fmt.Errorf("level from location: %v", err)
		}

		s, ok := tower.FixtureStatus_value[a.Status]
		if !ok {
			return nil, fmt.Errorf("invalid status '%s'", a.Status)
		}

		status := tower.FixtureStatus(s)

		if err := f.Set(
			Coordinates{Col: c, Lvl: l},
			&FXR{
				Status: status,
				Free:   validator(a),
			},
		); err != nil {
			return nil, fmt.Errorf("set fixture layout: %v", err)
		}
	}

	return f, nil
}

func fxrReadyForNormalOperation(fa FXRAvailable) bool {
	return fxrEquipmentIsMatchedAndReady(fa, tower.EquipmentStatus_EQUIPMENT_STATUS_IN_OPERATION)
}

func fxrReadyForCommissioning(fa FXRAvailable) bool {
	return fxrEquipmentIsMatchedAndReady(fa, tower.EquipmentStatus_EQUIPMENT_STATUS_NEEDS_COMMISSIONING)
}

func fxrEquipmentIsMatchedAndReady(fa FXRAvailable, es tower.EquipmentStatus) bool {
	s, ok := tower.FixtureStatus_value[fa.Status]
	if !ok {
		return false
	}

	e, ok := tower.EquipmentStatus_value[fa.EquipmentStatus]
	if !ok {
		return false
	}

	return tower.FixtureStatus(s) == tower.FixtureStatus_FIXTURE_STATUS_IDLE &&
		tower.EquipmentStatus(e) == es &&
		!fa.Reserved && fa.Allowed
}

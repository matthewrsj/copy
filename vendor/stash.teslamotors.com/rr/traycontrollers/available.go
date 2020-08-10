package traycontrollers

import (
	"fmt"

	pb "stash.teslamotors.com/rr/towerproto"
)

// CommissionSelfTestRecipeName is the recipe name TC and CDC use to special-case loading instructions
const CommissionSelfTestRecipeName = "commission-self-test"

// Availability is a map of fixtures to their corresponding statuses
type Availability map[string]FXRAvailable

// FXRAvailable contains availability information for one FXR
type FXRAvailable struct {
	Status          string `json:"status"`
	EquipmentStatus string `json:"equipment_status"`
	Reserved        bool   `json:"reserved"`
	Allowed         bool   `json:"allowed"`
}

// ToFXRLayout converts the availability info to a FXRLayout
func (as Availability) ToFXRLayout() (*FXRLayout, error) {
	return fxrLayoutForValidator(as, fxrReadyForNormalOperation)
}

// ToFXRLayoutForCommissioning converts the availability info to a FXRLayout
func (as Availability) ToFXRLayoutForCommissioning() (*FXRLayout, error) {
	return fxrLayoutForValidator(as, fxrReadyForCommissioning)
}

func fxrLayoutForValidator(as Availability, validator func(FXRAvailable) bool) (*FXRLayout, error) {
	f := NewFXRLayout()

	for loc, a := range as {
		c, err := ColFromLoc(loc)
		if err != nil {
			return nil, fmt.Errorf("column from location (%s): %v", loc, err)
		}

		l, err := LvlFromLoc(loc)
		if err != nil {
			return nil, fmt.Errorf("level from location: %v", err)
		}

		s, ok := pb.FixtureStatus_value[a.Status]
		if !ok {
			return nil, fmt.Errorf("invalid status '%s'", a.Status)
		}

		status := pb.FixtureStatus(s)

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
	return fxrEquipmentIsMatchedAndReady(fa, pb.EquipmentStatus_EQUIPMENT_STATUS_IN_OPERATION)
}

func fxrReadyForCommissioning(fa FXRAvailable) bool {
	return fxrEquipmentIsMatchedAndReady(fa, pb.EquipmentStatus_EQUIPMENT_STATUS_NEEDS_COMMISSIONING)
}

func fxrEquipmentIsMatchedAndReady(fa FXRAvailable, es pb.EquipmentStatus) bool {
	s, ok := pb.FixtureStatus_value[fa.Status]
	if !ok {
		return false
	}

	e, ok := pb.EquipmentStatus_value[fa.EquipmentStatus]
	if !ok {
		return false
	}

	return pb.FixtureStatus(s) == pb.FixtureStatus_FIXTURE_STATUS_IDLE &&
		pb.EquipmentStatus(e) == es &&
		!fa.Reserved && fa.Allowed
}

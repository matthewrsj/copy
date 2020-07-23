package traycontrollers

import (
	"fmt"

	pb "stash.teslamotors.com/rr/towerproto"
)

// Availability is a map of fixtures to their corresponding statuses
type Availability map[string]FXRAvailable

// FXRAvailable contains availability information for one FXR
type FXRAvailable struct {
	Status   string `json:"status"`
	Reserved bool   `json:"reserved"`
}

// ToFXRLayout converts the availability info to a FXRLayout
func (as Availability) ToFXRLayout() (*FXRLayout, error) {
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
				// IDLE means there is no tray sitting in there waiting to start
				// READY means a tray is already present in the fixture, therefore it is InUse
				InUse: status != pb.FixtureStatus_FIXTURE_STATUS_IDLE || a.Reserved,
			},
		); err != nil {
			return nil, fmt.Errorf("set fixture layout: %v", err)
		}
	}

	return f, nil
}

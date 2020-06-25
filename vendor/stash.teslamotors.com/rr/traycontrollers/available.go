package traycontrollers

import (
	"fmt"

	pb "stash.teslamotors.com/rr/towerproto"
)

// Availability is a slice of fixtures and their corresponding statuses
type Availability []struct {
	Location string           `json:"location"`
	Status   pb.FixtureStatus `json:"status"`
}

// ToFXRLayout converts the availability info to a FXRLayout
func (as Availability) ToFXRLayout() (*FXRLayout, error) {
	f := NewFXRLayout()

	for _, a := range as {
		c, err := ColFromLoc(a.Location)
		if err != nil {
			return nil, fmt.Errorf("column from location (%s): %v", a.Location, err)
		}

		l, err := LvlFromLoc(a.Location)
		if err != nil {
			return nil, fmt.Errorf("level from location: %v", err)
		}

		if err := f.Set(
			Coordinates{Col: c, Lvl: l},
			&FXR{
				Status: a.Status,
				InUse:  a.Status != pb.FixtureStatus_FIXTURE_STATUS_READY,
			},
		); err != nil {
			return nil, fmt.Errorf("set fixture layout: %v", err)
		}
	}

	return f, nil
}

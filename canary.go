package towercontroller

import (
	"sort"
)

type canaryResponse struct {
	FixturesUp   []string `json:"fixtures_broadcasting"`
	FixturesDown []string `json:"fixtures_not_broadcasting"`
}

// CanaryCallback is called every time the canary endpoint is hit. This function generates the data portion of the
// canary response with fixtures up and fixtures down lists.
func CanaryCallback(registry map[string]*FixtureInfo) func() interface{} {
	return func() interface{} {
		cr := canaryResponse{
			FixturesUp:   []string{},
			FixturesDown: []string{},
		}

		for name, info := range registry {
			if _, err := info.FixtureState.GetOp(); err != nil {
				cr.FixturesDown = append(cr.FixturesDown, name)
			} else {
				cr.FixturesUp = append(cr.FixturesUp, name)
			}
		}

		sort.Slice(cr.FixturesUp, func(i, j int) bool {
			return cr.FixturesUp[i] < cr.FixturesUp[j]
		})

		sort.Slice(cr.FixturesDown, func(i, j int) bool {
			return cr.FixturesDown[i] < cr.FixturesDown[j]
		})

		return cr
	}
}

package traycontrollers

import (
	"fmt"
	"strconv"
	"strings"
	"sync"

	pb "stash.teslamotors.com/rr/towerproto"
)

// FXR is the status of an individual fixture
type FXR struct {
	Status          pb.FixtureStatus
	EquipmentStatus pb.EquipmentStatus
	Free            bool
	Coord           Coordinates
}

// GetForward returns the forward FXR from the crane's perspective
func GetForward(f1, f2 *FXR) *FXR {
	if f1.Coord.Col > f2.Coord.Col {
		return f1
	}

	return f2
}

// NumCol and NumLevel are the number of columns and levels in a tower
const (
	NumCol   = 2
	NumLevel = 12
)

// FXRLayout is the physical layout of the tower; two columns, 12 rows
type FXRLayout struct {
	layout [NumCol][NumLevel]*FXR
	mx     *sync.Mutex
}

// NewFXRLayout returns a new FXRLayout
func NewFXRLayout() *FXRLayout {
	return &FXRLayout{
		layout: [2][12]*FXR{},
		mx:     &sync.Mutex{},
	}
}

// Coordinates define the column and level of the fixture
type Coordinates struct {
	Col, Lvl int
}

// ValidLevel returns true if the level is valid
func (c Coordinates) ValidLevel() bool {
	return ValidLoc(c.Lvl, 1, NumLevel)
}

// IsNeighborOf returns whether the fxr is a neighbor of f2
func (f *FXR) IsNeighborOf(f2 *FXR) bool {
	return f.Coord.Lvl == f2.Coord.Lvl
}

// ValidLoc returns true if the location is valid
func ValidLoc(loc, min, max int) bool {
	return loc >= min && loc <= max
}

// AreValid returns whether the coordinates are valid or not
func (c Coordinates) AreValid() bool {
	return c.ValidLevel()
}

func (c Coordinates) colIdx() int {
	return (c.Col + 1) % 2
}

func (c Coordinates) lvlIdx() int {
	return c.Lvl - 1
}

// Get gets the FXR at the coordinates
func (fl *FXRLayout) Get(coord Coordinates) *FXR {
	if !coord.AreValid() {
		return nil
	}

	return fl.layout[coord.colIdx()][coord.lvlIdx()]
}

// GetNeighbor gets the neighbor for the passed-in coordinates
func (fl *FXRLayout) GetNeighbor(coord Coordinates) *FXR {
	nc := Coordinates{
		Lvl: coord.Lvl,
	}

	if coord.colIdx() == 0 { // column 1, find neighbor in column 2
		nc.Col = coord.Col + 1
	} else { // column 2, find neighbor in column 1
		nc.Col = coord.Col - 1
	}

	return fl.Get(nc)
}

// GetTwoFXRs returns two FXRs in order of physical front/back location
// in the case where there is not a front/back fixture they are returned in
// order of lower/higher fixtures.
// nolint:gocognit // the algorithm is a simple one if all in one place
func (fl *FXRLayout) GetTwoFXRs() (front, back *FXR) {
	var (
		col1Lowest, col2Lowest             *FXR
		overallLowest, overallSecondLowest *FXR
	)

	col := fl.layout[0]
	c := Coordinates{Col: 1}

	for lvl := range col {
		c.Lvl = lvl + 1

		current, neighbor := fl.Get(c), fl.GetNeighbor(c)

		if current != nil && current.Free {
			if overallLowest == nil {
				overallLowest = current
			} else if overallSecondLowest == nil {
				overallSecondLowest = current
			}

			if col1Lowest == nil {
				col1Lowest = current
			}
		}

		if neighbor != nil && neighbor.Free {
			if overallLowest == nil {
				overallLowest = neighbor
			} else if overallSecondLowest == nil {
				overallSecondLowest = neighbor
			}

			if col2Lowest == nil {
				col2Lowest = neighbor
			}
		}

		if current != nil && current.Free && neighbor != nil && neighbor.Free {
			// found two available next to each other, two tray place
			// return col2 first as this is the most forward tray
			return neighbor, current
		}
	}

	// col2 first as this is the most forward tray
	return getMinimumTravelDistance(col2Lowest, col1Lowest, overallLowest, overallSecondLowest)
}

const _maximumEfficientTravelHeight = 3

// getMinimumTravelDistance calculates the most efficient travel distance between two pairs of
// FXRs. On one hand we have the lowest from each column, on the other hand we have the overall
// first and second lowest. Because of basic trigonometry it is not always more efficient to
// request place on lowest from each column.
//
// If the height diff between the lowest on each column is greater than the height diff between the
// two absolute lowest plus the maximum height at which the crane can efficiently travel.
// Calculated using pythagorean theorem.
// sqrt(a^2 + b^2) = c where a is the distance between levels (14")
//                           b is the distance between columns (43")
//                           c is the travel distance of the crane to perform a place
// It is only efficient to place one in each column if the height difference for that place is
// 3 or less levels different from the overall lowest and overall second lowest, even though
// those require lateral movement.
//
// A more direct calculation is with the ratio between column/level differences normalized to level
//     sqrt(1^2 + 2.8^2) = 3.03
// which shows that a difference of 3 units of level is the threshold upon which we should make these
// decisions.
//
// This is not calculated on the fly, instead a constant _maximumEfficientTravelHeight was introduced.
func getMinimumTravelDistance(frontLowest, backLowest, overallLowest, overallSecondLowest *FXR) (front, back *FXR) {
	// if either of these are nil we can't use these, use overall
	if frontLowest == nil || backLowest == nil {
		return overallLowest, overallSecondLowest
	}

	// if they are different columns we've already optimized for lowest with front/backLowest
	if overallLowest.Coord.Col != overallSecondLowest.Coord.Col {
		return frontLowest, backLowest
	}

	diffByColumn := frontLowest.Coord.Lvl - backLowest.Coord.Lvl
	if diffByColumn < 0 {
		diffByColumn *= -1
	}

	diffByLevel := overallSecondLowest.Coord.Lvl - overallLowest.Coord.Lvl // no need to abs this

	if diffByLevel <= diffByColumn-_maximumEfficientTravelHeight {
		return overallLowest, overallSecondLowest
	}

	return frontLowest, backLowest
}

// GetOneFXR returns the lowest and rear-most fixture available
func (fl *FXRLayout) GetOneFXR() *FXR {
	// prioritize lower levels (shortest route for crane)
	// this means we loop over level then column instead of column then level.
	for i := 1; i <= NumLevel; i++ {
		for j := 1; j <= NumCol; j++ {
			if current := fl.Get(Coordinates{Col: j, Lvl: i}); current != nil && current.Free {
				return current
			}
		}
	}

	// nothing found
	return nil
}

// GetAvail returns the number of available FXRs in the system
func (fl *FXRLayout) GetAvail() int {
	var avail int

	for _, col := range fl.layout {
		for _, f := range col {
			if f != nil && f.Free {
				avail++
			}
		}
	}

	return avail
}

// Set sets the fxr to the coordinates
func (fl *FXRLayout) Set(coord Coordinates, fxr *FXR) error {
	if !coord.AreValid() {
		return fmt.Errorf("invalid coordinates: %#v", coord)
	}

	fl.mx.Lock()
	defer fl.mx.Unlock()

	fxr.Coord = coord
	fl.layout[coord.colIdx()][coord.lvlIdx()] = fxr

	return nil
}

func (fl *FXRLayout) String() string {
	var ss []string

	for i := NumLevel; i > 0; i-- {
		left := fl.Get(Coordinates{
			Col: 1,
			Lvl: i,
		})

		right := fl.Get(Coordinates{
			Col: 2,
			Lvl: i,
		})

		ss = append(ss,
			[]string{
				fmt.Sprintf("col: %d, lvl: %d, status: %s, free: %v", left.Coord.Col, left.Coord.Lvl, left.Status, left.Free),
				fmt.Sprintf("col: %d, lvl: %d, status: %s, free: %v", right.Coord.Col, right.Coord.Lvl, right.Status, right.Free),
			}...,
		)
	}

	return strings.Join(ss, "||")
}

type locIdx int

const (
	_towerIdx locIdx = iota + 2
	_lvlIdx
)

// ColFromLoc returns the column from the fxr string
func ColFromLoc(fxr string) (int, error) {
	// MFGSYS-WORKCENTER-EQUIP(tower)-WORKSTN
	return fromFXR(fxr, _towerIdx)
}

// LvlFromLoc returns the level from the fxr string
func LvlFromLoc(fxr string) (int, error) {
	// MFGSYS-WORKCENTER-EQUIP-WORKSTN(level)
	return fromFXR(fxr, _lvlIdx)
}

func fromFXR(fxr string, idx locIdx) (int, error) {
	fields := strings.Split(fxr, "-")
	if len(fields) != 4 {
		return 0, fmt.Errorf("invalid location %s; length of fields %d (must be 4)", fxr, len(fields))
	}

	return strconv.Atoi(fields[idx])
}

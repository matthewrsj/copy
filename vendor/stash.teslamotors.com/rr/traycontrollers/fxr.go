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
	Status pb.FixtureStatus
	InUse  bool
	Coord  Coordinates
}

func (f *FXR) String() string {
	if f == nil {
		return "nil"
	}

	inUse := "in use"
	if !f.InUse {
		inUse = "not " + inUse
	}

	return fmt.Sprintf("%s ; %s", inUse, f.Status.String())
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

// GetForTrays gets enough fixtures for the available trays, if it can
func (fl *FXRLayout) GetForTrays(n int) []*FXR {
	if n == 2 {
		// if we get two we first look through the whole tower for neighbors
		// to do this loop over one column. Only need to do one since we are
		// looking for both to be available, and can check GetNeighbor for the
		// corresponding column on the other level
		col := fl.layout[0]
		for j := range col {
			var current, neighbor *FXR

			c := Coordinates{
				Col: 1, // coordinates are one-indexed
				Lvl: j + 1,
			}

			if current = fl.Get(c); current == nil || current.InUse {
				continue
			}

			if neighbor = fl.GetNeighbor(c); neighbor == nil || neighbor.InUse {
				continue
			}

			// found two available next to each other, two tray place
			return []*FXR{current, neighbor}
		}
	}

	var nfxr []*FXR

	// prioritize lower levels (shortest route for crane)
	// this means we loop over level then column instead of column then level.
	for i := 1; i <= NumLevel; i++ {
		for j := 1; j <= NumCol; j++ {
			if current := fl.Get(Coordinates{Col: j, Lvl: i}); current != nil && !current.InUse {
				nfxr = append(nfxr, current)
				if len(nfxr) == n {
					return nfxr
				}
			}
		}
	}

	return nfxr
}

// GetAvail returns the number of available FXRs in the system
func (fl *FXRLayout) GetAvail() int {
	var avail int

	for _, col := range fl.layout {
		for _, f := range col {
			if f != nil && !f.InUse {
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
				fmt.Sprintf("col: %d, lvl: %d, status: %s, in-use: %v", left.Coord.Col, left.Coord.Lvl, left.Status, left.InUse),
				fmt.Sprintf("col: %d, lvl: %d, status: %s, in-use: %v", right.Coord.Col, right.Coord.Lvl, right.Status, right.InUse),
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

package traycontrollers

import (
	"fmt"
	"strconv"
)

// Orientation defines one of the four orientations
// for a tray. A, B, C, or D.
type Orientation int

// There are four default orientations
const (
	OrientationA = iota + 1
	OrientationB
	OrientationC
	OrientationD
)

// NewOrientation returns a new Orientation based on the byte passed in
func NewOrientation(input byte) (Orientation, error) {
	switch input {
	case 'a', 'A':
		return OrientationA, nil
	case 'b', 'B':
		return OrientationB, nil
	case 'c', 'C':
		return OrientationC, nil
	case 'd', 'D':
		return OrientationD, nil
	default:
		return 0, fmt.Errorf("orientation \"%v\" invalid", input)
	}
}

func (o Orientation) String() string {
	switch o {
	case OrientationA:
		return "A"
	case OrientationB:
		return "B"
	case OrientationC:
		return "C"
	case OrientationD:
		return "D"
	default:
		return strconv.Itoa(int(o))
	}
}

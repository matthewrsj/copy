package towercontroller

import (
	"fmt"
	"strconv"
)

// Orientation defines one of the four orientations
// for a tray. A, B, C, or D.
type Orientation int

const (
	_orientA = iota + 1
	_orientB
	_orientC
	_orientD
)

func newOrientation(input byte) (Orientation, error) {
	switch input {
	case 'a', 'A':
		return _orientA, nil
	case 'b', 'B':
		return _orientB, nil
	case 'c', 'C':
		return _orientC, nil
	case 'd', 'D':
		return _orientD, nil
	default:
		return 0, fmt.Errorf("orientation \"%v\" invalid", input)
	}
}

func (o Orientation) String() string {
	switch o {
	case _orientA:
		return "A"
	case _orientB:
		return "B"
	case _orientC:
		return "C"
	case _orientD:
		return "D"
	default:
		return strconv.Itoa(int(o))
	}
}

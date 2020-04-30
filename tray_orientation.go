package towercontroller

import (
	"fmt"
)

type orientation int

const (
	_orientA = iota + 1
	_orientB
	_orientC
	_orientD
)

func newOrientation(input byte) (orientation, error) {
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

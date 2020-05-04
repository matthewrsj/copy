package towercontroller

import "strconv"

type status int

const (
	_statusStart = iota + 1
	_statusEnd
)

func (s status) isValid() bool {
	return s >= _statusStart && s <= _statusEnd
}

func (s status) String() string {
	switch s {
	case _statusStart:
		return "start"
	case _statusEnd:
		return "end"
	default:
		return strconv.Itoa(int(s))
	}
}

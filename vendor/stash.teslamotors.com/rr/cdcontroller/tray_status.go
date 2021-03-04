package cdcontroller

import "strconv"

// TrayStatus is a type describing the status of a tray's process
type TrayStatus int

// Statuses for start and end of a process
const (
	StatusStart = iota + 1
	StatusEnd
)

func (s TrayStatus) String() string {
	switch s {
	case StatusStart:
		return "start"
	case StatusEnd:
		return "end"
	default:
		return strconv.Itoa(int(s))
	}
}

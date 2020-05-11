package cellapi

import "strconv"

type TrayStatus int

const (
	StatusStart = iota + 1
	StatusEnd
)

func (s TrayStatus) isValid() bool {
	return s >= StatusStart && s <= StatusEnd
}

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

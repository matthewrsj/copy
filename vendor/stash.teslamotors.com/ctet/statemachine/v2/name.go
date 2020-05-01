package statemachine

import (
	"reflect"
	"strings"
)

// NameOf returns the set Name of the state or the name of the type if Name is
// not set
func NameOf(s State) string {
	// return the name if set
	if s.Name() != "" {
		return s.Name()
	}

	// find the name from the last field in the type
	// example type name: *statemachine.Common
	// we only care about "Common"
	fullType := reflect.TypeOf(s).String()
	names := strings.Split(fullType, ".")
	// per documentation at https://golang.org/pkg/strings/#Split
	// names will always be at least of length one
	return strings.Trim(names[len(names)-1], "{ }")
}

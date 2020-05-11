package towercontroller

import (
	"fmt"
	"regexp"
)

const _fixtureRegex = `^([A-Za-z0-9]+)-([A-Za-z0-9]+)-([A-Za-z0-9]+)-([A-Za-z0-9]+)$`

type FixtureBarcode struct {
	Location, Aisle, Tower, Fxn string
	raw                         string
}

func isValidFixtureBarcode(input string) error {
	r := regexp.MustCompile(_fixtureRegex)
	match := r.FindStringSubmatch(input)

	// first contains entire raw input
	if len(match) != 5 {
		return fmt.Errorf("invalid fixture barcode %s does not follow pattern \"%s\"", input, _fixtureRegex)
	}

	return nil
}

// NewFixtureBarcode returns a new FixtureBarcode object using fields parsed from the input string.
func NewFixtureBarcode(input string) (FixtureBarcode, error) {
	r := regexp.MustCompile(_fixtureRegex)
	match := r.FindStringSubmatch(input)

	// first contains entire raw input
	if len(match) != 5 {
		return FixtureBarcode{}, fmt.Errorf("invalid fixture barcode %s does not follow pattern \"%s\"", input, _fixtureRegex)
	}

	return FixtureBarcode{
		Location: match[1],
		Aisle:    match[2],
		Tower:    match[3],
		Fxn:      match[4],
		raw:      input,
	}, nil
}

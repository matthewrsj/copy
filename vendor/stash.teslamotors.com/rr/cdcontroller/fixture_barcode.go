package cdcontroller

import (
	"fmt"
	"regexp"
)

const _fixtureRegex = `^([A-Za-z0-9]+)-([A-Za-z0-9]+)-([A-Za-z0-9]+)-([A-Za-z0-9]+)$`

// FixtureBarcode is the barcode of the fixture
type FixtureBarcode struct {
	Location, Aisle, Tower, Fxn, Raw string
}

// IsValidFixtureBarcode returns an error if the input string is not a valid
// fixture barcode.
func IsValidFixtureBarcode(input string) error {
	r := regexp.MustCompile(_fixtureRegex)
	match := r.FindStringSubmatch(input)

	// first contains entire Raw input
	if len(match) != 5 {
		return fmt.Errorf("invalid fixture barcode %s does not follow pattern \"%s\"", input, _fixtureRegex)
	}

	return nil
}

// NewFixtureBarcode returns a new FixtureBarcode object using fields parsed from the input string.
func NewFixtureBarcode(input string) (FixtureBarcode, error) {
	r := regexp.MustCompile(_fixtureRegex)
	match := r.FindStringSubmatch(input)

	// first contains entire Raw input
	if len(match) != 5 {
		return FixtureBarcode{}, fmt.Errorf("invalid fixture barcode %s does not follow pattern \"%s\"", input, _fixtureRegex)
	}

	return FixtureBarcode{
		Location: match[1],
		Aisle:    match[2],
		Tower:    match[3],
		Fxn:      match[4],
		Raw:      input,
	}, nil
}

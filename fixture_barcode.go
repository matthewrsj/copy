package towercontroller

import (
	"fmt"
	"regexp"
)

const _fixtureRegex = `^([A-Za-z0-9]+)-([A-Za-z0-9]+)-([A-Za-z0-9]+)-([A-Za-z0-9]+)$`

type fixtureBarcode struct {
	location, aisle, tower, fxn string
	raw                         string
}

func newFixtureBarcode(input string) (fixtureBarcode, error) {
	r := regexp.MustCompile(_fixtureRegex)
	match := r.FindStringSubmatch(input)

	// first contains entire raw input
	if len(match) != 5 {
		return fixtureBarcode{}, fmt.Errorf("invalid fixture barcode %s does not follow pattern \"%s\"", input, _fixtureRegex)
	}

	return fixtureBarcode{
		location: match[1],
		aisle:    match[2],
		tower:    match[3],
		fxn:      match[4],
		raw:      input,
	}, nil
}

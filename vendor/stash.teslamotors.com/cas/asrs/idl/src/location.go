package asrs

import (
	"errors"
	"fmt"
	"strings"
)

const LocationTypeCM = "CM"
const LocationTypePort = "Port"

func LocationToString(location *Location) (string, error) {

	if location == nil {
		return "", errors.New("empty location is invalid")
	}

	var out string
	switch l := location.LocationByType.(type) {
	case *Location_CmFormat:
		out = fmt.Sprintf("%s:%s", LocationTypeCM, l.CmFormat.EquipmentId)
	case *Location_PortFormat:
		out = fmt.Sprintf("%s:%s", LocationTypePort, l.PortFormat)
	default:
		return "", fmt.Errorf("location type %T is no handled", l)
	}

	return out, nil
}

func LocationFromString(location string) (*Location, error) {
	tok := strings.SplitN(location, ":", 2)
	if len(tok) != 2 {
		return nil, fmt.Errorf("location unparseable, missing type: format [%s]", location)
	}

	switch tok[0] {
	case LocationTypeCM:

		// Shenanigans to accommodate unstructured Value. Alternative would be to use variable number of values,
		// at which point universal string location seems far less complex. Start by validating tok[1]
		if len(tok[1]) < 14 || tok[1][3] != '-' || tok[1][9] != '-' || tok[1][12] != '-' {
			return nil, fmt.Errorf("location unparseable, type [%s]: format [%s]", LocationTypeCM, location)
		}
		out := Location{
			LocationByType: &Location_CmFormat{
				CmFormat: &CMLocation{
					// "CM2-63001-02-L12..."
					EquipmentId:         tok[1],
					ManufacturingSystem: tok[1][0 : 0+3],
					Workcenter:          tok[1][4 : 4+2],
					Equipment:           tok[1][6 : 6+3],
					Workstation:         tok[1][10 : 10+2],
					SubIdentifier:       tok[1][13:],
				},
			},
		}
		return &out, nil

	case LocationTypePort:
		out := Location{LocationByType: &Location_PortFormat{PortFormat: tok[1]}}
		return &out, nil

	default:
	}

	return nil, fmt.Errorf("location unparseable, bad type [%s] in [%s]", tok[0], location)
}

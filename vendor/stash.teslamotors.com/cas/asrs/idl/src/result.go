package asrs

import (
	"encoding/json"
	"errors"
)

type JsonEncodedPayload struct {
	Description string          `json:"description"`
	Payload     json.RawMessage `json:"payload"`
}

func ProcessResultsToString(result *ProcessResults) (string, error) {
	if result == nil {
		return "", errors.New("empty ProcessResults is invalid")
	}

	// We go through intermediate structure to control marshalling of payload
	// rather than through jsonpb (and above json encoded payload from being base64 encoded).
	// If we choose to use non-JSON results, we can relax this with tentative json decode,
	// followed by jsonpb unmarshalling.
	jr := &JsonEncodedPayload{
		Description: result.Description,
		Payload:     result.Payload,
	}
	out, err := json.Marshal(jr)
	return string(out), err
}

func ProcessResultsFromString(in string) (*ProcessResults, error) {
	var jr JsonEncodedPayload
	err := json.Unmarshal([]byte(in), &jr)
	if err != nil {
		return nil, err
	}

	// We go through intermediate structure to control unmarshalling of step_configuration
	// rather than through jsonpb.
	if jr.Description != "" {
		return &ProcessResults{
			Description: jr.Description,
			Payload:     jr.Payload,
		}, nil
	}

	return nil, nil
}

package traycontrollers

import "fmt"

// BroadcastEndpoint is the endpoint both TC and CDC use to handle broadcast requests
const BroadcastEndpoint = "/broadcast"

// BroadcastScale defines the scale at which the broadcast request should be propagated
type BroadcastScale int

// Defined scales
const (
	ScaleNone   BroadcastScale = iota
	ScaleColumn                // broadcast to entire column
	ScaleTower                 // broadcast to entire tower (both columns)
	ScaleAisle                 // broadcast to entire aisle
	ScaleGlobal                // broadcast to all of charge/discharge
)

func (s BroadcastScale) String() string {
	switch s {
	case ScaleNone:
		return "SCALE_NONE"
	case ScaleColumn:
		return "SCALE_COLUMN"
	case ScaleTower:
		return "SCALE_TOWER"
	case ScaleAisle:
		return "SCALE_AISLE"
	case ScaleGlobal:
		return "SCALE_GLOBAL"
	default:
		return fmt.Sprintf("INVALID_SCALE[%d]", s)
	}
}

// BroadcastOperation defines the operation each node should perform
type BroadcastOperation int

// Defined operations
const (
	OperationNone            BroadcastOperation = iota
	OperationStopFormation                      // send a STOP request
	OperationPauseFormation                     // send a PAUSE request
	OperationResumeFormation                    // send a RESUME request
	OperationStopIsoCheck                       // send a STOP ISOLATION request
	OperationFaultReset                         // send a FAULT RESET request
)

func (o BroadcastOperation) String() string {
	switch o {
	case OperationNone:
		return "OPERATION_NONE"
	case OperationStopFormation:
		return "OPERATION_STOP_FORMATION"
	case OperationPauseFormation:
		return "OPERATION_PAUSE_FORMATION"
	case OperationResumeFormation:
		return "OPERATION_RESUME_FORMATION"
	case OperationStopIsoCheck:
		return "OPERATION_STOP_ISO_CHECK"
	case OperationFaultReset:
		return "OPERATION_FAULT_RESET"
	default:
		return fmt.Sprintf("INVALID_OPERATION[%d]", o)
	}
}

// BroadcastReason defines the reason for the broadcast request. Mostly for local log purposes.
type BroadcastReason int

// Defined reasons
const (
	ReasonNone           BroadcastReason = iota
	ReasonFireLevel0                     // fire level 0
	ReasonFireLevel1                     // fire level 1
	ReasonIsolationReset                 // isolation reset on tower
)

func (r BroadcastReason) String() string {
	switch r {
	case ReasonNone:
		return "REASON_NONE"
	case ReasonFireLevel0:
		return "REASON_FIRE_LEVEL_0"
	case ReasonFireLevel1:
		return "REASON_FIRE_LEVEL_1"
	case ReasonIsolationReset:
		return "REASON_ISOLATION_RESET"
	default:
		return fmt.Sprintf("INVALID_REASON[%d]", r)
	}
}

// BroadcastOrigin is the origin of the broadcast request
type BroadcastOrigin struct {
	Aisle    string `json:"aisle"`
	Location string `json:"location"` // Location contains the column-level location in the form of "CC-LL" (01-03 is column 1, level 3)
}

// BroadcastRequest is a request from a tower controller to broadcast to other
// controllers in the system
type BroadcastRequest struct {
	// this information comes from the requestor
	Scale         BroadcastScale     `json:"scale"`
	Operation     BroadcastOperation `json:"operation"`
	Reason        BroadcastReason    `json:"reason"`
	Originator    BroadcastOrigin    `json:"origin"`
	ExcludeOrigin bool               `json:"exclude_origin"`

	// This information is populated by C/D Controller when it broadcasts the request.
	// It is only populated when the scale is less than tower scale, since in that case the C/D Controller will be sending
	// an individual request for each fixture it is targeting.
	Target string `json:"target"` // Target contains the column-level location in the form of "CC-LL" (01-03 is column 1, level 3)
}

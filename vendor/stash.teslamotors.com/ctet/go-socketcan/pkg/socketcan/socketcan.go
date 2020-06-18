package socketcan

// CanFrame is the structure used to encode a CAN data frame
// nolint:maligned // keep for documentation (for now)
type CanFrame struct {
	ArbID    uint32
	DLC      byte
	Data     []byte
	Extended bool
}

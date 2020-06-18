package socketcan

import (
	"bytes"
	"encoding/binary"

	"golang.org/x/sys/unix"
)

// FD and BRS flags
// nolint:stylecheck // match kernel documentation
const (
	CAN_ISOTP_LL_OPTS = 5
	CAN_FD_MTU        = 72
	CAN_BRS_FLAG      = 1
	CAN_DEFAULT_DL    = 8
)

// nolint:stylecheck // match kernel documentation
type canIsotpLLOptions struct {
	mtu      uint8
	tx_dl    uint8
	tx_flags uint8
}

// SetCANFD sets the underlying socket to generate flexible data-rate CAN frames
func (i Interface) SetCANFD() error {
	opts := canIsotpLLOptions{
		mtu: CAN_FD_MTU,
		// TODO: provide control over data length
		tx_dl:    CAN_DEFAULT_DL,
		tx_flags: CAN_BRS_FLAG,
	}

	var buf bytes.Buffer
	if err := binary.Write(&buf, getEndianness(), opts); err != nil {
		return err
	}

	return unix.SetsockoptString(i.SocketFd, SOL_CAN_ISOTP, CAN_ISOTP_LL_OPTS, buf.String())
}

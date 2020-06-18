package socketcan

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"golang.org/x/sys/unix"
)

// CanIsotpOptions are the options that can be set in the kernel for a CAN ISOTP interface
// nolint:structcheck,unused,stylecheck // this specific structure is required by the kernel
type CanIsotpOptions struct {
	flags          uint32
	frame_txtime   uint32
	ext_address    uint8
	txpad_content  uint8
	rxpad_content  uint8
	rx_ext_address uint8
}

// System and CAN level flags
// nolint:stylecheck // match kernel documentation
const (
	SYS_SETSOCKOPT = 54
	SOL_CAN_BASE   = 100
	SOL_CAN_ISOTP  = 106
	CAN_ISOTP_OPTS = 1
	CAN_EFF_FLAG   = 0x80000000
)

// Flags for ISOTP-specific frames
// nolint:stylecheck // match kernel documentation
const (
	CAN_ISOTP_EXTEND_ADDR = 0x002
	CAN_ISOTP_TX_PADDING  = 0x004
	CAN_ISOTP_RX_PADDING  = 0x008
)

// NewIsotpInterface creates a new ISOTP CAN interface
func NewIsotpInterface(ifName string, rxID uint32, txID uint32) (Interface, error) {
	canIf := Interface{}
	canIf.ifType = IF_TYPE_ISOTP

	fd, err := unix.Socket(unix.AF_CAN, unix.SOCK_DGRAM, CAN_ISOTP)
	if err != nil {
		return canIf, err
	}

	ifIndex, err := getIfIndex(fd, ifName)
	if err != nil {
		return canIf, err
	}

	// set extended ID flags if required
	if rxID > 0x7FF {
		rxID |= CAN_EFF_FLAG
	}

	if txID > 0x7FF {
		txID |= CAN_EFF_FLAG
	}

	addr := &unix.SockaddrCAN{Ifindex: ifIndex, RxID: rxID, TxID: txID}
	if err = unix.Bind(fd, addr); err != nil {
		return canIf, err
	}

	canIf.IfName = ifName
	canIf.SocketFd = fd

	return canIf, nil
}

// Rebind re-binds the interface to the underlying socket
func (i Interface) Rebind(rxID uint32, txID uint32) error {
	ifIndex, err := getIfIndex(i.SocketFd, i.IfName)
	if err != nil {
		return err
	}

	// set extended ID flags if required
	if rxID > 0x7FF {
		rxID |= CAN_EFF_FLAG
	}

	if txID > 0x7FF {
		txID |= CAN_EFF_FLAG
	}

	addr := &unix.SockaddrCAN{Ifindex: ifIndex, RxID: rxID, TxID: txID}

	return unix.Bind(i.SocketFd, addr)
}

// SendBuf sends a buffer to the interface
func (i Interface) SendBuf(data []byte) error {
	if i.ifType != IF_TYPE_ISOTP {
		return fmt.Errorf("interface is not isotp type")
	}

	_, err := unix.Write(i.SocketFd, data)

	return err
}

// RecvBuf receives a buffer off the interface
func (i Interface) RecvBuf() ([]byte, error) {
	if i.ifType != IF_TYPE_ISOTP {
		return []byte{}, fmt.Errorf("interface is not isotp type")
	}

	data := make([]byte, 4095)

	ln, err := unix.Read(i.SocketFd, data)
	if err != nil {
		return data, err
	}

	// only return the valid bytes (0 to length received)
	return data[:ln], nil
}

// SetTxPadding sets the padding of the data on the CAN frame
func (i Interface) SetTxPadding(on bool, value uint8) error {
	var buf bytes.Buffer

	opts := CanIsotpOptions{}
	if on {
		opts.flags = CAN_ISOTP_TX_PADDING
	}

	opts.txpad_content = value

	err := binary.Write(&buf, getEndianness(), opts)
	if err != nil {
		return err
	}

	return unix.SetsockoptString(i.SocketFd, SOL_CAN_ISOTP, CAN_ISOTP_OPTS, buf.String())
}

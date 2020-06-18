// Package socketcan provides a CAN interface for sending RAW or ISOTP CAN frames on a CAN bus.
package socketcan

import (
	"errors"
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// nolint:stylecheck // keep all caps and underscores to match kernel documentation
const (
	CAN_RAW   = 1
	CAN_ISOTP = 6

	CAN_RAW_RECV_OWN_MSGS = 4
	SOL_CAN_RAW           = 101
)

// nolint:stylecheck // keep all caps and underscores to match kernel documentation
const (
	IF_TYPE_RAW   = 0
	IF_TYPE_ISOTP = 1
)

// Interface is the type that exposes the send and receive and other operations on the CAN socket
type Interface struct {
	IfName   string
	SocketFd int
	ifType   int
}

func getIfIndex(fd int, ifName string) (int, error) {
	ifNameRaw, err := unix.ByteSliceFromString(ifName)
	if err != nil {
		return 0, err
	}

	if len(ifNameRaw) > 16 {
		return 0, errors.New("maximum ifname length is 16 characters")
	}

	ifReq := ifreqIndex{}
	copy(ifReq.Name[:], ifNameRaw)
	err = ioctlIfreq(fd, &ifReq)

	return ifReq.Index, err
}

type ifreqIndex struct {
	Name  [16]byte
	Index int
}

func ioctlIfreq(fd int, ifreq *ifreqIndex) (err error) {
	_, _, errno := unix.Syscall(
		unix.SYS_IOCTL,
		uintptr(fd),
		unix.SIOCGIFINDEX,
		uintptr(unsafe.Pointer(ifreq)),
	)
	if errno != 0 {
		err = fmt.Errorf("ioctl: %v", errno)
	}

	return
}

// SetLoopback sets the underlying socket to loopback its own messages
func (i Interface) SetLoopback(enable bool) error {
	var value int
	if enable {
		value = 1
	}

	return unix.SetsockoptInt(i.SocketFd, SOL_CAN_RAW, CAN_RAW_RECV_OWN_MSGS, value)
}

// SetRecvTimeout sets a timeout on receives from the underlying socket
// a timeout will return an error stating the resource is temporarily unavailable
func (i Interface) SetRecvTimeout(timeout time.Duration) error {
	tv := unix.NsecToTimeval(timeout.Nanoseconds())
	return unix.SetsockoptTimeval(i.SocketFd, unix.SOL_SOCKET, unix.SO_RCVTIMEO, &tv)
}

// SetSendTimeout sets a timeout on sends on the underlying socket
func (i Interface) SetSendTimeout(timeout time.Duration) error {
	tv := unix.NsecToTimeval(timeout.Nanoseconds())
	return unix.SetsockoptTimeval(i.SocketFd, unix.SOL_SOCKET, unix.SO_SNDTIMEO, &tv)
}

// Close the underlying socket
func (i Interface) Close() error {
	return unix.Close(i.SocketFd)
}

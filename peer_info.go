package vnet

import (
	"errors"
	"net"

	"github.com/multiformats/go-multiaddr"
)

type PeerInfo struct {
	mac  net.HardwareAddr
	addr multiaddr.Multiaddr // TODO Make an array
}

func (info *PeerInfo) MarshalBinary() ([]byte, error) {
	var b []byte
	b = append(b, info.mac...)
	b = append(info.addr.Bytes())
	return b, nil
}

func (info *PeerInfo) UnmarshalBinary(b []byte) (err error) {
	if len(b) < 14 {
		return errors.New("slice is not long enougth")
	}

	info.mac = b[:14] // size of mac address
	info.addr, err = multiaddr.NewMultiaddrBytes(b[14:])
	if err != nil {
		return err
	}

	return nil
}

package vnet

import (
	"io"
	"net"
)

type VirtualNetwork interface {
}

type VirtualNetworkInterface interface {
	io.ReadWriteCloser

	Name() string
	MAC() net.HardwareAddr
}

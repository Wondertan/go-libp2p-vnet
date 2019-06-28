package vnet

import (
	net "github.com/libp2p/go-libp2p-core/network"
	ma "github.com/multiformats/go-multiaddr"
)

type networkNotify network

func (n *networkNotify) OpenedStream(net net.Network, s net.Stream)  {}
func (n *networkNotify) ClosedStream(net net.Network, s net.Stream)  {}
func (n *networkNotify) Disconnected(net net.Network, c net.Conn)    {}
func (n *networkNotify) Listen(net net.Network, _ ma.Multiaddr)      {}
func (n *networkNotify) ListenClose(net net.Network, _ ma.Multiaddr) {}

func (n *networkNotify) Connected(net net.Network, c net.Conn) {
	go func() {
		select {
		case n.newMember <- c.RemotePeer():
		case <-n.ctx.Done():
		}
	}()
}

package vnet

import (
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/songgao/packets/ethernet"
)

type networkMember struct {
	id       peer.ID
	info     *PeerInfo
	outgoing chan ethernet.Frame
	stream   inet.Stream
}

func (m *networkMember) Close() error {
	close(m.outgoing)
	return m.stream.Reset()
}

func (m *networkMember) receive() {
	for {
		select {
		case frame, ok := <-m.outgoing:
			if !ok {
				return
			}

			_, err := m.stream.Write(frame)
			if err != nil {
				m.stream.Reset()
				return
			}
		}
	}
}

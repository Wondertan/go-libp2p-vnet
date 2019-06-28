package vnet

import (
	"context"
	"net"

	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/songgao/packets/ethernet"
)

type networkMember struct {
	id       peer.ID
	mac      net.HardwareAddr
	outgoing chan ethernet.Frame
	stream   inet.Stream
}

func (m *networkMember) Send(frame ethernet.Frame) bool {
	if m.stream != nil {
		m.outgoing <- frame
		return true
	}

	return false
}

func (m *networkMember) receive(ctx context.Context) {
	defer func() {
		m.outgoing = nil
		m.stream.Reset()
	}()

	for {
		select {
		case frame := <-m.outgoing:
			log.Debugf("Sending frame to %s...", m.stream.Conn().RemotePeer())

			_, err := m.stream.Write(frame)
			if err != nil {
				log.Errorf("error writing to %s: %s", m.stream.Conn().RemotePeer(), err)
				m.stream.Reset()
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

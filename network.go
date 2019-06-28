package vnet

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"time"

	"github.com/gyf304/water/waterutil"
	logging "github.com/ipfs/go-log"
	idiscovery "github.com/libp2p/go-libp2p-core/discovery"
	"github.com/libp2p/go-libp2p-core/host"
	inet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/protocol"
	discovery "github.com/libp2p/go-libp2p-discovery"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/songgao/packets/ethernet"
)

const Protocol = protocol.ID("/tap")

const (
	MTU     = 1500
	MACSize = 6
)

var log = logging.Logger("vnet")

type network struct {
	name string

	host      host.Host
	inet      VirtualNetworkInterface
	discovery idiscovery.Discovery

	ingoing  chan ethernet.Frame
	outgoing chan ethernet.Frame
	msgs     chan *pubsub.Message

	newMember chan peer.ID
	members   map[peer.ID]*networkMember

	ctx    context.Context
	cancel context.CancelFunc
}

func NewVirtualNetwork(ctx context.Context, name string, host host.Host, inet VirtualNetworkInterface, d idiscovery.Discovery) (*network, error) {
	ctx, cancel := context.WithCancel(ctx)

	n := &network{
		name:      name,
		host:      host,
		inet:      inet,
		discovery: d,
		ingoing:   make(chan ethernet.Frame, 32),
		outgoing:  make(chan ethernet.Frame, 32),
		msgs:      make(chan *pubsub.Message, 32),
		members:   make(map[peer.ID]*networkMember),
		newMember: make(chan peer.ID),
		ctx:       ctx,
		cancel:    cancel,
	}

	err := n.bootstrap()
	if err != nil {
		return nil, err
	}

	go n.router()
	go n.handleOutgoing()
	go n.handleIngoing()
	host.SetStreamHandler(Protocol, n.handleStream)
	host.Network().Notify((*networkNotify)(n))

	return n, nil
}

func (net *network) Close() error {
	net.cancel()
	return nil
}

func (net *network) bootstrap() error {
	discovery.Advertise(net.ctx, net.discovery, net.name)

	wg := &sync.WaitGroup{}

	fctx, cancel := context.WithTimeout(net.ctx, time.Second*30)
	defer cancel()

	peers, err := net.discovery.FindPeers(fctx, net.name)
	if err != nil {
		return err
	}

	for info := range peers {
		if info.ID == net.host.ID() {
			continue
		}

		wg.Add(1)
		go func(info peer.AddrInfo) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(net.ctx, time.Second*15)
			defer cancel()

			log.Infof("Network member %s found, connecting....", info.ID)
			err := net.host.Connect(ctx, info)
			if err != nil {
				log.Debugf("error connecting to network member %s: %s", info.ID, err.Error())
				return
			}
		}(info)
	}

	wg.Wait()

	return nil
}

func (net *network) findReceivers(dest net.HardwareAddr) []*networkMember {
	var receivers []*networkMember
	isBroad := waterutil.IsBroadcast(dest)

	for _, m := range net.members {
		if isBroad || bytes.Equal(m.mac, dest) {
			receivers = append(receivers, m)
		}
	}

	return receivers
}

func (net *network) router() {
	for {
		select {
		case frame := <-net.outgoing:
			receivers := net.findReceivers(frame.Destination())
			for _, r := range receivers {
				log.Debugf("Sending from to %s", r.id)
				if !r.Send(frame) {
					delete(net.members, r.id)
				}
			}
		case p := <-net.newMember:
			m := &networkMember{
				id:       p,
				mac:      make([]byte, MACSize),
				outgoing: make(chan ethernet.Frame, 32),
			}

			go func(m *networkMember) {
				stream, err := net.host.NewStream(net.ctx, m.id, Protocol)
				if err != nil {
					log.Errorf("error establishing new stream to peer %s: %s", p, err)
					return
				}

				_, err = stream.Read(m.mac)
				if err != nil {
					log.Errorf("error sending MAC to peer %s: %s", p, err)
					return
				}

				m.stream = stream
				go m.receive(net.ctx)
			}(m)

			net.members[p] = m
		case <-net.ctx.Done():
			return
		}
	}
}

func (net *network) handleOutgoing() {
	var frame ethernet.Frame
	for {
		frame.Resize(MTU)
		n, err := net.inet.Read([]byte(frame))
		if err != nil {
			log.Errorf("error reading from VN", err)
			return
		}

		select {
		case net.outgoing <- frame[:n]:
		case <-net.ctx.Done():
			return
		}
	}
}

func (net *network) handleIngoing() {
	for {
		select {
		case frame := <-net.ingoing:
			_, err := net.inet.Write(frame)
			if err != nil {
				log.Errorf("error writing to VN", err)
				return
			}
		case <-net.ctx.Done():
			return
		}
	}
}

func (net *network) handleStream(s inet.Stream) {
	peer := s.Conn().RemotePeer()

	_, err := s.Write(net.inet.MAC())
	if err != nil {
		log.Errorf("error writing mac address to %s: %s", peer, err)
		return
	}

	var frame ethernet.Frame
	for {
		frame.Resize(MTU)
		n, err := s.Read([]byte(frame))
		if err != nil {
			if err != io.EOF {
				log.Errorf("error reading from %s: %s", peer, err)
				s.Reset()
				return
			}

			return
		}

		log.Debugf("Received frame from: %s", peer)

		select {
		case net.ingoing <- frame[:n]:
		case <-net.ctx.Done():
			return
		}
	}
}

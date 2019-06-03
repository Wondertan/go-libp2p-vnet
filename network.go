package vnet

import (
	"context"
	"github.com/gyf304/water/waterutil"
	"log"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/protocol"
	inet "github.com/libp2p/go-libp2p-net"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-pubsub"
	"github.com/multiformats/go-multiaddr"
	"github.com/songgao/packets/ethernet"
)

const Protocol = protocol.ID("tap")

type network struct {
	ctx    context.Context
	cancel context.CancelFunc

	name string

	host host.Host
	ps   *pubsub.PubSub
	sub  *pubsub.Subscription

	inet VirtualNetworkInterface

	self PeerInfo

	members map[string]*networkMember
	ingoing chan ethernet.Frame
}

func NewVirtualNetwork(ctx context.Context, name string, host host.Host, ps *pubsub.PubSub, inet VirtualNetworkInterface) (*network, error) {
	ctx, cancel := context.WithCancel(ctx)

	// TODO Add validation
	sub, err := ps.Subscribe(name)
	if err != nil {
		return nil, err
	}

	n := &network{
		ctx:    ctx,
		cancel: cancel,
		name:   name,
		host:   host,
		ps:     ps,
		sub:    sub,
		inet:   inet,
		self: PeerInfo{
			mac:  inet.MAC(),
			addr: host.Peerstore().Addrs(host.ID())[0],
		},
		members: make(map[string]*networkMember),
		ingoing: make(chan ethernet.Frame, 32),
	}

	err = n.announceSelf()
	if err != nil {
		return nil, err
	}

	host.SetStreamHandler(Protocol, n.handle)

	go n.listen()

	return n, err
}

func (net *network) listen() {
	var err error
	frames := make(chan ethernet.Frame)
	msgs := make(chan *pubsub.Message)

	go func() {
		for {
			msg, err := net.sub.Next(net.ctx)
			if err != nil {
				return
			}

			msgs <- msg
		}
	}()

	go func() {
		var frame ethernet.Frame
		for {
			select {
			case <-net.ctx.Done():
				return
			default:
			}

			frame.Resize(1500)
			n, err := net.inet.Read([]byte(frame))
			if err != nil {
				log.Println(err)
				return
			}

			frames <- frame[:n]
		}
	}()

	go func() {
		for {
			select {
			case frame := <-net.ingoing:
				_, err := net.inet.Write(frame)
				if err != nil {
					return
				}
			}
		}
	}()

	for {
		select {
		case frame := <-frames:
			dest := frame.Destination()
			if waterutil.IsBroadcast(dest) {
				for _, m := range net.members {
					m.outgoing <- frame
				}
			} else {
				m, ok := net.members[dest.String()]
				if !ok {
					continue
				}

				// TODO Better connection strategy
				if m.stream == nil {
					m.stream, err = net.host.NewStream(net.ctx, m.id, Protocol)
					if err != nil {
						err = net.host.Connect(net.ctx, peerstore.PeerInfo{
							ID: m.id,
							Addrs: []multiaddr.Multiaddr{
								m.info.addr,
							},
						})
						m.stream, err = net.host.NewStream(net.ctx, m.id, Protocol)
						if err != nil {
							continue
						}
					}

					go m.receive()
				}

				m.outgoing <- frame
			}
		case msg := <-msgs:
			info := &PeerInfo{}
			err := info.UnmarshalBinary(msg.Data)
			if err != nil {
				continue
				log.Println(err)
			}

			mac := info.mac.String()
			if _, ok := net.members[mac]; !ok {
				net.members[mac] = &networkMember{
					id:       msg.GetFrom(),
					info:     info,
					outgoing: make(chan ethernet.Frame, 32),
				}

				// TODO Not a best decision
				err = net.announceSelf()
				if err != nil {
					log.Println(err)
				}
			}
		case <-net.ctx.Done():
			net.sub.Cancel()

			for _, m := range net.members {
				m.Close()
			}
		}
	}
}

func (net *network) announceSelf() error {
	selfB, err := net.self.MarshalBinary()
	if err != nil {
		return err
	}

	err = net.ps.Publish(net.name, selfB)
	if err != nil {
		return err
	}

	return nil
}

func (net *network) handle(s inet.Stream) {
	for {
		go func() {
			var frame ethernet.Frame
			for {
				select {
				case <-net.ctx.Done():
					return
				default:
				}

				frame.Resize(1500)
				n, err := s.Read([]byte(frame))
				if err != nil {
					log.Println(err)
					return
				}

				net.ingoing <- frame[:n]
			}
		}()
	}
}

func (net *network) Close() error {
	net.cancel()
	return nil
}

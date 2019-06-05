package main

import (
	"context"
	"flag"
	"fmt"
	vnet "github.com/Wondertan/go-libp2p-vnet"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/p2p/discovery"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/Wondertan/go-libp2p-vnet/tap"
)

var listen = flag.String("listen", "", "")
var dial = flag.String("dial", "", "")

func main() {
	rendezvous := "test"

	flag.Parse()

	ctx := context.Background()

	host, err := libp2p.New(ctx,
		libp2p.ListenAddrStrings(*listen),
	)
	if err != nil {
		panic(err)
	}

	ser, err := discovery.NewMdnsService(ctx, host, 10*time.Second, rendezvous)
	if err != nil {
		panic(err)
	}

	ps, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		panic(err)
	}

	tap, err := tap.NewTAPInterface()
	if err != nil {
		panic(err)
	}

	net, err := vnet.NewVirtualNetwork(ctx, rendezvous, host, ps, tap)
	if err != nil {
		panic(err)
	}

	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt)

	go func() {
		select {
		case <-ch:
			fmt.Println("Closing...")
			ser.Close()
			net.Close()
			tap.Close()
			os.Exit(0)
		}
	}()

	select {}
}

type notifee struct {
	host.Host

	ctx context.Context
}

func (n *notifee) HandlePeerFound(p peer.AddrInfo) {
	log.Printf("Found peer: %s", p)
	log.Println(n.Connect(n.ctx, p))
}

func NewNotifee(ctx context.Context, host host.Host) discovery.Notifee {
	return &notifee{ctx: ctx, Host: host}
}

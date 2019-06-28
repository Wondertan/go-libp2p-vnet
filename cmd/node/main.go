package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/routing"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	golog "github.com/whyrusleeping/go-logging"

	vnet "github.com/Wondertan/go-libp2p-vnet"
	"github.com/Wondertan/go-libp2p-vnet/tap"
)

var (
	iAddr      = flag.String("interface", "10.0.0.10/24", "")
	listenAddr = flag.String("listen", "", "")
	rendezvous = flag.String("rendezvous", "vnet", "")
	key        = flag.String("key", "", "")
)

func main() {
	golog.SetLevel(golog.DEBUG, "vnet")

	err := run(context.Background())
	if err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	flag.Parse()

	var opts []libp2p.Option
	var dhtRouter *dht.IpfsDHT

	if *key == "" {
		key, _, _ := crypto.GenerateEd25519Key(rand.Reader)
		strKey, err := keyToString(key)
		if err != nil {
			return err
		}

		log.Printf("New identity generated: %s", strKey)
		opts = append(opts, libp2p.Identity(key))
	} else {
		key, err := keyFromString(*key)
		if err != nil {
			return err
		}

		opts = append(opts, libp2p.Identity(key))
	}

	if *listenAddr != "" {
		opts = append(opts, libp2p.ListenAddrStrings(*listenAddr))
	}

	opts = append(opts,
		libp2p.Routing(
			func(h host.Host) (_ routing.PeerRouting, err error) {
				dhtRouter, err = dht.New(ctx, h)
				return dhtRouter, err
			},
		),
		libp2p.DisableRelay(),
	)

	rhost, err := libp2p.New(ctx, opts...)
	if err != nil {
		return err
	}

	bootstrap(ctx, rhost)

	err = dhtRouter.Bootstrap(ctx)
	if err != nil {
		return err
	}

	vi, err := tap.NewTAPInterface(ctx, tap.Address(*iAddr))
	if err != nil {
		return err
	}

	net, err := vnet.NewVirtualNetwork(ctx, *rendezvous, rhost, vi, discovery.NewRoutingDiscovery(dhtRouter))
	if err != nil {
		return err
	}

	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt)

	go func() {
		<-ch
		fmt.Println("Closing...")

		net.Close()
		vi.Close()
		dhtRouter.Close()
		rhost.Close()

		os.Exit(0)
	}()

	select {}
}

func keyToString(key crypto.PrivKey) (string, error) {
	b, err := key.Bytes()
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}

func keyFromString(key string) (crypto.PrivKey, error) {
	b, err := hex.DecodeString(key)
	if err != nil {
		return nil, err
	}

	return crypto.UnmarshalPrivateKey(b)
}

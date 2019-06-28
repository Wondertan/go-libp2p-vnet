package tap

import (
	"context"
	"fmt"
	"net"
	"os/exec"

	"github.com/gyf304/water"

	vnet "github.com/Wondertan/go-libp2p-vnet"
)

var inetName = "tap"
var inetCount int

type tapInterface struct {
	*water.Interface

	mac net.HardwareAddr
}

// NewTAPInterface creates new TAP interface
// Needs TunTapOSXDriver to be installed
func NewTAPInterface(ctx context.Context, opts ...Option) (vnet.VirtualNetworkInterface, error) {
	def := &tapCfg{}

	for _, opt := range opts {
		opt(def)
	}

	// not allow to name interface by user
	var interfaceName string

	// TODO Probably there is a better solution
	for {
		interfaceName = fmt.Sprint(inetName, inetCount)
		_, err := net.InterfaceByName(interfaceName)
		if err != nil {
			break
		}

		inetCount++
	}

	tap, err := water.New(water.Config{
		DeviceType: water.TAP,
		PlatformSpecificParams: water.PlatformSpecificParams{
			Driver: water.TunTapOSXDriver,
			Name:   interfaceName,
		},
	})
	if err != nil {
		return nil, err
	}

	inet, err := net.InterfaceByName(interfaceName)
	if err != nil {
		return nil, err
	}

	inetCount++

	if def.Address != "" {
		cmd := exec.Command("ifconfig", inet.Name, def.Address)
		err = cmd.Run()
		if err != nil {
			return nil, err
		}
	}

	go func() {
		<-ctx.Done()
		tap.Close()
	}()

	return &tapInterface{
		Interface: tap,
		mac:       inet.HardwareAddr,
	}, nil
}

func (t *tapInterface) MAC() net.HardwareAddr {
	return t.mac
}

type tapCfg struct {
	Address string
}

type Option func(cfg *tapCfg)

// Address ties the provided address to the created tap interface
func Address(ip string) Option {
	return func(cfg *tapCfg) {
		if cfg.Address == "" {
			cfg.Address = ip
		}
	}
}

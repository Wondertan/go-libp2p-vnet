package tap

import (
	"fmt"
	"net"

	vnet "github.com/Wondertan/go-libp2p-vnet"
	"github.com/gyf304/water"
)

var inetName = "tap"
var inetCount int

type tapInterface struct {
	*water.Interface

	mac net.HardwareAddr
}

// NewTAPInterface creates new TAP interface
// Needs TunTapOSXDriver to be installed
func NewTAPInterface() (vnet.VirtualNetworkInterface, error) {
	// not allow to name interface by user
	interfaceName := fmt.Sprint(inetName, inetCount)

	inetCount++

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

	return &tapInterface{
		Interface: tap,
		mac:       inet.HardwareAddr,
	}, nil
}

func (t *tapInterface) MAC() net.HardwareAddr {
	return t.mac
}

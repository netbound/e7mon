package net

import (
	"fmt"
	"net"

	"github.com/google/gopacket/pcap"
	"github.com/mdlayher/arp"
)

func getInterface(ifaceName string) (*pcap.Interface, error) {
	devs, err := pcap.FindAllDevs()
	if err != nil {
		return nil, err
	}

	if ifaceName != "" {
		for _, dev := range devs {
			// UP and RUNNING
			if dev.Name == ifaceName {
				return &dev, nil
			}
		}
	}

	// We want a device which is UP and RUNNING, pcap allows for more detailed flags
	for _, dev := range devs {
		// UP and RUNNING
		if dev.Flags == 22 {
			return &dev, nil
		}
	}

	return nil, fmt.Errorf("no suitable interface found, please specify one in the config")
}

func getInterfaceAddress(iface *pcap.Interface) *pcap.InterfaceAddress {
	for _, addr := range iface.Addresses {
		if addr.IP.To4() != nil {
			return &addr
		}
	}
	return nil
}

func getGatewayMAC(iface *pcap.InterfaceAddress, dev *net.Interface) (net.HardwareAddr, error) {
	cl, err := arp.Dial(dev)
	if err != nil {
		return nil, err
	}

	// Get default gateway
	network := iface.IP.Mask(iface.Netmask)
	gw := network.To4()
	gw[3]++
	dstMAC, err := cl.Resolve(gw)
	if err != nil {
		return nil, err
	}

	return dstMAC, nil
}

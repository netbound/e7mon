package net

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/mdlayher/arp"
)

type Scanner struct {
	InterfaceName     string
	InterfaceAddresss *pcap.InterfaceAddress
	Device            *net.Interface
	GatewayMAC        net.HardwareAddr
}

// Sources:
// * https://github.com/google/gopacket/blob/master/examples/synscan/main.go
// * https://github.com/v-byte-cpu/sx/blob/master/pkg/scan/tcp/tcp.go

func NewScanner(ifaceName string) *Scanner {
	i, err := getInterface(ifaceName)
	if err != nil {
		log.Fatal(err)
	}

	dev, err := net.InterfaceByName(i.Name)
	if err != nil {
		log.Fatal(err)
	}

	iAddr := getInterfaceAddress(i)

	dstMAC, err := getGatewayMAC(iAddr, dev)
	if err != nil {
		log.Fatal(err)
	}

	return &Scanner{
		InterfaceName:     ifaceName,
		InterfaceAddresss: iAddr,
		Device:            dev,
		GatewayMAC:        dstMAC,
	}
}

// StartLatencyScan starts scanning the addresses provided in the format of "ip:port".
func (s Scanner) StartLatencyScan(addresses []string) error {
	handle, err := pcap.OpenLive(s.InterfaceName, 65535, false, time.Second*5)
	if err != nil {
		return err
	}
	defer handle.Close()

	// TCP SYN-ACK BPF filter
	var filter = "tcp[13] = 18"
	err = handle.SetBPFFilter(filter)
	if err != nil {
		return err
	}

	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())

	for _, addr := range addresses {
		x := strings.Split(addr, ":")
		i, _ := strconv.Atoi(x[1])
		dst := net.ParseIP(x[0])

		start, err := s.sendSYNPacket(handle, x[0], uint16(i))
		if err != nil {
			return err
		}
		startListener(packetSource, dst, uint16(i), *start)
	}
	return nil
}

func (s Scanner) sendSYNPacket(handle *pcap.Handle, address string, port uint16) (*time.Time, error) {
	dst := net.ParseIP(address)

	outPkt, err := s.buildRawSYNPacket(dst, port)
	if err != nil {
		return nil, err
	}

	// Send our packet
	err = handle.WritePacketData(outPkt)
	start := time.Now()
	if err != nil {
		return nil, err
	}

	return &start, nil
}

func startListener(src *gopacket.PacketSource, dst net.IP, port uint16, start time.Time) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for packet := range src.Packets() {
			ip := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
			fmt.Printf("Source IP: %s\n", ip.SrcIP)
			fmt.Printf("Dest IP: %s\n", dst)
			if ip.SrcIP.Equal(dst) {
				tcp := packet.Layer(layers.LayerTypeTCP).(*layers.TCP)
				fmt.Printf("Source port: %s\n", tcp.SrcPort)
				if uint16(tcp.SrcPort) == port {
					fmt.Println("YES")
					fmt.Println(time.Since(start))
				}
			}
		}
	}()

	wg.Wait()
}

func (s Scanner) buildRawSYNPacket(dst net.IP, dstPort uint16) ([]byte, error) {
	buffer := gopacket.NewSerializeBuffer()

	ether := &layers.Ethernet{
		SrcMAC:       s.Device.HardwareAddr,
		DstMAC:       s.GatewayMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		TTL:      64,
		SrcIP:    s.InterfaceAddresss.IP,
		DstIP:    dst,
		Protocol: layers.IPProtocolTCP,
		Flags:    layers.IPv4DontFragment,
	}

	tcp := &layers.TCP{
		SYN:     true,
		DstPort: layers.TCPPort(dstPort),
	}

	if err := tcp.SetNetworkLayerForChecksum(ip); err != nil {
		return nil, err
	}

	gopacket.SerializeLayers(buffer,
		gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true},
		ether, ip, tcp)

	return buffer.Bytes(), nil
}

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

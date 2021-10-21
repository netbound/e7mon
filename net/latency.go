package net

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

type Scanner struct {
	InterfaceName    string
	InterfaceAddress *pcap.InterfaceAddress
	Device           *net.Interface
	Handle           *pcap.Handle
	GatewayMAC       net.HardwareAddr
	Results          []LatencyResult
}

type LatencyResult struct {
	Host    string
	Latency time.Duration
}

type RSTSettings struct {
	DstIP   net.IP
	SrcPort layers.TCPPort
	DstPort layers.TCPPort
	Seq     uint32
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
		InterfaceName:    ifaceName,
		InterfaceAddress: iAddr,
		Device:           dev,
		GatewayMAC:       dstMAC,
	}
}

// StartLatencyScan starts scanning the addresses provided in the format of "ip:port".
func (s *Scanner) StartLatencyScan(addresses []string) ([]LatencyResult, error) {
	var err error
	s.Handle, err = pcap.OpenLive(s.InterfaceName, 65535, false, -time.Millisecond)
	if err != nil {
		return nil, err
	}

	// This takes long...
	// defer s.Handle.Close()

	// TCP SYN-ACK BPF filter
	var filter = "tcp[13] = 18"
	err = s.Handle.SetBPFFilter(filter)
	if err != nil {
		return nil, err
	}

	// This channel will receive RST settings after receiving a SYN-ACK,
	// to break down the connection.
	reset := make(chan RSTSettings)

	packetSource := gopacket.NewPacketSource(s.Handle, s.Handle.LinkType())

	for _, addr := range addresses {
		dst, port := parseDestination(addr)

		// Send TCP SYN packet for every address
		pkt, err := s.buildSYNPacket(dst, port)
		if err != nil {
			return nil, err
		}

		start, err := s.sendSYNPacket(pkt)
		if err != nil {
			return nil, err
		}

		// Spawn a listener for every packet sent
		go s.startListener(packetSource, addr, start, reset)
	}

	ctr := 0
	for rstSettings := range reset {
		fmt.Println(rstSettings)

		rst, err := s.buildRawRSTPacket(rstSettings)
		if err != nil {
			log.Fatal(err)
		}

		err = s.sendRSTPacket(rst)
		if err != nil {
			log.Fatal(err)
		}

		ctr++
		fmt.Println(ctr >= len(addresses))
		if ctr >= len(addresses) {
			break
		}
	}

	return s.Results, nil
}

func parseDestination(dstString string) (address net.IP, port uint16) {
	x := strings.Split(dstString, ":")
	i, _ := strconv.Atoi(x[1])
	port = uint16(i)
	address = net.ParseIP(x[0])
	return
}

func (s *Scanner) startListener(src *gopacket.PacketSource, host string, start time.Time, c chan<- RSTSettings) {
	dst, port := parseDestination(host)
	for packet := range src.Packets() {
		ip := packet.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
		if ip.SrcIP.Equal(dst) {
			tcp := packet.Layer(layers.LayerTypeTCP).(*layers.TCP)
			// Don't want to detect other packets
			if uint16(tcp.SrcPort) == port && tcp.Ack == 1 {
				c <- RSTSettings{
					DstIP:   dst,
					SrcPort: tcp.DstPort,
					DstPort: tcp.SrcPort,
					Seq:     tcp.Ack + 1,
				}

				s.Results = append(s.Results, LatencyResult{
					Host:    host,
					Latency: time.Since(start),
				})
			}
		}
	}
}

func (s *Scanner) sendSYNPacket(pkt []byte) (time.Time, error) {
	// Send our packet
	err := s.Handle.WritePacketData(pkt)
	start := time.Now()
	if err != nil {
		return start, err
	}

	return start, nil
}

func (s *Scanner) buildSYNPacket(dst net.IP, dstPort uint16) ([]byte, error) {
	buffer := gopacket.NewSerializeBuffer()

	ether := &layers.Ethernet{
		SrcMAC:       s.Device.HardwareAddr,
		DstMAC:       s.GatewayMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		TTL:      64,
		SrcIP:    s.InterfaceAddress.IP,
		DstIP:    dst,
		Protocol: layers.IPProtocolTCP,
		Flags:    layers.IPv4DontFragment,
	}

	tcp := &layers.TCP{
		SYN:     true,
		Window:  65535,
		SrcPort: layers.TCPPort(32768 + rand.Intn(61000-32768)),
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

func (s *Scanner) sendRSTPacket(pkt []byte) error {
	// Send our packet
	err := s.Handle.WritePacketData(pkt)
	if err != nil {
		return err
	}

	return nil
}

func (s *Scanner) buildRawRSTPacket(rst RSTSettings) ([]byte, error) {
	buffer := gopacket.NewSerializeBuffer()

	ether := &layers.Ethernet{
		SrcMAC:       s.Device.HardwareAddr,
		DstMAC:       s.GatewayMAC,
		EthernetType: layers.EthernetTypeIPv4,
	}

	ip := &layers.IPv4{
		Version:  4,
		TTL:      64,
		SrcIP:    s.InterfaceAddress.IP,
		DstIP:    rst.DstIP,
		Protocol: layers.IPProtocolTCP,
		Flags:    layers.IPv4DontFragment,
	}

	tcp := &layers.TCP{
		RST:     true,
		Seq:     rst.Seq,
		SrcPort: rst.SrcPort,
		DstPort: rst.DstPort,
	}

	if err := tcp.SetNetworkLayerForChecksum(ip); err != nil {
		return nil, err
	}

	gopacket.SerializeLayers(buffer,
		gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true},
		ether, ip, tcp)

	return buffer.Bytes(), nil
}

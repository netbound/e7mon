package net

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
)

// TODO: add mutex, we're concurrently writing to map which can cause panic
// OR: send results over channel back to main goroutine where we can write to map,
// -> no locks needed
type Results map[string]time.Duration

func (r *Results) Reset() {
	*r = make(map[string]time.Duration)
}

var results Results = make(map[string]time.Duration)
var mu sync.Mutex

type Scanner struct {
	InterfaceName    string
	InterfaceAddress *pcap.InterfaceAddress
	Device           *net.Interface
	Handle           *pcap.Handle
	GatewayMAC       net.HardwareAddr
}

type LatencyResult struct {
	Host    string
	Latency time.Duration
}

type RSTSettings struct {
	// Reset caused by timeout
	Timeout bool
	DstIP   net.IP
	SrcPort layers.TCPPort
	DstPort layers.TCPPort
	Seq     uint32
}

type ListenParams struct {
	start   time.Time
	DstPort layers.TCPPort
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
		InterfaceName:    dev.Name,
		InterfaceAddress: iAddr,
		Device:           dev,
		GatewayMAC:       dstMAC,
	}
}

// StartLatencyScan starts scanning the addresses provided in the format of "ip:port".
func (s *Scanner) StartLatencyScan(hosts []string) (map[string]time.Duration, error) {
	// Clear results, we don't want to keep old peers
	results.Reset()
	var err error
	s.Handle, err = pcap.OpenLive(s.InterfaceName, 65535, false, pcap.BlockForever)
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

	flows := make(chan ListenParams)
	// This channel will receive RST settings after receiving a SYN-ACK,
	// to break down the connection.
	reset := make(chan RSTSettings)

	packetSource := gopacket.NewPacketSource(s.Handle, s.Handle.LinkType())
	ctr := 0

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Start listening already
	go s.startListener(ctx, packetSource, flows, reset)

	for _, host := range hosts {
		dst, port, err := net.SplitHostPort(host)
		if err != nil {
			return nil, err
		}

		p, _ := strconv.Atoi(port)

		// Send TCP SYN packet for every address
		pkt, dstport, err := s.buildSYNPacket(net.ParseIP(dst), uint16(p))
		if err != nil {
			return nil, err
		}

		start, err := s.sendSYNPacket(pkt)
		if err != nil {
			return nil, err
		}

		flows <- ListenParams{
			start:   start,
			DstPort: dstport,
		}
	}

	for rstSettings := range reset {
		// Not expired by timeout
		if !rstSettings.Timeout {
			rst, err := s.buildRawRSTPacket(rstSettings)
			if err != nil {
				log.Fatal(err)
			}

			err = s.sendRSTPacket(rst)
			if err != nil {
				log.Fatal(err)
			}
		}

		ctr++
		if ctr >= len(hosts) {
			break
		}
	}

	return results, nil
}

func (s *Scanner) startListener(ctx context.Context, src *gopacket.PacketSource, flows <-chan ListenParams, rst chan<- RSTSettings) {
	// TODO: fix concurrent read/write panic with mutex
	targetFlows := make(map[layers.TCPPort]time.Time)
	var (
		ethLayer layers.Ethernet
		ip       layers.IPv4
		tcp      layers.TCP

		decoded = []gopacket.LayerType{}
	)

	parser := gopacket.NewDecodingLayerParser(
		layers.LayerTypeEthernet,
		&ethLayer,
		&ip,
		&tcp,
	)

	go func() {
		for packet := range src.Packets() {
			parser.DecodeLayers(packet.Data(), &decoded)
			if start, ok := targetFlows[tcp.DstPort]; ok {
				// Possible concurrent writes to map here
				mu.Lock()
				results[fmt.Sprintf("%s:%d", ip.SrcIP, tcp.SrcPort)] = packet.Metadata().Timestamp.Sub(start)
				mu.Unlock()
				rst <- RSTSettings{
					Timeout: false,
					DstIP:   ip.SrcIP,
					SrcPort: tcp.DstPort,
					DstPort: tcp.SrcPort,
					Seq:     tcp.Ack + 1,
				}
			}
		}
	}()

	for {
		select {
		case flow := <-flows:
			targetFlows[flow.DstPort] = flow.start
		case <-ctx.Done():
			rst <- RSTSettings{
				Timeout: true,
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

func (s *Scanner) buildSYNPacket(dst net.IP, dstPort uint16) ([]byte, layers.TCPPort, error) {
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
		return nil, 0, err
	}

	gopacket.SerializeLayers(buffer,
		gopacket.SerializeOptions{FixLengths: true, ComputeChecksums: true},
		ether, ip, tcp)

	return buffer.Bytes(), tcp.SrcPort, nil
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

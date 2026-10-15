package netstack

import (
	"bytes"
	"testing"

	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/checksum"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/link/channel"
	"gvisor.dev/gvisor/pkg/tcpip/network/ipv4"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
	"gvisor.dev/gvisor/pkg/tcpip/transport/icmp"
	"gvisor.dev/gvisor/pkg/tcpip/transport/raw"
	"gvisor.dev/gvisor/pkg/waiter"
)

func TestICMPHandlingDeliversRawPacketWithoutAutomaticReply(t *testing.T) {
	ns := stack.New(stack.Options{
		NetworkProtocols:   []stack.NetworkProtocolFactory{ipv4.NewProtocol},
		TransportProtocols: []stack.TransportProtocolFactory{icmp.NewProtocol4},
	})
	defer ns.Close()
	configureICMPHandling(ns)

	linkEP := channel.New(1, 1500, "")
	defer linkEP.Close()
	if err := ns.CreateNIC(1, linkEP); err != nil {
		t.Fatalf("CreateNIC: %v", err)
	}

	localAddress := tcpip.AddrFrom4([4]byte{192, 0, 2, 10})
	remoteAddress := tcpip.AddrFrom4([4]byte{198, 51, 100, 20})
	if err := ns.AddProtocolAddress(1, tcpip.ProtocolAddress{
		Protocol:          ipv4.ProtocolNumber,
		AddressWithPrefix: localAddress.WithPrefix(),
	}, stack.AddressProperties{}); err != nil {
		t.Fatalf("AddProtocolAddress: %v", err)
	}
	ns.SetRouteTable([]tcpip.Route{{Destination: header.IPv4EmptySubnet, NIC: 1}})

	var wq waiter.Queue
	rawEP, err := raw.NewEndpoint(ns, ipv4.ProtocolNumber, icmp.ProtocolNumber4, &wq)
	if err != nil {
		t.Fatalf("raw.NewEndpoint: %v", err)
	}
	defer rawEP.Close()
	if err := rawEP.Bind(tcpip.FullAddress{}); err != nil {
		t.Fatalf("raw endpoint Bind: %v", err)
	}

	packet := make([]byte, header.IPv4MinimumSize+header.ICMPv4MinimumSize)
	ipHeader := header.IPv4(packet)
	ipHeader.Encode(&header.IPv4Fields{
		TotalLength: uint16(len(packet)),
		TTL:         64,
		Protocol:    uint8(icmp.ProtocolNumber4),
		SrcAddr:     remoteAddress,
		DstAddr:     localAddress,
	})
	icmpHeader := header.ICMPv4(packet[header.IPv4MinimumSize:])
	icmpHeader.SetType(header.ICMPv4Echo)
	icmpHeader.SetCode(header.ICMPv4UnusedCode)
	icmpHeader.SetIdent(1)
	icmpHeader.SetSequence(1)
	icmpHeader.SetChecksum(^checksum.Checksum(icmpHeader, 0))
	ipHeader.SetChecksum(^ipHeader.CalculateChecksum())

	pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{Payload: buffer.MakeWithData(packet)})
	defer pkt.DecRef()
	linkEP.InjectInbound(ipv4.ProtocolNumber, pkt)

	var delivered bytes.Buffer
	if _, err := rawEP.Read(&delivered, tcpip.ReadOptions{}); err != nil {
		t.Fatalf("raw endpoint Read: %v", err)
	}
	if delivered.Len() != len(packet) {
		t.Fatalf("raw endpoint received %d bytes, want %d", delivered.Len(), len(packet))
	}

	if reply := linkEP.Read(); reply != nil {
		defer reply.DecRef()
		t.Fatal("gVisor emitted an automatic ICMP echo reply")
	}
}

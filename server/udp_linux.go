package server

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

var nativeEndian binary.ByteOrder

func init() {
	buf := [2]byte{}
	*(*uint16)(unsafe.Pointer(&buf[0])) = uint16(0xABCD)

	switch buf {
	case [2]byte{0xCD, 0xAB}:
		nativeEndian = binary.LittleEndian
	case [2]byte{0xAB, 0xCD}:
		nativeEndian = binary.BigEndian
	default:
		panic("Could not determine native endianness.")
	}
}

const (
	IPV6_TRANSPARENT     = 75
	IPV6_RECVORIGDSTADDR = 74
)

// readFromUDP reads a UDP packet from c, copying the payload into b.
// It returns the number of bytes copied into b and the return address
// that was on the packet.
//
// Out-of-band data is also read in so that the original destination
// address can be identified and parsed.
func readFromUDP(conn *net.UDPConn, b []byte) (int, *net.UDPAddr, *net.UDPAddr, error) {
	oob := make([]byte, 1024)
	n, oobn, _, addr, err := conn.ReadMsgUDP(b, oob)
	if err != nil {
		return 0, nil, nil, err
	}

	msgs, err := syscall.ParseSocketControlMessage(oob[:oobn])
	if err != nil {
		return 0, nil, nil, fmt.Errorf("parsing socket control message: %s", err)
	}

	ntohs := func(n uint16) uint16 {
		return (n >> 8) | (n << 8)
	}

	var originalDst *net.UDPAddr
	for _, msg := range msgs {
		if msg.Header.Level == syscall.SOL_IP && msg.Header.Type == syscall.IP_RECVORIGDSTADDR {
			originalDstRaw := &syscall.RawSockaddrInet4{}
			if err = binary.Read(bytes.NewReader(msg.Data), nativeEndian, originalDstRaw); err != nil {
				return 0, nil, nil, fmt.Errorf("reading original destination address: %s", err)
			}
			originalDst = &net.UDPAddr{
				IP:   net.IPv4(originalDstRaw.Addr[0], originalDstRaw.Addr[1], originalDstRaw.Addr[2], originalDstRaw.Addr[3]),
				Port: int(ntohs(originalDstRaw.Port)),
			}
		} else if msg.Header.Level == syscall.SOL_IPV6 && msg.Header.Type == IPV6_RECVORIGDSTADDR {
			originalDstRaw := &syscall.RawSockaddrInet6{}
			if err = binary.Read(bytes.NewReader(msg.Data), nativeEndian, originalDstRaw); err != nil {
				return 0, nil, nil, fmt.Errorf("reading original destination address: %s", err)
			}
			originalDst = &net.UDPAddr{
				IP:   originalDstRaw.Addr[:],
				Port: int(ntohs(originalDstRaw.Port)),
				Zone: strconv.Itoa(int(originalDstRaw.Scope_id)),
			}
		}
	}

	if originalDst == nil {
		return 0, nil, nil, fmt.Errorf("unable to obtain original destination: %s", err)
	}

	return n, addr, originalDst, nil
}

type rawUDPConn struct {
	conn net.PacketConn
}

var (
	ErrUnsupportedAF     = errors.New("unsupported address family")
	ErrUnsupportedMethod = errors.New("unsupported method")
)

func newRawUDPConn(network string) (*rawUDPConn, error) {
	switch network {
	case "udp4":
	default:
		return nil, ErrUnsupportedAF
	}
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW)
	if err != nil {
		return nil, fmt.Errorf("failed open socket(syscall.AF_INET, syscall.SOCK_RAW, syscall.IPPROTO_RAW): %s", err)
	}
	syscall.SetsockoptInt(fd, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1)

	conn, err := net.FilePacketConn(os.NewFile(uintptr(fd), fmt.Sprintf("fd %d", fd)))
	if err != nil {
		return nil, err
	}

	return &rawUDPConn{
		conn: conn,
	}, nil
}

func buildUDPPacket(b []byte, src, dst *net.UDPAddr) ([]byte, error) {
	buffer := gopacket.NewSerializeBuffer()
	payload := gopacket.Payload(b)
	ip := &layers.IPv4{
		DstIP:    dst.IP,
		SrcIP:    src.IP,
		Version:  4,
		TTL:      64,
		Protocol: layers.IPProtocolUDP,
	}
	udp := &layers.UDP{
		SrcPort: layers.UDPPort(src.Port),
		DstPort: layers.UDPPort(dst.Port),
	}
	if err := udp.SetNetworkLayerForChecksum(ip); err != nil {
		return nil, fmt.Errorf("failed calc checksum: %s", err)
	}
	if err := gopacket.SerializeLayers(buffer, gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}, ip, udp, payload); err != nil {
		return nil, fmt.Errorf("failed serialize packet: %s", err)
	}
	return buffer.Bytes(), nil
}

func (c *rawUDPConn) WriteFromTo(b []byte, from *net.UDPAddr, to *net.UDPAddr) (int, error) {
	b, err := buildUDPPacket(b, from, to)
	if err != nil {
		return 0, fmt.Errorf("can't build UDP packet: %w", err)
	}
	return c.conn.WriteTo(b, &net.IPAddr{IP: to.IP})
}

func (c *rawUDPConn) Close() error {
	return c.conn.Close()
}

func transparentDgramControlFunc(network, address string, conn syscall.RawConn) error {
	var operr error
	if err := conn.Control(func(fd uintptr) {
		level := syscall.SOL_IP
		transOptName := syscall.IP_TRANSPARENT
		origDstOptName := syscall.IP_RECVORIGDSTADDR
		switch network {
		case "tcp6", "udp6", "ip6":
			level = syscall.SOL_IPV6
			transOptName = IPV6_TRANSPARENT
			origDstOptName = IPV6_RECVORIGDSTADDR
		}

		operr = syscall.SetsockoptInt(int(fd), level, transOptName, 1)
		if operr != nil {
			return
		}
		operr = syscall.SetsockoptInt(int(fd), level, origDstOptName, 1)
	}); err != nil {
		return err
	}
	return operr
}


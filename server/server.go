package server

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"net/netip"

	"github.com/hashicorp/go-multierror"
)

type Server struct {
	receiver   *net.UDPConn
	sender     *rawUDPConn
	clientReq  []byte
	serverResp []byte
}

func New(ctx context.Context, cfg *Config) (*Server, error) {
	receiver, err := (&net.ListenConfig{
		Control: transparentDgramControlFunc,
	}).ListenPacket(ctx, "udp", cfg.BindAddress)
	if err != nil {
		return nil, fmt.Errorf("UDP recv socket bind failed: %w", err)
	}

	udpReceiver, ok := receiver.(*net.UDPConn)
	if !ok {
		return nil, fmt.Errorf("expected *net.UDPConn receiver but got %T", receiver)
	}

	sender, err := newRawUDPConn("udp4")
	if err != nil {
		return nil, fmt.Errorf("UDP send socket bind failed: %w", err)
	}

	s := &Server{
		receiver:   udpReceiver,
		sender:     sender,
		clientReq:  cfg.ClientReq,
		serverResp: cfg.ServerResp,
	}
	go s.listen()
	return s, nil
}

func (s *Server) listen() {
	defer s.Close()
	buf := make([]byte, 4096)
	for {
		read, from, to, err := readFromUDP(s.receiver, buf)
		if err != nil {
			log.Printf("udp recv failed: %v", err)
			break
		}
		from, to = unmapUDPAddr(from), unmapUDPAddr(to)
		pkt := buf[:read]
		log.Printf("got packet %q from %s to %s", string(pkt), from.String(), to.String())
		if bytes.Compare(pkt, s.clientReq) != 0 {
			continue
		}
		log.Printf("sending %q to %s from %s", string(s.serverResp), from.String(), to.String())
		_, err = s.sender.WriteFromTo(s.serverResp, to, from)
		if err != nil {
			log.Printf("send failed: %v", err)
		}
	}
}

func (s *Server) Close() error {
	var result error
	if err := s.receiver.Close(); err != nil {
		result = multierror.Append(result, err)
	}
	if err := s.sender.Close(); err != nil {
		result = multierror.Append(result, err)
	}
	return result
}

func unmapUDPAddr(a *net.UDPAddr) *net.UDPAddr {
	ap := a.AddrPort()
	apu := netip.AddrPortFrom(ap.Addr().Unmap(), ap.Port())
	return net.UDPAddrFromAddrPort(apu)
}

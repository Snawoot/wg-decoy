package server

import (
	"context"
	"fmt"
	"net"
)

type Server struct {
	receiver   *net.UDPConn
	sender     *rawUDPConn
	clientReq  []byte
	serverResp []byte
}

func New(ctx context.Context, cfg *Config) (*Server, error) {
	receiver, err := (&net.ListenConfig{}).ListenPacket(ctx, "udp", cfg.BindAddress)
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

	return &Server{
		receiver:   udpReceiver,
		sender:     sender,
		clientReq:  cfg.ClientReq,
		serverResp: cfg.ServerResp,
	}, nil
}

package client

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"time"
)

type Client struct {
	clientReq  []byte
	serverResp []byte
	conn       net.Conn
}

func Dial(ctx context.Context, cfg *Config) (*Client, error) {
	lAddr := &net.UDPAddr{
		Port: int(cfg.LocalPort),
	}
	dialer := &net.Dialer{
		LocalAddr: lAddr,
	}

	conn, err := dialer.DialContext(ctx, "udp", cfg.RemoteAddress)
	if err != nil {
		return nil, fmt.Errorf("client dial failed: %w", err)
	}

	return &Client{
		clientReq:  cfg.ClientReq,
		serverResp: cfg.ServerResp,
		conn:       conn,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Exchange(ctx context.Context) error {
	_, err := c.conn.Write(c.clientReq)
	if err != nil {
		return fmt.Errorf("request send failed: %w", err)
	}

	ctxWatchDone := make(chan struct{})
	mainDone := make(chan struct{})

	go func() {
		defer close(ctxWatchDone)
		select {
		case <-ctx.Done():
			if err := c.conn.SetReadDeadline(time.Unix(0, 0)); err != nil {
				log.Printf("can't set read deadline in past: %v", err)
			}
		case <-mainDone:
		}
	}()
	defer func() {
		<-ctxWatchDone
	}()

	buf := make([]byte, 4096)
	defer close(mainDone)
	for {
		if err := c.conn.SetReadDeadline(time.Time{}); err != nil {
			return fmt.Errorf("can't set read deadline in future: %w", err)
		}
		read, err := c.conn.Read(buf)
		if err != nil {
			return fmt.Errorf("client recv error: %w", err)
		}
		if bytes.Compare(buf[:read], c.serverResp) == 0 {
			return nil
		}
	}
}

//go:build !linux
// +build !linux

package server

import (
	"net"
	"syscall"
)

type rawUDPConn struct{}

func transparentDgramControlFunc(network, address string, conn syscall.RawConn) error {
	panic("not implemented")
}

func newRawUDPConn(network string) (*rawUDPConn, error) {
	panic("not implemented")
}

func (c *rawUDPConn) Close() error {
	panic("not implemented")
}

func (c *rawUDPConn) WriteFromTo(b []byte, from *net.UDPAddr, to *net.UDPAddr) (int, error) {
	panic("not implemented")
}

func readFromUDP(conn *net.UDPConn, b []byte) (int, *net.UDPAddr, *net.UDPAddr, error) {
	panic("not implemented")
}

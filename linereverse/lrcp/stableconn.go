package lrcp

import (
	"errors"
	"fmt"
	"net"
)

var ErrSessionNotOpen = errors.New("no open session on connection")

type (
	StableConn struct {
		udpConn *net.UDPConn
		session session
	}
)

func newStableConn(conn *net.UDPConn, remoteAddr *net.UDPAddr) *StableConn {
	return &StableConn{
		udpConn: conn,
		session: session{
			remoteAddr: remoteAddr,
		},
	}
}

func (sc *StableConn) Read(data []byte) (int, error) {
	return sc.session.buf.Read(data)
}

func (sc *StableConn) Write(data []byte) (int, error) {
	return sc.udpConn.Write(data)
}

func (sc *StableConn) sendAck(pos int) error {
	if sc.session.id == nil {
		return ErrSessionNotOpen
	}
	msg := fmt.Sprintf("/ack/%s/%d/", *sc.session.id, pos)
	n, err := sc.udpConn.WriteToUDP([]byte(msg), sc.session.remoteAddr)
	if err != nil {
		return fmt.Errorf("writeToUDP: %w", err)
	}
	if n != len(msg) {
		return fmt.Errorf("expected to write %d bytes, but wrote %d", len(msg), n)
	}
	return nil
}

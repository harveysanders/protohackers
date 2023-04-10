package lrcp

import (
	"bytes"
	"net"
	"time"
)

const (
	MsgConnect = "connect"
	MsgData    = "data"
	MsgAck     = "ack"
	MsgClose   = "close"
)

type (
	Listener struct {
		sessionExpiryTimeout  time.Duration
		retransmissionTimeout time.Duration
		localAddr             *net.UDPAddr
	}

	StableConn struct {
		udpConn *net.UDPConn
		session session
	}

	session struct {
		id  string
		buf bytes.Buffer
		pos int
	}
)

func Listen(port string) (*Listener, error) {
	localAddr, err := net.ResolveUDPAddr("udp", ":"+port)
	if err != nil {
		return nil, err
	}
	return &Listener{
		retransmissionTimeout: time.Second * 3,
		sessionExpiryTimeout:  time.Second * 60,
		localAddr:             localAddr,
	}, nil
}

func (l *Listener) Accept() (*StableConn, error) {
	conn, err := net.ListenUDP("udp", l.localAddr)
	if err != nil {
		return &StableConn{}, err
	}
	return &StableConn{
		udpConn: conn,
	}, nil
}

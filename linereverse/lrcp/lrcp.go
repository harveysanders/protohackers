package lrcp

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"time"
)

const (
	MsgConnect = "connect"
	MsgData    = "data"
	MsgAck     = "ack"
	MsgClose   = "close"
)

const (
	SectionMsgType sectionPos = iota
	SectionSessionID
)

type (
	Listener struct {
		sessionExpiryTimeout  time.Duration
		retransmissionTimeout time.Duration
		localAddr             net.Addr
		udpConn               *net.UDPConn
	}

	StableConn struct {
		udpConn *net.UDPConn
		session session
	}

	session struct {
		id         *string
		buf        bytes.Buffer
		pos        int // Current position in the overall stream of bytes.
		remoteAddr *net.UDPAddr
	}

	sectionPos int
)

func Listen(address string) (*Listener, error) {
	localAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("resolveUDPAddr: %w", err)
	}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("listenUDP: %w", err)
	}
	return &Listener{
		retransmissionTimeout: time.Second * 3,
		sessionExpiryTimeout:  time.Second * 60,
		localAddr:             conn.LocalAddr(),
		udpConn:               conn,
	}, nil
}

func (l *Listener) Accept() (*StableConn, error) {
	if err := l.handleConn(l.udpConn); err != nil {
		return nil, fmt.Errorf("handleConn: %w", err)
	}

	return &StableConn{}, nil
}

func (l *Listener) handleConn(c *net.UDPConn) error {
	firstByte := make([]byte, 1)
	n, rAddr, err := c.ReadFromUDP(firstByte)
	if err != nil {
		return fmt.Errorf("readFromUDP: %w", err)
	}

	log.Printf("read %d bytes..", n)

	if firstByte[0] != '/' {
		return fmt.Errorf("expected \"/\", but got: %q at pos 0", firstByte[0])
	}
	sess := session{
		remoteAddr: rAddr,
	}
	log.Println(sess)

	if err := c.SetReadDeadline(time.Now().Add(l.sessionExpiryTimeout)); err != nil {
		return fmt.Errorf("setReadDeadline: %w", err)
	}

	scr := bufio.NewScanner(c)
	scr.Split(ScanLRCPSection)

	msgParts := []string{}

	for scr.Scan() {
		if scr.Err() != nil {
			return fmt.Errorf("scan: %w", err)
		}
		part := scr.Text()
		msgParts = append(msgParts, part)
		log.Printf("msgParts: %+v", msgParts)
	}

	return nil
}

func (sc *StableConn) Read(data []byte) (int, error) {
	return sc.session.buf.Read(data)
}

func (sc *StableConn) Write(data []byte) (int, error) {
	return sc.udpConn.Write(data)
}

// ScanLRCPSection scans each section of an LRCP message, using "/" as the section delimiter. (Used bufio.ScanLines as example.)
func ScanLRCPSection(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}

	if i := bytes.IndexByte(data, '/'); i >= 0 {
		// We have a section.
		return i + 1, data[0:i], nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return len(data), data, nil
	}
	// Request more data.
	return 0, nil, nil
}

package lrcp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
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
		msgLengths            map[string]int
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
		msgLengths: map[string]int{
			string(MsgConnect): 2, // /connect/SESSION/
			string(MsgData):    4, // /data/SESSION/POS/DATA/
			string(MsgAck):     3, // /ack/SESSION/LENGTH/
			string(MsgClose):   2, // /close/SESSION/
		},
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

	if err := c.SetReadDeadline(time.Now().Add(l.sessionExpiryTimeout)); err != nil {
		return fmt.Errorf("setReadDeadline: %w", err)
	}

	sc := newStableConn(c, rAddr)

	scr := bufio.NewScanner(c)
	scr.Split(ScanLRCPSection)

	msgParts := []string{}

	for scr.Scan() {
		if scr.Err() != nil {
			return fmt.Errorf("scan: %w", err)
		}
		part := scr.Text()
		if part == "" {
			continue
		}

		msgParts = append(msgParts, part)
		log.Printf("msgParts: %+v", msgParts)

		// parse messages
		if len(msgParts) > 1 {
			msgType := msgParts[0]
			switch msgType {
			case MsgConnect:
				if len(msgParts) == l.msgLengths[MsgConnect] {
					// /connect/SESSION/
					sc.session.id = &msgParts[1]
					// Reset msg buffer
					msgParts = []string{}
					// Send ack
					if err := sc.sendAck(0); err != nil {
						return fmt.Errorf("send connect ack: %w", err)
					}
				}

			case MsgData:
				if len(msgParts) == l.msgLengths[MsgData] {
					log.Printf("%s: %+v", MsgData, msgParts)
					// /data/SESSION/POS/DATA/
					sessionID := msgParts[1]
					pos, err := strconv.Atoi(msgParts[2])
					if err != nil {
						return fmt.Errorf("parse data position: %w", err)
					}
					data := msgParts[3]
					// reset msgParts
					msgParts = []string{}

					log.Printf("sessionID: %s, pos: %d, data: %s", sessionID, pos, data)
					// If the session is not open: send /close/SESSION/ and stop.
					if sc.session.id == nil {
						sc.sendClose()
						return ErrSessionNotOpen
					}
					if sessionID != *sc.session.id {
						return fmt.Errorf("expected ID: %q, but got %q", *sc.session.id, sessionID)
					}

					// Check if recv everything so far
					if pos > sc.BytesRecvd() {
						// missing data. send prev ack
						if err := sc.sendAck(sc.BytesRecvd()); err != nil {
							log.Printf("sendAck: %v", err)
						}
						continue
					}
					if pos == sc.BytesRecvd() {
						// Write data to buffer at pos
						if _, err := sc.session.recvBuf.Write(unescapeSlashes([]byte(data))); err != nil {
							return fmt.Errorf("recvBuf.Write: %w", err)
						}
						if err := sc.sendAck(sc.BytesRecvd()); err != nil {
							log.Printf("sendAck: %v", err)
						}
						continue
					}
					// we already have more bytes than pos. Most likely a problem? Start over?
					log.Printf("POS: %d, BYTES RECV: %d", pos, sc.BytesRecvd())
					if err := sc.sendAck(sc.BytesRecvd()); err != nil {
						log.Printf("sendAck: %v", err)
					}
					sendRdr := bufio.NewReader(&sc.session.sendBuf)
					sendData, err := sendRdr.ReadBytes('\n')
					if err != nil {
						if err == io.EOF {
							continue
						}
						return fmt.Errorf("sendRdr.ReadBytes: %w", err)
					}
					sc.sendData(sendData, sc.session.bytesSent)
					continue
				}
			case MsgAck:
				if len(msgParts) == l.msgLengths[MsgAck] {
					// /ack/SESSION/LENGTH/
					log.Printf("%s: %+v", MsgAck, msgParts)
				}
			case MsgClose:
				if len(msgParts) == l.msgLengths[MsgClose] {
					// /close/SESSION/
					log.Printf("%s: %+v", MsgClose, msgParts)
				}
			}
		}
	}

	return nil
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

func unescapeSlashes(data []byte) []byte {
	out := bytes.ReplaceAll(data, []byte(`\/`), []byte(`/`))
	return bytes.ReplaceAll(out, []byte(`\\`), []byte(`\`))
}

func escapeSlashes(data []byte) []byte {
	out := bytes.ReplaceAll(data, []byte(`/`), []byte(`\/`))
	return bytes.ReplaceAll(out, []byte(`\`), []byte(`\\`))
}

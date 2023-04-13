package lrcp

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
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
		bufSize               int
		sessionExpiryTimeout  time.Duration
		retransmissionTimeout time.Duration
		localAddr             net.Addr
		udpConn               *net.UDPConn
		msgLengths            map[string]int

		mu       sync.Mutex
		sessions map[string]*StableConn
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
		bufSize:               1024,
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
		sessions: map[string]*StableConn{},
	}, nil
}

func (l *Listener) Close() error {
	return l.udpConn.Close()
}

func (l *Listener) Accept() (*StableConn, error) {
	buffer := make([]byte, l.bufSize)
	for {
		if err := l.udpConn.SetReadDeadline(time.Now().Add(l.sessionExpiryTimeout)); err != nil {
			return nil, fmt.Errorf("setReadDeadline: %w", err)
		}
		n, rAddr, err := l.udpConn.ReadFromUDP(buffer)
		if err != nil {
			// Keep reading even if there is an error on a specific read.
			log.Printf("readFrom: %v", err)
			continue
		}
		log.Printf("read %d bytes..", n)
		if n == 0 {
			continue
		}
		go l.handleConn(buffer[0:n], rAddr)
	}
}

func (l *Listener) handleConn(data []byte, remoteAddr net.Addr) error {
	scr := bufio.NewScanner(bytes.NewBuffer(data))
	scr.Split(ScanLRCPSection)

	var sc *StableConn
	msgParts := []string{}

	for scr.Scan() {
		if scr.Err() != nil {
			return fmt.Errorf("scan: %w", scr.Err())
		}
		part := scr.Text()
		if part == "" {
			continue
		}

		msgParts = append(msgParts, part)

		// parse messages
		if len(msgParts) > 1 {
			msgType := msgParts[0]
			switch msgType {
			case MsgConnect:
				if len(msgParts) == l.msgLengths[MsgConnect] {
					// /connect/SESSION/
					sessionID := msgParts[1]
					l.mu.Lock()
					if _, ok := l.sessions[sessionID]; !ok {
						l.sessions[sessionID] = &StableConn{
							sessionID:  &sessionID,
							remoteAddr: remoteAddr,
							udpConn:    *l.udpConn,
						}
					}
					sc = l.sessions[sessionID]

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
					if sc.sessionID == nil {
						sc.sendClose()
						return ErrSessionNotOpen
					}
					if sessionID != *sc.sessionID {
						return fmt.Errorf("expected ID: %q, but got %q", *sc.sessionID, sessionID)
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
						if _, err := sc.recvBuf.Write(unescapeSlashes([]byte(data))); err != nil {
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
					sendRdr := bufio.NewReader(&sc.sendBuf)
					sendData, err := sendRdr.ReadBytes('\n')
					if err != nil {
						if err == io.EOF {
							continue
						}
						return fmt.Errorf("sendRdr.ReadBytes: %w", err)
					}
					sc.sendData(sendData, sc.bytesSent)
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

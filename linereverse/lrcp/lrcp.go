package lrcp

import (
	"bufio"
	"bytes"
	"fmt"
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
		appIn                 chan SessionMsg
		appOut                chan SessionMsg

		mu       sync.Mutex
		sessions map[string]*StableConn
	}

	SessionMsg struct {
		ID   string
		Data []byte
	}

	sectionPos int
)

func Listen(address string, in, out chan SessionMsg) (*Listener, error) {
	localAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return nil, fmt.Errorf("resolveUDPAddr: %w", err)
	}

	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		return nil, fmt.Errorf("listenUDP: %w", err)
	}

	l := &Listener{
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
		appIn:    in,
		appOut:   out,
	}

	go l.outboundPump()

	return l, nil
}

func (l *Listener) Close() error {
	return l.udpConn.Close()
}

func (l *Listener) Accept() {
	buffer := make([]byte, l.bufSize)
	for {
		if err := l.udpConn.SetReadDeadline(time.Now().Add(l.sessionExpiryTimeout)); err != nil {
			log.Printf("setReadDeadline: %v", err)
			return
		}
		n, rAddr, err := l.udpConn.ReadFromUDP(buffer)
		if err != nil {
			// Keep reading even if there is an error on a specific read.
			log.Printf("readFrom: %v", err)
			continue
		}
		log.Printf("read %d bytes from: %s", n, rAddr.String())
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
						l.sessions[sessionID] = newStableConn(
							sessionID,
							remoteAddr,
							*l.udpConn,
						)
					}
					sc = l.sessions[sessionID]
					l.mu.Unlock()

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
					sc = l.lookupSession(sessionID)
					// If the session is not open: send /close/SESSION/ and stop.
					if sc == nil {
						if err := l.sendClose(remoteAddr, sessionID); err != nil {
							log.Printf("sendClose: %v", err)
						}
						return ErrSessionNotOpen
					}

					pos, err := strconv.Atoi(msgParts[2])
					if err != nil {
						return fmt.Errorf("parse data position: %w", err)
					}
					data := msgParts[3]
					// reset msgParts
					msgParts = []string{}

					log.Printf("sessionID: %s, pos: %d, data: %s", sessionID, pos, data)

					nextPos, err := sc.handleData([]byte(data), pos)
					if err != nil {
						return fmt.Errorf("handleData: %w", err)
					}

					if err := sc.sendAck(nextPos); err != nil {
						return fmt.Errorf("sendAck: %w", err)
					}

					// Send completed line off to app layer
					line, rest, isCompleteLine := bytes.Cut(sc.recvBuf, []byte("\n"))
					log.Printf("rest: %d", len(rest))
					if isCompleteLine {
						l.appIn <- SessionMsg{ID: *sc.sessionID, Data: line}
					}

				}
			case MsgAck:
				if len(msgParts) == l.msgLengths[MsgAck] {
					// /ack/SESSION/LENGTH/
					log.Printf("%s: %+v", MsgAck, msgParts)
				}
			case MsgClose:
				if len(msgParts) == l.msgLengths[MsgClose] {
					// /close/SESSION/
					sessionID := msgParts[1]
					sc := l.lookupSession(sessionID)
					if sc == nil {
						log.Print(ErrSessionNotOpen.Error())
					}
					if err := l.sendClose(remoteAddr, sessionID); err != nil {
						log.Printf("sendClose: %v", err)
					}
				}
			}
		}
	}

	return nil
}

func (l *Listener) lookupSession(id string) *StableConn {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.sessions[id]
}

func (l *Listener) sendClose(rAddr net.Addr, sessionID string) error {
	msg := fmt.Sprintf("/%s/%s/", MsgClose, sessionID)
	_, err := l.udpConn.WriteTo([]byte(msg), rAddr)
	return err
}

func (l *Listener) outboundPump() {
	for {
		msg := <-l.appOut
		escaped := escapeSlashes(msg.Data)
		sc := l.lookupSession(msg.ID)
		if sc == nil {
			log.Printf("%v: %q", ErrSessionNotOpen, msg.ID)
			continue
		}
		n, err := sc.sendData(escaped, 0)
		if err != nil {
			log.Printf("writeTo: %v", err)
			continue
		}
		log.Printf("responded with %d byte to %s", n, *sc.sessionID)
	}
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

func (l *Listener) lineListen(sc *StableConn) {
	scr := bufio.NewScanner(bytes.NewBuffer(sc.recvBuf))
	for scr.Scan() {
		err := scr.Err()
		if err != nil {
			log.Printf("scan: %v", err)
			return
		}
		line := scr.Text()
		l.appIn <- SessionMsg{ID: *sc.sessionID, Data: []byte(line)}
	}
}

func unescapeSlashes(data []byte) []byte {
	out := bytes.ReplaceAll(data, []byte(`\/`), []byte(`/`))
	return bytes.ReplaceAll(out, []byte(`\\`), []byte(`\`))
}

func escapeSlashes(data []byte) []byte {
	out := bytes.ReplaceAll(data, []byte(`/`), []byte(`\/`))
	return bytes.ReplaceAll(out, []byte(`\`), []byte(`\\`))
}

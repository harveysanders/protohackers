package lrcp

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
)

var (
	ErrSessionNotOpen   = errors.New("no open session on connection")
	ErrInvalidSessionID = errors.New("invalid session ID")
)

type (
	StableConn struct {
		sessionID  *string
		sendBuf    []byte   // Bytes to be sent to the remote address. Any slashes or backslashes have been escaped and prepared for the UDP network layer.
		recvBuf    []byte   // Bytes to be collected from the remote address. The contents' escaped slashes and backslashes have been unescaped and prepared for application layer.
		bytesSent  int      // Number of bytes acknowledged to have been successfully sent to the remote address.
		bytesRecvd int      // Number of unescaped bytes received from the client.
		remoteAddr net.Addr // Remote UDP Address.
		// TODO: Remove this connection
		udpConn net.UDPConn
	}
)

func newStableConn(id string, rAddr net.Addr, c net.UDPConn) *StableConn {
	return &StableConn{
		sessionID:  &id,
		recvBuf:    make([]byte, 4096),
		sendBuf:    make([]byte, 1024),
		remoteAddr: rAddr,
		udpConn:    c,
	}
}

// bytesRecvd returns the number of bytes received from the remote address.
func (sc *StableConn) BytesRecvd() int {
	return sc.bytesRecvd
}

// bytesSent returns the number of bytes acknowledged to have been successfully sent to the remote address.
func (sc *StableConn) BytesSent() int {
	// This value will not necessarily match the length of the send buffer.
	return sc.bytesSent
}

func (sc *StableConn) sendAck(pos int) error {
	if sc.sessionID == nil {
		return ErrSessionNotOpen
	}
	msg := fmt.Sprintf("/ack/%s/%d/", *sc.sessionID, pos)
	n, err := sc.udpConn.WriteTo([]byte(msg), sc.remoteAddr)
	if err != nil {
		return fmt.Errorf("writeTo: %w", err)
	}
	if n != len(msg) {
		return fmt.Errorf("expected to write %d bytes, but wrote %d", len(msg), n)
	}
	return nil
}

// handleData takes escaped data from the wire and appends it to the internal buffer at position pos. If pos is past the current buffer position, data is ignored and the current position is returned. The ASCII data is unescaped before inserted into the internal buffer.
func (sc *StableConn) handleData(rawData []byte, pos int) (nextPos int, err error) {
	// Don't create data gaps (yet)
	if sc.BytesRecvd() < pos {
		return sc.BytesRecvd(), nil
	}
	unescaped := unescapeSlashes(rawData)
	if sc.BytesRecvd() >= pos+len(unescaped) {
		// we have all this already
		return sc.BytesRecvd(), nil
	}
	// the incoming data is more than when currently have
	n := copy(sc.recvBuf[pos:], unescaped)
	sc.bytesRecvd += n
	if n != len(unescaped) {
		log.Printf("wrote %d to internal buffer. Should've written %d", n, len(unescaped))
		return sc.BytesRecvd(), io.ErrShortWrite
	}

	return sc.bytesRecvd, nil
}

// SendData writes data to the underlying UDP connection and returns the number of bytes written. curPos is the position in the overall session buffer before sending the data. The message header will look like this:
// /data/SESSION_ID/(curPos + len(data))/DATA/
func (sc *StableConn) sendData(data []byte, curPos int) (int, error) {
	if sc.sessionID == nil {
		return 0, ErrSessionNotOpen
	}
	msgHeader := fmt.Sprintf("/%s/%s/%d/", MsgData, *sc.sessionID, len(data)+curPos)
	msg := bytes.NewBufferString(msgHeader)
	if _, err := msg.Write(data); err != nil {
		return 0, fmt.Errorf("write to pre-transmission buffer: %w", err)
	}
	if _, err := msg.WriteRune('/'); err != nil {
		return 0, fmt.Errorf("writeRune: %w", err)
	}

	return sc.udpConn.WriteTo(msg.Bytes(), sc.remoteAddr)
}

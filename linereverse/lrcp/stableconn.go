package lrcp

import (
	"bytes"
	"errors"
	"fmt"
	"net"
)

var (
	ErrSessionNotOpen   = errors.New("no open session on connection")
	ErrInvalidSessionID = errors.New("invalid session ID")
)

type (
	StableConn struct {
		sessionID  *string
		sendBuf    bytes.Buffer // Bytes to be sent to the remote address.
		recvBuf    bytes.Buffer // Bytes to be collected from the remote address.
		bytesSent  int          // Number of bytes acknowledged to have been successfully sent to the remote address.
		remoteAddr net.Addr     // Remote UDP Address.
		// TODO: Remove this connection
		udpConn net.UDPConn
	}
)

// bytesRecvd returns the number of bytes received from the remote address.
func (sc *StableConn) BytesRecvd() int {
	return sc.recvBuf.Len()
}

// bytesSent returns the number of bytes acknowledged to have been successfully sent to the remote address.
func (sc *StableConn) BytesSent() int {
	// This value will not necessarily match the length of the send buffer.
	return sc.bytesSent
}

func (sc *StableConn) Read(data []byte) (int, error) {
	return sc.recvBuf.Read(data)
}

func (sc *StableConn) Write(data []byte) (int, error) {
	n, err := sc.sendData(data, sc.bytesSent)
	if err != nil {
		return n, err
	}
	sc.bytesSent += n
	return n, nil
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

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
		udpConn *net.UDPConn
		session session
	}

	session struct {
		id         *string      // Session ID.
		sendBuf    bytes.Buffer // Bytes to be sent to the remote address.
		recvBuf    bytes.Buffer // Bytes to be collected from the remote address.
		bytesSent  int          // Number of bytes acknowledged to have been successfully sent to the remote address.
		remoteAddr *net.UDPAddr // Remote UDP Address.
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

// bytesRecvd returns the number of bytes received from the remote address.
func (sc *StableConn) BytesRecvd() int {
	return sc.session.recvBuf.Len()
}

// bytesSent returns the number of bytes acknowledged to have been successfully sent to the remote address.
func (sc *StableConn) BytesSent() int {
	// This value will not necessarily match the length of the send buffer.
	return sc.session.bytesSent
}

func (sc *StableConn) Read(data []byte) (int, error) {
	return sc.session.recvBuf.Read(data)
}

func (sc *StableConn) Write(data []byte) (int, error) {
	n, err := sc.sendData(data, sc.session.bytesSent)
	if err != nil {
		return n, err
	}
	sc.session.bytesSent += n
	return n, nil
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

// SendData writes data to the underlying UDP connection and returns the number of bytes written. curPos is the position in the overall session buffer before sending the data. The message header will look like this:
// /data/SESSION_ID/(curPos + len(data))/DATA/
func (sc *StableConn) sendData(data []byte, curPos int) (int, error) {
	if sc.session.id == nil {
		return 0, ErrSessionNotOpen
	}
	msgHeader := fmt.Sprintf("/%s/%s/%d/", MsgData, *sc.session.id, len(data)+curPos)
	msg := bytes.NewBufferString(msgHeader)
	if _, err := msg.Write(data); err != nil {
		return 0, fmt.Errorf("write to pre-transmission buffer: %w", err)
	}
	if _, err := msg.WriteRune('/'); err != nil {
		return 0, fmt.Errorf("writeRune: %w", err)
	}

	return sc.udpConn.WriteToUDP(msg.Bytes(), sc.session.remoteAddr)
}

func (sc *StableConn) sendClose() error {
	msg := fmt.Sprintf("/%s/%s/", MsgClose, *sc.session.id)
	_, err := sc.udpConn.WriteToUDP([]byte(msg), sc.session.remoteAddr)
	return err
}

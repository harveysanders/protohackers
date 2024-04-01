package proto

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type MsgType byte

const (
	MsgTypeHello             MsgType = 0x50
	MsgTypeError             MsgType = 0x51
	MsgTypeOK                MsgType = 0x52
	MsgTypeDialAuthority     MsgType = 0x53
	MsgTypeTargetPopulations MsgType = 0x54
	MsgTypeCreatePolicy      MsgType = 0x55
	MsgTypeDeletePolicy      MsgType = 0x56
	MsgTypePolicyResult      MsgType = 0x57
	MsgTypeSiteVisit         MsgType = 0x58
)

var (
	ErrShortMessage   = errors.New("message too short")
	ErrContentTooLong = errors.New("content too long")
	ErrInvalidFormat  = errors.New("invalid binary format")
	ErrBadChecksum    = errors.New("bad checksum")
)

// Message represents a message in the pestcontrol protocol. The message content can be unmarshaled to a specific struct based on the message type.
type Message struct {
	Type     MsgType // Type of the message.
	Len      uint32  // Total length of the message, including the 6 bytes for the type (1), length (4), and checksum (1).
	Content  []byte  // Content of the message.
	Checksum byte    // Checksum of the message. The sum of checksum and all bytes in the message should be 0 (modulo 256).
}

func (m *Message) ReadFrom(r io.Reader) (int64, error) {
	var fullMsg bytes.Buffer
	tr := io.TeeReader(r, &fullMsg)

	// Type is the first byte
	rawType := make([]byte, 1)
	if _, err := io.ReadFull(tr, rawType); err != nil {
		return int64(fullMsg.Len()), fmt.Errorf("read type byte: %w", err)
	}
	m.Type = MsgType(rawType[0])

	// Total length is the next uin32 (4 bytes)
	rawLen := make([]byte, 4)
	if _, err := io.ReadFull(tr, rawLen); err != nil {
		return int64(fullMsg.Len()), fmt.Errorf("read total length: %w", err)
	}
	m.Len = binary.BigEndian.Uint32(rawLen)

	// Read the rest of the message (save for the 5 bytes we already read)
	contentLen := m.Len - 5
	content := make([]byte, contentLen)
	if _, err := io.ReadFull(tr, content); err != nil {
		return int64(fullMsg.Len()), fmt.Errorf("read content: %w", err)
	}
	m.Content = content[:contentLen-1]
	m.Checksum = content[contentLen-1]
	if err := VerifyChecksum(fullMsg.Bytes()); err != nil {
		// TODO: Send error response
		return int64(fullMsg.Len()), fmt.Errorf("verify checksum: %w", err)
	}

	return int64(fullMsg.Len()), nil
}

func (m *Message) MarshalBinary() ([]byte, error) {
	data := make([]byte, 0, m.Len)
	buf := bytes.NewBuffer(data)
	// Write type
	if err := buf.WriteByte(byte(m.Type)); err != nil {
		return nil, err
	}

	// Write total length
	if err := binary.Write(buf, binary.BigEndian, m.Len); err != nil {
		return nil, err
	}

	// Write content
	if _, err := buf.Write(m.Content); err != nil {
		return nil, err
	}

	if err := buf.WriteByte(calcChecksum(buf.Bytes())); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MsgHello must be sent by each side as the first message of every session. The values for protocol and version must be "pestcontrol" and 1 respectively.
type MsgHello struct {
	Protocol string // Must be "pestcontrol"
	Version  uint32 // Must be 1
}

// ToMsgHello converts a message to a MsgHello struct.
func (m *Message) ToMsgHello() (MsgHello, error) {
	if m.Type != MsgTypeHello {
		return MsgHello{}, fmt.Errorf("unexpected message type: %v", m.Type)
	}

	protocol := "pestcontrol"
	// content length must be at least 16 bytes
	// 11 bytes for protocol ("pestcontrol"), 1 byte for version, 4 bytes for content length
	if len(m.Content) < len(protocol)+1+4 {
		return MsgHello{}, ErrShortMessage
	}

	protocolNameLen := binary.BigEndian.Uint32(m.Content[:4])

	var hello MsgHello
	hello.Protocol = string(m.Content[4 : 4+protocolNameLen])
	if hello.Protocol != "pestcontrol" {
		return hello, fmt.Errorf("unexpected protocol: %v", hello.Protocol)
	}
	// Last 4 bytes are the version
	hello.Version = binary.BigEndian.Uint32(m.Content[4+protocolNameLen:])
	if hello.Version != 1 {
		return hello, fmt.Errorf("unexpected version: %v", hello.Version)
	}
	return hello, nil
}

func (h *MsgHello) MarshalBinary() ([]byte, error) {
	protocol := "pestcontrol"
	version := uint32(4)
	//  "pestcontrol" (11) +  uint32 version (4)
	contentLen := len(protocol) + 4
	content := make([]byte, 0, contentLen)
	content = binary.BigEndian.AppendUint32(content, uint32(contentLen))
	content = append(content, []byte(protocol)...)
	content = binary.BigEndian.AppendUint32(content, version)
	msg := Message{
		Type:    MsgTypeHello,
		Len:     MsgLen(contentLen),
		Content: content,
	}
	return msg.MarshalBinary()
}

// MsgLen calculates the total length a Message, including the type, length, body, and checksum.
func MsgLen(bodyLen int) uint32 {
	// Type (1) + Len (4) + Checksum (1)
	headerTrailerLen := 5
	return uint32(bodyLen + headerTrailerLen)
}

// calcChecksum calculates the uint8 value with summed of all bytes in the message equals 0.
func calcChecksum(data []byte) byte {
	var sum byte
	for _, b := range data {
		sum += b
	}
	// Bitwise NOT sum + 1
	return ^sum + 1
}

// VerifyChecksum return a nil error if the sum of data's bytes equals 0, and ErrBadChecksum otherwise.
func VerifyChecksum(data []byte) error {
	var sum byte
	for _, b := range data {
		sum += b
	}
	if sum != 0 {
		return ErrBadChecksum
	}
	return nil
}

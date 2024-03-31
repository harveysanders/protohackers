package proto

import (
	"encoding/binary"
	"errors"
	"fmt"
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

// MsgHello must be sent by each side as the first message of every session. The values for protocol and version must be "pestcontrol" and 1 respectively.
type MsgHello struct {
	Protocol string // Must be "pestcontrol"
	Version  uint32 // Must be 1
}

func (m *Message) UnmarshalBinary(data []byte) error {
	headerLen := 5 // 1 byte for type, 4 bytes for length
	checksumLen := 1
	if len(data) < headerLen+checksumLen {
		return ErrShortMessage
	}
	// Type is the first byte
	m.Type = MsgType(data[0])
	// Total length is the next uin32 (4 bytes)
	m.Len = binary.BigEndian.Uint32(data[1:5])
	if len(data) != int(m.Len) {
		return fmt.Errorf("expected content length: %d, got: %d", m.Len, len(data)-headerLen-checksumLen)
	}
	m.Content = data[headerLen : m.Len-uint32(checksumLen)]
	m.Checksum = data[len(data)-1]
	return nil
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

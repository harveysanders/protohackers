package message

import (
	"encoding/binary"
	"fmt"
	"time"
)

type (
	MsgType uint8

	Error struct {
		Msg string
	}

	Plate struct {
		Plate     string
		Timestamp time.Time // uint32
	}

	Ticket struct {
		Plate      string // License plate value
		Road       uint16 // Road ID
		Mile1      uint16 // Position of observation
		Mile2      uint16
		Timestamp1 time.Time // uint32 // Chronologically first UNIX timestamp of observation
		Timestamp2 time.Time // uint32 // Chronologically last UNIX timestamp of observation
		Speed      uint16    // Average speed of the car multiplied by 100
	}

	WantHeartbeat struct {
		Interval time.Duration // uint32 // Decisecond interval to send Heartbeat messages to client
	}

	Heartbeat struct{}

	IAmCamera struct {
		Road  uint16
		Mile  uint16
		Limit uint16 // Speed limit (MPH)
	}

	IAmDispatcher struct {
		NumRoads uint8
		Roads    []uint16
	}
)

const (
	TypeError         MsgType = 0x10
	TypePlate         MsgType = 0x20
	TypeTicket        MsgType = 0x21 // (Server->Client)
	TypeWantHeartbeat MsgType = 0x40 // (Client->Server)
	TypeHeartbeat     MsgType = 0x41 // (Server->Client)
	TypeIAmCamera     MsgType = 0x80 // (Client->Server)
	TypeIAmDispatcher MsgType = 0x81 // (Client->Server)
)

// Len returns the expected length of the message of the given type. This includes 1 byte for the message type uint8 itself.
func (t MsgType) Len(buf []byte) int {
	// Message type is the first byte of all messages
	headerLen := 1
	switch t {
	case TypePlate:
		// Read "plate" str len +1 for str header
		plateLen := uint8(buf[headerLen]) + 1
		timestampLen := 4
		return headerLen + int(plateLen) + timestampLen
	case TypeTicket:
		// plate: str +1 for str header
		plateLen := uint8(buf[headerLen]) + 1
		return headerLen + int(plateLen) +
			// road: u16
			2 +
			// mile1: u16
			2 +
			// timestamp1: u32
			4 +
			// mile2: u16
			2 +
			// timestamp2: u32
			4 +
			// speed: u16
			2
	case TypeWantHeartbeat:
		// interval: u32
		return headerLen + 4
	case TypeHeartbeat:
		// No fields
		return headerLen + 0
	case TypeIAmCamera:
		// road: u16
		// mile: u16
		// limit: u16
		return headerLen + 3*2
	case TypeIAmDispatcher:
		// numroads: u8
		numroads := uint8(buf[headerLen])
		// hsg type byte + numroads byte + roads: [u16]
		return headerLen + 1 + int(numroads)*2
	}
	return 0
}

func ParseType(raw uint8) (MsgType, error) {
	switch raw {
	case uint8(TypeError):
		return TypeError, nil
	case uint8(TypePlate):
		return TypePlate, nil
	case uint8(TypeTicket):
		return TypeTicket, nil
	case uint8(TypeWantHeartbeat):
		return TypeWantHeartbeat, nil
	case uint8(TypeHeartbeat):
		return TypeHeartbeat, nil
	case uint8(TypeIAmCamera):
		return TypeIAmCamera, nil
	case uint8(TypeIAmDispatcher):
		return TypeIAmDispatcher, nil
	default:
		return TypeError, fmt.Errorf("invalid message type: %x", raw)
	}
}

// ParseTimestamp interprets a byte slice of of a uint32 as a UNIX timestamp.
func parseTimestamp(data []byte) time.Time {
	// Timestamps are exactly the same as Unix timestamps (counting seconds since 1st of January 1970), except that they are unsigned.
	ts := binary.BigEndian.Uint32(data)
	return time.Unix(int64(ts), 0)
}

func (p *Plate) UnmarshalBinary(msg []byte) {
	offset := 2 // msg type + data type (str) headers
	plateLen := uint8(msg[1])
	p.Plate = string(msg[offset : plateLen+uint8(offset)])
	p.Timestamp = parseTimestamp(msg[2+plateLen:])
}

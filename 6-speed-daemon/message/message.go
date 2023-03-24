package message

import (
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

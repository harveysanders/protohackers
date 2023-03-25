package spdaemon

import (
	"encoding/binary"
	"net"

	"github.com/harveysanders/protohackers/spdaemon/message"
)

type (
	// Each camera is on a specific road, at a specific location, and has a specific speed limit.
	Camera struct {
		conn net.Conn
		message.IAmCamera
	}
)

func (c *Camera) UnmarshalBinary(msg []byte) error {
	// Fields are ORDERED in data
	// road: u16
	c.Road = binary.BigEndian.Uint16(msg[1:3])
	// mile: u16
	c.Mile = binary.BigEndian.Uint16(msg[3:5])
	// limit: u16 (miles per hour)
	c.Limit = binary.BigEndian.Uint16(msg[5:])
	return nil
}

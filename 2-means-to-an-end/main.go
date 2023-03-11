package meanstoend

import (
	"encoding/binary"
	"fmt"
	"net"
)

type (
	// "I" for insert, or "Q" for query
	messageType string

	InsertMessage struct {
		Type      messageType
		Timestamp int32
		Price     int32
	}

	QueryMessage struct {
		Type    messageType
		MinTime int32
		MaxTime int32
	}
)

func (i *InsertMessage) Parse(raw []byte) error {
	if len(raw) != 9 {
		return fmt.Errorf("expected 9 bytes, got %d", len(raw))
	}

	i.Type = messageType(raw[0])
	if i.Type != "I" {
		return fmt.Errorf(`expected type "I", got %q`, i.Type)
	}

	tsRaw := raw[1:5]
	i.Timestamp = int32(binary.BigEndian.Uint32(tsRaw))

	priceRaw := raw[5:9]
	i.Price = int32(binary.BigEndian.Uint32(priceRaw))
	return nil
}

func (i *QueryMessage) Parse(raw []byte) error {
	if len(raw) != 9 {
		return fmt.Errorf("expected 9 bytes, got %d", len(raw))
	}

	i.Type = messageType(raw[0])
	if i.Type != "Q" {
		return fmt.Errorf(`expected type "Q", got %q`, i.Type)
	}

	i.MinTime = int32(binary.BigEndian.Uint32(raw[1:5]))
	i.MaxTime = int32(binary.BigEndian.Uint32(raw[5:9]))
	return nil
}

func HandleConnection(c net.Conn, clientID int) error {
	return nil
}

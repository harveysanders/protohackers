package proto

import (
	"encoding/binary"
)

// Str is a string.
type Str string

func (s Str) String() string {
	return string(s)
}

// UnmarshalBinary decodes Str from a network byte-order byte slice. The first 4 bytes are the length of the string, followed by the ASCII-encoded string itself.
func (s *Str) UnmarshalBinary(data []byte) error {
	u32Len := 4
	if len(data) < u32Len {
		return ErrInvalidFormat
	}
	strLen := binary.BigEndian.Uint32(data[:u32Len])
	if strLen == 0 {
		return nil
	}

	if u32Len+int(strLen) > len(data) {
		return ErrInvalidFormat
	}

	*s = Str(data[u32Len : u32Len+int(strLen)])
	return nil
}

// U32 is a 32-bit unsigned integer.
type U32 uint32

// UnmarshalBinary decodes a U32 from a network byte-order byte slice.
func (u *U32) UnmarshalBinary(data []byte) error {
	u32Len := 4
	if len(data) < u32Len {
		return ErrInvalidFormat
	}

	*u = U32(binary.BigEndian.Uint32(data))
	return nil
}

type Element map[string]any
type Array []Element

func (a *Array) UnmarshalBinary(data []byte) error {
	return nil
}

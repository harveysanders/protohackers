package proto

import (
	"encoding/binary"
	"io"
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

func (s *Str) ReadFrom(r io.Reader) (int64, error) {
	u32Len := 4
	rawLen := make([]byte, u32Len)
	// Using io.ReadFull() rather than binary.Read() to return the correct number of bytes read.
	if n, err := io.ReadFull(r, rawLen); err != nil {
		return int64(n), err
	}
	strLen := binary.BigEndian.Uint32(rawLen)
	if strLen == 0 {
		return int64(u32Len), nil
	}
	str := make([]byte, strLen)
	if n, err := io.ReadFull(r, str); err != nil {
		return int64(u32Len + n), err
	}
	*s = Str(str)
	return int64(u32Len + int(strLen)), nil
}

func (s Str) MarshalBinary() ([]byte, error) {
	length := len(s.String())
	// uint32 size + len of string
	data := make([]byte, 0, 4+length)
	data = binary.BigEndian.AppendUint32(data, uint32(length))
	data = append(data, []byte(s.String())...)
	return data, nil
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

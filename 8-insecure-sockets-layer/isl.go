package isl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/bits"
)

const cipherEnd = 0x00
const (
	operationReverseBits = 0x01
	operandXORN          = 0x02
	operandXORPos        = 0x03
	operandAddN          = 0x04
	operandAddPos        = 0x05
)

type Operation = func(byte, int) byte
type Cipher struct {
	ops []Operation
}

// NewCipher creates a Cipher. The spec is translated to list of
// operations to apply to a byte stream.
// An error is returned for an invalid spec.
func NewCipher(spec io.Reader) (Cipher, error) {
	rdr := bufio.NewReader(spec)
	c := Cipher{ops: []Operation{}}

	var err error
	for _, err = rdr.Peek(1); err == nil; {
		b, err := rdr.ReadByte()
		if err != nil {
			// Should never be EOF because of the Peek call above
			return Cipher{}, fmt.Errorf("rdr.ReadByte(): %w", err)
		}
		switch b {
		case cipherEnd:
			return c, nil
		case operationReverseBits:
			op := func(b byte, pos int) byte {
				return byte(bits.Reverse8(uint8(b)))
			}
			c.ops = append(c.ops, op)
		case operandXORN:
			n, err := rdr.ReadByte()
			if err != nil {
				return Cipher{}, fmt.Errorf("operandXORN ReadByte(): %w", err)
			}
			// Note that 0 is a valid value for N
			if n == 0 {
				return Cipher{}, errors.New("xor(0) is invalid")
			}
			// XOR the byte by the value N.
			op := func(b byte, pos int) byte {
				return b ^ n
			}
			c.ops = append(c.ops, op)
		case operandXORPos:
			// XOR the byte by its position in the stream, starting from 0.
			op := func(b byte, pos int) byte {
				return b ^ byte(pos)
			}
			c.ops = append(c.ops, op)
		case operandAddN:
			n, err := rdr.ReadByte()
			if err != nil {
				return Cipher{}, fmt.Errorf("operandAddN ReadByte: %w", err)
			}
			// Note that 0 is a valid value for N
			if n == 0 {
				return Cipher{}, errors.New("add(0) is invalid")
			}
			//  Add N to the byte, modulo 256.
			//  Addition wraps, so that 255+1=0, 255+2=1, and so on.
			op := func(b byte, pos int) byte {
				return byte((uint(b) + uint(n)) % 256)
			}
			c.ops = append(c.ops, op)
		case operandAddPos:
			op := func(b byte, pos int) byte {
				return byte((uint(b) + uint(pos)) % 256)
			}
			c.ops = append(c.ops, op)
		}
	}
	if err != io.EOF {
		return Cipher{}, err
	}
	return c, nil
}

// Apply serially applies each operation in the Cipher,
// one byte at a time, and returns the resulting byte slice.
func (c Cipher) Apply(in []byte, streamPos int) []byte {
	out := make([]byte, len(in))
	_ = copy(out, in)
	for _, op := range c.ops {
		for bytePos, b := range out {
			out[bytePos] = op(b, streamPos+bytePos)
		}
	}
	return out
}

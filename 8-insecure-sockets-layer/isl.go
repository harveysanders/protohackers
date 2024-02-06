package isl

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/bits"
)

const (
	cipherEnd            = 0x00
	operationReverseBits = 0x01
	operandXORN          = 0x02
	operandXORPos        = 0x03
	operandAddN          = 0x04
	operandAddPos        = 0x05

	MaxSpecLen = 80
)

var (
	ErrMaxCipherSpecSize = errors.New("maximum cipher spec size exceeded")
	ErrXOR0              = errors.New("xor(0) is invalid")
	ErrAddN0             = errors.New("add(0) is invalid")
)

type Operation = func(byte, int) byte
type Cipher struct {
	ops []Operation
}

// ReadFrom populates the cipher's operations from the spec
// contained in the reader. The spec is translated to list of
// operations to apply to a byte stream.
// An error is returned for an invalid spec.
func (c *Cipher) ReadFrom(r io.Reader) (int64, error) {
	rdr := bufio.NewReader(r)
	var err error
	var nRead int64
	for _, err = rdr.Peek(1); err == nil; {
		if nRead > MaxSpecLen {
			return nRead, ErrMaxCipherSpecSize
		}

		b, err := rdr.ReadByte()
		if err != nil {
			// Should never be EOF because of the Peek call above
			return 0, fmt.Errorf("rdr.ReadByte(): %w", err)
		}
		nRead += 1
		switch b {
		case cipherEnd:
			return nRead, nil
		case operationReverseBits:
			op := func(b byte, pos int) byte {
				return byte(bits.Reverse8(uint8(b)))
			}
			c.ops = append(c.ops, op)
		case operandXORN:
			n, err := rdr.ReadByte()
			if err != nil {
				return nRead, fmt.Errorf("operandXORN ReadByte(): %w", err)
			}

			nRead += 1
			// Note that 0 is a valid value for N
			if n == 0 {
				return nRead, ErrXOR0
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
				return nRead, fmt.Errorf("operandAddN ReadByte: %w", err)
			}

			nRead += 1
			// Note that 0 is a valid value for N
			if n == 0 {
				return nRead, ErrAddN0
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
		return nRead, err
	}
	return nRead, nil
}

// NewCipher creates a Cipher.
func NewCipher() *Cipher {
	return &Cipher{ops: []Operation{}}
}

// Encode serially applies each operation in the Cipher,
// one byte at a time, and returns the resulting byte slice.
func (c Cipher) Encode(in []byte, streamPos int) []byte {
	out := make([]byte, len(in))
	_ = copy(out, in)
	for _, op := range c.ops {
		for bytePos, b := range out {
			out[bytePos] = op(b, streamPos+bytePos)
		}
	}
	return out
}

// Decode applies the cipher's operations in reverse order.
func (c Cipher) Decode(in []byte, streamPos int) []byte {
	out := make([]byte, len(in))
	_ = copy(out, in)
	for i := len(c.ops) - 1; i >= 0; i-- {
		op := c.ops[i]
		for bytePos, b := range out {
			out[bytePos] = op(b, streamPos+bytePos)
		}
	}
	return out
}

type StreamDecoder struct {
	encrypted io.Reader
	cipher    Cipher
	pos       int
}

func NewStreamDecoder(r io.Reader, c Cipher, pos int) *StreamDecoder {
	return &StreamDecoder{
		encrypted: r,
		cipher:    c,
		pos:       pos,
	}
}

func (s *StreamDecoder) Read(p []byte) (int, error) {
	buf := make([]byte, len(p))
	n, err := s.encrypted.Read(buf)
	s.pos += n
	if n > 0 {
		decrypted := s.cipher.Decode(buf[:n], s.pos)
		_ = copy(p, decrypted)
		return n, err
	}
	return 0, io.EOF
}

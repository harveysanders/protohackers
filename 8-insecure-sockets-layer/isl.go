package isl

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/bits"
	"time"
)

const (
	cipherEnd            = 0x00 // End of cipher spec.
	operationReverseBits = 0x01 // Reverse the bits of the byte.
	operationXORN        = 0x02 // XOR the byte by N.
	operationXORPos      = 0x03 // XOR the byte by its position in the stream.
	operationAddN        = 0x04 // Add N to the byte. If decoding, subtract N from the byte.
	operationAddPos      = 0x05 // Add the position in the stream to the byte. If decoding, subtract the position from the byte.

	MaxSpecLen    = 80  // Maximum length of the cipher spec.
	maxEmptyReads = 100 // Maximum number of empty reads before returning EOF.
)

var (
	ErrMaxCipherSpecSize = errors.New("maximum cipher spec size exceeded")
	ErrXOR0              = errors.New("xor(0) is invalid")
	ErrAddN0             = errors.New("add(0) is invalid")
	ErrNoOpCipher        = errors.New("cipher spec is a no-op")
)

type Operation = func(byte, int, bool) byte
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
	for _, err := rdr.Peek(1); err == nil; {
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
			// Check for a noop cipher spec.
			if c.IsNoOp() {
				return nRead, ErrNoOpCipher
			}
			return nRead, nil
		case operationReverseBits:
			op := func(b byte, pos int, reverse bool) byte {
				return byte(bits.Reverse8(uint8(b)))
			}
			c.ops = append(c.ops, op)
		case operationXORN:
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
			op := func(b byte, pos int, reverse bool) byte {
				return b ^ n
			}
			c.ops = append(c.ops, op)
		case operationXORPos:
			// XOR the byte by its position in the stream, starting from 0.
			op := func(b byte, pos int, reverse bool) byte {
				return b ^ byte(pos)
			}
			c.ops = append(c.ops, op)
		case operationAddN:
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
			op := func(b byte, pos int, reverse bool) byte {
				if reverse {
					return byte((uint(b) - uint(n)) % 256)
				}
				return byte((uint(b) + uint(n)) % 256)
			}
			c.ops = append(c.ops, op)
		case operationAddPos:
			op := func(b byte, pos int, reverse bool) byte {
				if reverse {
					return byte((uint(b) - uint(pos)) % 256)
				}
				return byte((uint(b) + uint(pos)) % 256)
			}
			c.ops = append(c.ops, op)
		}
	}
	if err != io.EOF {
		return nRead, err
	}

	// We should never reach this point because the loop should always
	// return on encountering a cipherEnd byte.
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
			out[bytePos] = op(b, streamPos+bytePos, false)
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
			out[bytePos] = op(b, streamPos+bytePos, true)
		}
	}
	return out
}

// IsNoOp returns true if the cipher spec leaves every byte unchanged, e.g. no-op.
func (c *Cipher) IsNoOp() bool {
	// "hello world\n"
	sample := []byte{0x68, 0x65, 0x6c, 0x6c, 0x6f, 0x20, 0x77, 0x6f, 0x72, 0x6c, 0x64, 0x0a}
	encoded := c.Encode(sample, 0)
	return bytes.Equal(sample, encoded)
}

type StreamDecoder struct {
	encoded    io.Reader
	cipher     Cipher
	pos        int
	emptyReads int
}

func NewStreamDecoder(r io.Reader, c Cipher, pos int) *StreamDecoder {
	return &StreamDecoder{
		encoded: r,
		cipher:  c,
		pos:     pos,
	}
}

func (s *StreamDecoder) Read(p []byte) (int, error) {
	buf := make([]byte, len(p))
	for {
		n, err := s.encoded.Read(buf)
		if n > 0 {
			s.emptyReads = 0
			decoded := s.cipher.Decode(buf[:n], s.pos)
			_ = copy(p, decoded)
			s.pos += n
			return n, err
		}
		// Empty read
		s.emptyReads++
		if s.emptyReads > maxEmptyReads {
			return 0, io.EOF
		}
		time.Sleep(10 * time.Millisecond)
	}
}

package mobprox

import (
	"bytes"
	"regexp"
	"slices"
)

type (
	bcoinReplacer struct {
		// Boguscoin Address to replace with
		bCoinAddr string
		// Pattern to match a Boguscoin address.
		bCoinPattern *regexp.Regexp
	}
)

func newbcoinReplacer(bCoinAddr string) *bcoinReplacer {
	return &bcoinReplacer{
		bCoinAddr: bCoinAddr,
		// it starts with a "7"
		// it consists of at least 26, and at most 35, alphanumeric characters
		// it starts at the start of a chat message, or is preceded by a space
		// it ends at the end of a chat message, or is followed by a space
		bCoinPattern: regexp.MustCompile(`\s?(7\w{25,34})\s?`),
	}
}

func (b *bcoinReplacer) intercept(in []byte) []byte {
	matches := b.bCoinPattern.FindAllSubmatchIndex(in, -1)
	if matches == nil {
		return in
	}

	out := slices.Clone(in)
	offset := 0
	for _, match := range matches {
		// Submatch start and end indexes will be at match[2], match[3]
		startI := match[2] + offset
		endI := match[3] + offset

		endsWithNewline := out[len(out)-1] == '\n'
		_in := bytes.TrimSpace(out)
		// Remove the original
		origBcoin := _in[startI:endI]
		if bytes.Equal(origBcoin, []byte(b.bCoinAddr)) {
			continue
		}

		offset += len(b.bCoinAddr) - len(origBcoin)

		_in = bytes.Replace(_in, origBcoin, []byte(""), 1)
		replaced := slices.Insert(_in, startI, []byte(b.bCoinAddr)...)
		if endsWithNewline {
			replaced = append(replaced, '\n')
		}
		out = replaced
	}
	return out
}

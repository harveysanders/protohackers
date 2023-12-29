package mobprox

import (
	"bytes"
	"regexp"
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
	match := b.bCoinPattern.FindSubmatchIndex(in)
	// Submatch start and end indexes will be at match[2], match[3]
	if match == nil || len(match) < 4 {
		return in
	}

	startI := match[2]
	endI := match[3]

	if startI > 1 && endI < len(in)-2 {
		// Match is not at the start or end of the message
		return in
	}

	endsWithNewline := in[len(in)-1] == '\n'
	in = bytes.TrimSpace(in)
	// Remove the original
	origBcoin := in[startI:endI]
	in = bytes.Replace(in, origBcoin, []byte(""), 1)

	out := make([]byte, 0)
	// Address was at the beginning of the message
	if startI < 2 {
		out = append(out, []byte(b.bCoinAddr)...)
		out = append(out, in...)
		return out
	}
	// Address was at the end of the message
	out = append(out, in...)
	out = append(out, []byte(b.bCoinAddr)...)
	if endsWithNewline {
		out = append(out, '\n')
	}
	return out
}

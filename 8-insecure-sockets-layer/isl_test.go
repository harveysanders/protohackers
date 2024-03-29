package isl_test

import (
	"bytes"
	"os"
	"testing"

	isl "github.com/harveysanders/protohackers/8-insecure-sockets-layer"
	"github.com/stretchr/testify/require"
)

func TestEncode(t *testing.T) {
	testCases := []struct {
		desc       string
		cipherSpec []byte
		input      []byte
		want       []byte
	}{
		{
			desc:  "cipher spec: reversebits",
			input: []byte{0x69, 0x64, 0x6d, 0x6d, 0x6e},
			// reversebits
			cipherSpec: []byte{
				0x01, // reversebits
				0x00, // end of cipher spec
			},
			want: []byte{0x96, 0x26, 0xb6, 0xb6, 0x76},
		},
		{
			desc:  "cipher spec: xor(1), reversebits",
			input: []byte("hello"), // 0x68, 0x65, 0x6c, 0x6c, 0x6f
			cipherSpec: []byte{
				0x02, 0x01, // xor(1)
				0x01, // reversebits
				0x00, // end of cipher spec
			},
			want: []byte{0x96, 0x26, 0xb6, 0xb6, 0x76},
		},
		{
			desc:  "cipher spec: addpos,addpos",
			input: []byte("hello"),
			cipherSpec: []byte{
				0x05, // addpos
				0x05, // addpos
				0x00, // end of cipher spec
			},
			want: []byte{0x68, 0x67, 0x70, 0x72, 0x77},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {

			cipher := isl.NewCipher()
			n, err := cipher.ReadFrom(bytes.NewReader(tc.cipherSpec))
			require.NoError(t, err)
			require.Equal(t, len(tc.cipherSpec), int(n))

			got := cipher.Encode(tc.input, 0)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestDecode(t *testing.T) {
	testCases := []struct {
		desc       string
		cipherSpec []byte
		input      []byte
		want       []byte
	}{
		{
			desc: "cipher spec: xor(123),addpos,reversebits",
			input: []byte{
				0xf2, 0x20, 0xba, 0x44, 0x18, 0x84, 0xba, 0xaa, 0xd0, 0x26, 0x44, 0xa4, 0xa8, 0x7e,
			},
			// reversebits
			cipherSpec: []byte{
				0x02, 0x7b, // xor(123)
				0x05, // addpos
				0x01, // reversebits
				0x00, // end of cipher spec
			},
			want: []byte("4x dog,5x car\n"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cipher := isl.NewCipher()
			n, err := cipher.ReadFrom(bytes.NewReader(tc.cipherSpec))
			require.NoError(t, err)
			require.Equal(t, len(tc.cipherSpec), int(n))

			got := cipher.Decode(tc.input, 0)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestEdgeCases(t *testing.T) {
	testCases := []struct {
		desc       string
		cipherSpec []byte
		input      []byte
		want       []byte
	}{
		{
			desc:       "long message with inline newline",
			cipherSpec: edgeCase1.cipherSpec,
			input:      edgeCase1.reqs[1],
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			cipher := isl.NewCipher()
			n, err := cipher.ReadFrom(bytes.NewReader(tc.cipherSpec))
			require.NoError(t, err)
			require.Equal(t, len(tc.cipherSpec), int(n))

			got := cipher.Decode(tc.input, 0)
			f, err := os.Create("./got2.txt")
			require.NoError(t, err)
			defer f.Close()
			_, err = f.Write(got)
			require.NoError(t, err)
			bytes.Contains(got, []byte{'\n'})
		})
	}
}

func TestDecodeCase(t *testing.T) {
	cipher := isl.NewCipher()
	_, err := cipher.ReadFrom(bytes.NewReader(edgeCase1.cipherSpec))
	require.NoError(t, err)

	testCases := []struct {
		desc    string
		encoded []byte
		want    []byte
	}{
		{
			desc:    "response 1",
			encoded: edgeCase1.responses[0],
			want:    []byte("99x bear with remote-controlled lorry simulator\n"),
		},
		{
			desc:    "response 2",
			encoded: edgeCase1.responses[1],
			want:    []byte("100x inflatable bear toy\n"),
		},
		{
			desc:    "response 3",
			encoded: edgeCase1.responses[2],
			want:    []byte("98x giant sheep with FPV car with carry case\n"),
		},
	}

	offset := 0
	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			got := cipher.Decode(tc.encoded, offset)
			require.Equal(t, string(tc.want), string(got))
			offset += len(tc.encoded)
		})
	}

	decoded := cipher.Decode(edgeCase1.reqs[1], 4250)
	os.WriteFile("./decoded.txt", decoded, 0644)
	chunks := bytes.Split(decoded, []byte{'\n'})
	require.Equal(t, 3, len(chunks))
	require.Equal(t, "4x dog,5x car\n", string(decoded))

}

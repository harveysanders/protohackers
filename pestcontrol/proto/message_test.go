package proto_test

import (
	"bytes"
	"encoding"
	"testing"

	"github.com/harveysanders/protohackers/pestcontrol/proto"
	"github.com/stretchr/testify/require"
)

func TestMessageHello(t *testing.T) {
	input := []byte{
		0x50,                   // MsgTypeHello{
		0x00, 0x00, 0x00, 0x19, // (length 25)
		0x00, 0x00, 0x00, 0x0b, // protocol: (length 11)
		0x70, 0x65, 0x73, 0x74, // "pest
		0x63, 0x6f, 0x6e, 0x74, // 	cont
		0x72, 0x6f, 0x6c, //			 	rol"
		0x00, 0x00, 0x00, 0x01, // version: 1
		0xce, // (checksum 0xce)
	}

	wantHello := proto.MsgHello{Protocol: "pestcontrol", Version: 1}

	var gotMessage proto.Message
	_, err := gotMessage.ReadFrom(bytes.NewReader(input))
	require.NoError(t, err)
	require.Equal(t, gotMessage.Type, proto.MsgTypeHello)
	require.Equal(t, gotMessage.Len, uint32(25))

	gotHello, err := gotMessage.ToMsgHello()
	require.NoError(t, err)
	require.Equal(t, wantHello, gotHello)
}

func TestMessage_MarshalBinary(t *testing.T) {
	testCases := []struct {
		name    string
		message encoding.BinaryMarshaler
		want    []byte
	}{
		{
			name: "Message struct",
			message: proto.Message{
				Type: proto.MsgTypeHello,
				Len:  25,
				Content: []byte{
					0x00, 0x00, 0x00, 0x0b, // protocol: (length 11)
					0x70, 0x65, 0x73, 0x74, // "pest
					0x63, 0x6f, 0x6e, 0x74, // 	cont
					0x72, 0x6f, 0x6c, //			 	rol"
					0x00, 0x00, 0x00, 0x01, // version: 1
				},
			},
			want: []byte{
				0x50,                   // MsgTypeHello{
				0x00, 0x00, 0x00, 0x19, // (length 25)
				0x00, 0x00, 0x00, 0x0b, // protocol: (length 11)
				0x70, 0x65, 0x73, 0x74, // "pest
				0x63, 0x6f, 0x6e, 0x74, // 	cont
				0x72, 0x6f, 0x6c, //			 	rol"
				0x00, 0x00, 0x00, 0x01, // version: 1
				0xce, // (checksum 0xce)
			},
		},
		{
			name:    "Empty MsgHello struct",
			message: proto.MsgHello{},
			want: []byte{
				0x50,                   // MsgTypeHello{
				0x00, 0x00, 0x00, 0x19, // (length 25)
				0x00, 0x00, 0x00, 0x0b, // protocol: (length 11)
				0x70, 0x65, 0x73, 0x74, // "pest
				0x63, 0x6f, 0x6e, 0x74, // 	cont
				0x72, 0x6f, 0x6c, //			 	rol"
				0x00, 0x00, 0x00, 0x01, // version: 1
				0xce, // (checksum 0xce)
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.message.MarshalBinary()
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	testCases := []struct {
		data []byte
		want error
	}{
		{data: []byte{0x50, // MsgTypeHello{
			0x00, 0x00, 0x00, 0x19, // (length 25)
			0x00, 0x00, 0x00, 0x0b, // protocol: (length 11)
			0x70, 0x65, 0x73, 0x74, // "pest
			0x63, 0x6f, 0x6e, 0x74, // 	cont
			0x72, 0x6f, 0x6c, //			 	rol"
			0x00, 0x00, 0x00, 0x01, // version: 1
			0xce}, want: nil},
	}

	for _, tc := range testCases {
		err := proto.VerifyChecksum(tc.data)
		require.Equal(t, tc.want, err)
	}
}

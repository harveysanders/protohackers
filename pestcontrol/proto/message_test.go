package proto_test

import (
	"bytes"
	"encoding"
	"testing"

	"github.com/harveysanders/protohackers/pestcontrol/proto"
	"github.com/stretchr/testify/require"
)

func TestMsgHello(t *testing.T) {
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

func TestMsgTargetPopulations(t *testing.T) {
	// 	Hexadecimal:    Decoded:
	// 54              TargetPopulations{
	// 00 00 00 2c       (length 44)
	// 00 00 30 39       site: 12345,
	// 00 00 00 02       populations: (length 2) [
	//                     {
	// 00 00 00 03           species: (length 3)
	// 64 6f 67                "dog",
	// 00 00 00 01           min: 1,
	// 00 00 00 03           max: 3,
	//                     },
	//                     {
	// 00 00 00 03           species: (length 3)
	// 72 61 74                "rat",
	// 00 00 00 00           min: 0,
	// 00 00 00 0a           max: 10,
	//                     },
	//                   ],
	// 80                (checksum 0x80)
	//                 }

	input := []byte{
		0x54,                   // TargetPopulations{
		0x00, 0x00, 0x00, 0x2c, // (length 44)
		0x00, 0x00, 0x30, 0x39, // site: 12345,
		0x00, 0x00, 0x00, 0x02, // populations: (length 2) [
		// _______{
		0x00, 0x00, 0x00, 0x03, // species: (length 3)
		0x64, 0x6f, 0x67, // "dog"
		0x00, 0x00, 0x00, 0x01, // min: 1,
		0x00, 0x00, 0x00, 0x03, // max: 3,
		// _______},
		// _______{
		0x00, 0x00, 0x00, 0x03, // species: (length 3)
		0x72, 0x61, 0x74, // "rat"
		0x00, 0x00, 0x00, 0x00, // min: 0,
		0x00, 0x00, 0x00, 0x0a, // max: 10,
		// _______},
		// _______],
		0x80, // (checksum 0x80)
	}

	wantPopulations := proto.MsgTargetPopulations{
		Site: 12345,
		Populations: []proto.Population{
			{Species: "dog", Min: 1, Max: 3},
			{Species: "rat", Min: 0, Max: 10},
		},
	}

	var gotMessage proto.Message
	_, err := gotMessage.ReadFrom(bytes.NewReader(input))
	require.NoError(t, err)
	require.Equal(t, gotMessage.Type, proto.MsgTypeTargetPopulations)
	require.Equal(t, gotMessage.Len, uint32(44))

	gotPopulations, err := gotMessage.ToMsgTargetPopulations()
	require.NoError(t, err)
	require.Equal(t, wantPopulations, gotPopulations)
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
			name:    "Empty MsgHello",
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
		{
			name:    "MsgDialAuthority",
			message: proto.MsgDialAuthority{Site: 12345},
			want: []byte{
				0x53,                   // DialAuthority{
				0x00, 0x00, 0x00, 0x0a, // (length 10)
				0x00, 0x00, 0x30, 0x39, // site: 12345,
				0x3a, // (checksum 0x3a)
			},
		},
		{
			name:    "MsgTargetPopulations",
			message: proto.MsgTargetPopulations{Site: 12345, Populations: []proto.Population{{Species: "dog", Min: 1, Max: 3}, {Species: "rat", Min: 0, Max: 10}}},
			want: []byte{
				0x54,                   // TargetPopulations{
				0x00, 0x00, 0x00, 0x2c, // (length 44)
				0x00, 0x00, 0x30, 0x39, // site: 12345,
				0x00, 0x00, 0x00, 0x02, // populations: (length 2) [
				// _______{
				0x00, 0x00, 0x00, 0x03, // species: (length 3)
				0x64, 0x6f, 0x67, // "dog"
				0x00, 0x00, 0x00, 0x01, // min: 1,
				0x00, 0x00, 0x00, 0x03, // max: 3,
				// _______},
				// _______{
				0x00, 0x00, 0x00, 0x03, // species: (length 3)
				0x72, 0x61, 0x74, // "rat"
				0x00, 0x00, 0x00, 0x00, // min: 0,
				0x00, 0x00, 0x00, 0x0a, // max: 10,
				// _______},
				// _______],
				0x80, // (checksum 0x80)
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

package proto_test

import (
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
	err := gotMessage.UnmarshalBinary(input)
	require.NoError(t, err)
	require.Equal(t, gotMessage.Type, proto.MsgTypeHello)
	require.Equal(t, gotMessage.Len, uint32(25))

	gotHello, err := gotMessage.ToMsgHello()
	require.NoError(t, err)
	require.Equal(t, wantHello, gotHello)
}

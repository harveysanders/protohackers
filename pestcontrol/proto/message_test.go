package proto_test

import (
	"bytes"
	"encoding"
	"testing"

	"github.com/harveysanders/protohackers/pestcontrol/proto"
	"github.com/stretchr/testify/require"
)

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
			message: proto.MsgTargetPopulations{Site: 12345, Populations: []proto.PopulationTarget{{Species: "dog", Min: 1, Max: 3}, {Species: "rat", Min: 0, Max: 10}}},
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
		{
			name: "MsgSiteVisit",
			message: proto.MsgSiteVisit{
				Site: 12345,
				Populations: []proto.PopulationCount{
					{Species: "dog", Count: 1},
					{Species: "rat", Count: 5},
				}},
			want: []byte{
				0x58,                   // SiteVisit{
				0x00, 0x00, 0x00, 0x24, // (length 36)
				0x00, 0x00, 0x30, 0x39, // site: 12345,
				0x00, 0x00, 0x00, 0x02, // populations: (length 2) [
				// _______{
				0x00, 0x00, 0x00, 0x03, // species: (length 3)
				0x64, 0x6f, 0x67, // "dog"
				0x00, 0x00, 0x00, 0x01, // count: 1,
				// _______},
				// _______{
				0x00, 0x00, 0x00, 0x03, // species: (length 3)
				0x72, 0x61, 0x74, // "rat"
				0x00, 0x00, 0x00, 0x05, // count: 5,
				// _______},
				// _______],
				0x8c, // (checksum 0x8c)

			},
		},
		{
			name: "CreatePolicy",
			message: proto.MsgCreatePolicy{
				Species: "dog",
				Action:  proto.Conserve},
			want: []byte{
				0x55,                   // CreatePolicy{
				0x00, 0x00, 0x00, 0x0e, // (length 14)
				0x00, 0x00, 0x00, 0x03, // species: (length 3)
				0x64, 0x6f, 0x67, // "dog"
				0xa0, // action: conserve,
				0xc0, // (checksum 0xc0)
			},
		},
		{
			name:    "MsgPolicyResult",
			message: proto.MsgPolicyResult{PolicyID: 123},
			want: []byte{
				0x57,                   // PolicyResult{
				0x00, 0x00, 0x00, 0x0a, // (length 10)
				0x00, 0x00, 0x00, 0x7b, // policy: 123,
				0x24, // (checksum 0x24)
			},
		},
		{
			name:    "MsgDeletePolicy",
			message: proto.MsgDeletePolicy{Policy: 123},
			want: []byte{
				0x56,                   // DeletePolicy{
				0x00, 0x00, 0x00, 0x0a, // (length 10)
				0x00, 0x00, 0x00, 0x7b, // policy: 123,
				0x25, // (checksum 0x25)
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
		Populations: []proto.PopulationTarget{
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

func TestMsgCreatePolicy(t *testing.T) {
	input := []byte{
		0x55,                   // CreatePolicy{
		0x00, 0x00, 0x00, 0x0e, // (length 14)
		0x00, 0x00, 0x00, 0x03, // species: (length 3)
		0x64, 0x6f, 0x67, // "dog"
		0xa0, // action: conserve,
		0xc0, // (checksum 0xc0)
	}

	wantCreatePolicy := proto.MsgCreatePolicy{Species: "dog", Action: proto.Conserve}

	var gotMessage proto.Message
	_, err := gotMessage.ReadFrom(bytes.NewReader(input))
	require.NoError(t, err)
	require.Equal(t, gotMessage.Type, proto.MsgTypeCreatePolicy)
	require.Equal(t, gotMessage.Len, uint32(14))

	gotCreatePolicy, err := gotMessage.ToMsgCreatePolicy()
	require.NoError(t, err)
	require.Equal(t, wantCreatePolicy, gotCreatePolicy)
}

func TestMsgPolicyResult(t *testing.T) {
	input := []byte{
		0x57,                   // PolicyResult{
		0x00, 0x00, 0x00, 0x0a, // (length 10)
		0x00, 0x00, 0x00, 0x7b, // policy: 123,
		0x24, // (checksum 0x24)
	}

	wantPolicyResult := proto.MsgPolicyResult{PolicyID: 123}

	var gotMessage proto.Message
	_, err := gotMessage.ReadFrom(bytes.NewReader(input))
	require.NoError(t, err)
	require.Equal(t, gotMessage.Type, proto.MsgTypePolicyResult)
	require.Equal(t, gotMessage.Len, uint32(10))

	gotPolicyResult, err := gotMessage.ToMsgPolicyResult()
	require.NoError(t, err)
	require.Equal(t, wantPolicyResult, gotPolicyResult)
}

func TestMsgSiteVisit(t *testing.T) {
	input := []byte{
		0x58,                   // SiteVisit{
		0x00, 0x00, 0x00, 0x24, // (length 36)
		0x00, 0x00, 0x30, 0x39, // site: 12345,
		0x00, 0x00, 0x00, 0x02, // populations: (length 2) [
		// _______{
		0x00, 0x00, 0x00, 0x03, // species: (length 3)
		0x64, 0x6f, 0x67, // "dog"
		0x00, 0x00, 0x00, 0x01, // count: 1,
		// _______},
		// _______{
		0x00, 0x00, 0x00, 0x03, // species: (length 3)
		0x72, 0x61, 0x74, // "rat"
		0x00, 0x00, 0x00, 0x05, // count: 5,
		// _______},
		// _______],
		0x8c, // (checksum 0x8c)
	}

	wantSiteVisit := proto.MsgSiteVisit{
		Site: 12345,
		Populations: []proto.PopulationCount{
			{Species: "dog", Count: 1},
			{Species: "rat", Count: 5},
		},
	}

	var gotMessage proto.Message
	_, err := gotMessage.ReadFrom(bytes.NewReader(input))
	require.NoError(t, err)
	require.Equal(t, gotMessage.Type, proto.MsgTypeSiteVisit)
	require.Equal(t, gotMessage.Len, uint32(36))

	gotSiteVisit, err := gotMessage.ToMsgSiteVisit()
	require.NoError(t, err)
	require.Equal(t, wantSiteVisit, gotSiteVisit)
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

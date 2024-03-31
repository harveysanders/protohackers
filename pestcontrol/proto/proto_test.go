package proto_test

import (
	"testing"

	"github.com/harveysanders/protohackers/pestcontrol/proto"
	"github.com/stretchr/testify/require"
)

func TestStr_UnmarshalBinary(t *testing.T) {
	testCases := []struct {
		data []byte
		want string
	}{
		{want: "", data: []byte{0x00, 0x00, 0x00, 0x00}},
		{want: "foo", data: []byte{0x00, 0x00, 0x00, 0x03, 0x66, 0x6f, 0x6f}},
		{want: "Elbereth", data: []byte{0x00, 0x00, 0x00, 0x08, 0x45, 0x6C, 0x62, 0x65, 0x72, 0x65, 0x74, 0x68}},
	}

	for _, tc := range testCases {
		var s proto.Str
		err := s.UnmarshalBinary(tc.data)
		require.NoError(t, err)
		require.Equal(t, tc.want, s.String())
	}
}

func TestU32_UnmarshalBinary(t *testing.T) {
	testCases := []struct {
		data []byte
		want uint32
	}{
		{want: 32, data: []byte{0x00, 0x00, 0x00, 0x20}},
		{want: 4677, data: []byte{0x00, 0x00, 0x12, 0x45}},
		{want: 2796139879, data: []byte{0xa6, 0xa9, 0xb5, 0x67}},
	}

	for _, tc := range testCases {
		var u proto.U32
		err := u.UnmarshalBinary(tc.data)
		require.NoError(t, err)
		require.Equal(t, tc.want, uint32(u))
	}
}

func TestArray_UnmarshalBinary(t *testing.T) {
	testCases := []struct {
		data []byte
		want proto.Array
	}{
		{
			want: []proto.Element{
				{"species": "rat", "count": 10},
				{"species": "dog", "count": 15},
			},
			data: []byte{
				0x00, 0x00, 0x00, 0x02, // (length of array 2) [
				// {
				0x00, 0x00, 0x00, 0x03, 0x72, 0x61, 0x74, //  species: (length 3) "rat",

				0x00, 0x00, 0x00, 0x0a, //  count: (length 10) 10,
				// },
				// {
				// 00 00 00 03 64 6f 67
				0x00, 0x00, 0x00, 0x03, 0x64, 0x6f, 0x67, //  species: (length 3) "dog",
				0x00, 0x00, 0x00, 0x0f, //  count: (length 15) 15,
				// },
				// ],

			}},
		{want: nil, data: []byte{0x00, 0x00, 0x00, 0x00}},
		{want: nil, data: []byte{0x00, 0x00, 0x00, 0x01, 0x00}},
	}

	for _, tc := range testCases {
		var a proto.Array
		err := a.UnmarshalBinary(tc.data)
		require.NoError(t, err)
		require.Equal(t, tc.want, a)
	}
}

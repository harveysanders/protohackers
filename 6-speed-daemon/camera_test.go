package spdaemon_test

import (
	"testing"

	"github.com/harveysanders/protohackers/spdaemon"
	"github.com/stretchr/testify/require"
)

func TestCameraUnmarshalBinary(t *testing.T) {
	testCases := []struct {
		data []byte
		want spdaemon.Camera
	}{
		{
			data: []byte{0x80, 0x00, 0x42, 0x00, 0x64, 0x00, 0x3c},
			want: spdaemon.Camera{Road: 66, Mile: 100, Limit: 60},
		},
		{
			data: []byte{0x80, 0x01, 0x70, 0x04, 0xd2, 0x00, 0x28},
			want: spdaemon.Camera{Road: 368, Mile: 1234, Limit: 40},
		},
	}

	for _, tc := range testCases {
		cam := spdaemon.Camera{}
		err := cam.UnmarshalBinary(tc.data)
		require.NoError(t, err)

		require.Equal(t, tc.want.Road, cam.Road)
		require.Equal(t, tc.want.Mile, cam.Mile)
		require.Equal(t, tc.want.Limit, cam.Limit)
	}
}

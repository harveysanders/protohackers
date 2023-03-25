package spdaemon_test

import (
	"testing"

	"github.com/harveysanders/protohackers/spdaemon"
	"github.com/stretchr/testify/require"
)

func TestCameraUnmarshalBinary(t *testing.T) {
	iAmCamMsg := []byte{0x80, 0x00, 0x42, 0x00, 0x64, 0x00, 0x3c}

	cam := spdaemon.Camera{}
	err := cam.UnmarshalBinary(iAmCamMsg)
	require.NoError(t, err)

	require.Equal(t, uint16(66), cam.Road)
	require.Equal(t, uint16(100), cam.Mile)
	require.Equal(t, uint16(60), cam.Limit)
}

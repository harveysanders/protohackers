package message_test

import (
	"testing"
	"time"

	"github.com/harveysanders/protohackers/spdaemon/message"
	"github.com/stretchr/testify/require"
)

func TestParseType(t *testing.T) {
	testCases := []struct {
		hex  uint8
		want message.MsgType
	}{
		{hex: 0x10, want: message.TypeError},
		{hex: 0x20, want: message.TypePlate},
		{hex: 0x21, want: message.TypeTicket},
		{hex: 0x40, want: message.TypeWantHeartbeat},
		{hex: 0x41, want: message.TypeHeartbeat},
		{hex: 0x80, want: message.TypeIAmCamera},
		{hex: 0x81, want: message.TypeIAmDispatcher},
	}
	for _, tc := range testCases {
		got, err := message.ParseType(tc.hex)
		require.NoError(t, err)
		require.Equal(t, tc.want, got)
	}
}

func TestPlateUnmarshalBinary(t *testing.T) {
	testCases := []struct {
		data []byte
		want message.Plate
	}{
		{
			data: []byte{0x20, 0x04, 0x55, 0x4e, 0x31, 0x58, 0x00, 0x00, 0x03, 0xe8},
			want: message.Plate{Plate: "UN1X", Timestamp: time.Unix(1000, 0)},
		},
		{
			data: []byte{0x20, 0x07, 0x52, 0x45, 0x30, 0x35, 0x42, 0x4b, 0x47, 0x00, 0x01, 0xe2, 0x40},
			want: message.Plate{Plate: "RE05BKG", Timestamp: time.Unix(123456, 0)},
		},
	}

	for _, tc := range testCases {
		var got message.Plate
		got.UnmarshalBinary(tc.data)
		require.Equal(t, tc.want.Plate, got.Plate)
		require.Equal(t, tc.want.Timestamp, got.Timestamp)

	}
}

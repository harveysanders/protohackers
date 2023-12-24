package message_test

import (
	"testing"
	"time"

	"github.com/harveysanders/protohackers/6-speed-daemon/message"
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

func TestTicketMarshalBinary(t *testing.T) {
	testCases := []struct {
		ticket message.Ticket
		want   []byte
	}{
		{
			ticket: message.Ticket{
				Plate:      "UN1X",
				Road:       66,
				Mile1:      100,
				Timestamp1: 123456,
				Mile2:      110,
				Timestamp2: 123816,
				Speed:      10_000,
			},
			want: []byte{0x21, 0x04, 0x55, 0x4e, 0x31, 0x58, 0x00, 0x42, 0x00, 0x64, 0x00, 0x01, 0xe2, 0x40, 0x00, 0x6e, 0x00, 0x01, 0xe3, 0xa8, 0x27, 0x10},
		},
		{
			ticket: message.Ticket{
				Plate:      "RE05BKG",
				Road:       368,
				Mile1:      1234,
				Timestamp1: 1000000,
				Mile2:      1235,
				Timestamp2: 1000060,
				Speed:      6000,
			},
			want: []byte{0x21, 0x07, 0x52, 0x45, 0x30, 0x35, 0x42, 0x4b, 0x47, 0x01, 0x70, 0x04, 0xd2, 0x00, 0x0f, 0x42, 0x40, 0x04, 0xd3, 0x00, 0x0f, 0x42, 0x7c, 0x17, 0x70},
		},
	}

	for _, tc := range testCases {
		got := tc.ticket.MarshalBinary()
		require.Equal(t, tc.want, got)
	}

}

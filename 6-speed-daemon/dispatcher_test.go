package spdaemon_test

import (
	"testing"

	"github.com/harveysanders/protohackers/spdaemon"
	"github.com/stretchr/testify/require"
)

func TestDispatcherUnmarshalBinary(t *testing.T) {
	testCases := []struct {
		data []byte
		want spdaemon.TicketDispatcher
	}{
		{
			data: []byte{0x81, 0x01, 0x00, 0x42},
			want: spdaemon.TicketDispatcher{Roads: []uint16{66}},
		},
		{
			data: []byte{0x81, 0x03, 0x00, 0x42, 0x01, 0x70, 0x13, 0x88},
			want: spdaemon.TicketDispatcher{Roads: []uint16{66, 368, 5000}},
		},
	}

	for _, tc := range testCases {
		td := spdaemon.TicketDispatcher{}
		td.UnmarshalBinary(tc.data)

		require.Equal(t, tc.want.Roads, td.Roads)
	}
}

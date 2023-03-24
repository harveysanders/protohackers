package message_test

import (
	"testing"

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

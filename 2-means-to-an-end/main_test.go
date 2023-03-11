package meanstoend_test

import (
	"testing"

	m2e "github.com/harveysanders/protohackers/meanstoend"
	"github.com/stretchr/testify/require"
)

func TestInsertMessageParse(t *testing.T) {
	testCases := []struct {
		raw  []byte
		want m2e.InsertMessage
	}{
		{
			raw: []byte{0x49, 0x00, 0x00, 0x30, 0x39, 0x00, 0x00, 0x00, 0x65},
			want: m2e.InsertMessage{
				Type:      "I",
				Timestamp: 12_345,
				Price:     101},
		},
	}

	for _, tc := range testCases {
		var got m2e.InsertMessage
		err := got.Parse(tc.raw)
		require.NoError(t, err)

		require.Equal(t, tc.want, got)
	}
}

func TestQueryMessageParse(t *testing.T) {
	testCases := []struct {
		raw  []byte
		want m2e.QueryMessage
	}{
		{
			raw: []byte{0x51, 0x00, 0x00, 0x03, 0xe8, 0x00, 0x01, 0x86, 0xa0},
			want: m2e.QueryMessage{
				Type:    "Q",
				MinTime: 1_000,
				MaxTime: 100_000},
		},
	}

	for _, tc := range testCases {
		var got m2e.QueryMessage
		err := got.Parse(tc.raw)
		require.NoError(t, err)

		require.Equal(t, tc.want, got)
	}
}

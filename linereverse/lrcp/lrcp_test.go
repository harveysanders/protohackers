package lrcp_test

import (
	"bytes"
	"testing"

	"github.com/harveysanders/protohackers/linereverse/lrcp"
	"github.com/stretchr/testify/require"
)

func TestScanLRCPSection(t *testing.T) {
	testCases := []struct {
		message   string
		wantParts []string
		desc      string
	}{
		{
			message:   `/connect/1234567/`,
			wantParts: []string{"connect", "1234567"},
			desc:      "Connect Message",
		},
		{
			message:   `/data/1234567/0/hello/`,
			wantParts: []string{"data", "1234567", "0", "hello"},
			desc:      "Data \"hello\" Message",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			r := lrcp.NewReader(bytes.NewBufferString(tc.message))
			msgParts, err := r.ReadMessage()

			require.NoError(t, err)
			require.Equal(t, tc.wantParts, msgParts)

		})
	}

}

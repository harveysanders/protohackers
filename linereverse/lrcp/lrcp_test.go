package lrcp_test

import (
	"bufio"
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
			message:   "connect/1234567/",
			wantParts: []string{"connect", "1234567"},
			desc:      "Connect Message",
		},
		{
			message:   "data/1234567/0/hello/",
			wantParts: []string{"data", "1234567", "0", "hello"},
			desc:      "Data \"hello\" Message",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			r := bytes.NewBufferString(tc.message)
			scr := bufio.NewScanner(r)

			// Split function under test
			scr.Split(lrcp.ScanLRCPSection)

			index := 0
			for scr.Scan() {
				require.NoError(t, scr.Err())

				got := scr.Text()
				require.Equal(t, tc.wantParts[index], got)
				index++
			}
		})
	}

}

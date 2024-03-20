package vcs_test

import (
	"bufio"
	"fmt"
	"net"
	"testing"
	"time"

	vcs "github.com/harveysanders/protohackers/10-voracious-code-storage"
	"github.com/stretchr/testify/require"
)

type direction int

const (
	recv direction = iota
	send
)

type reqResp struct {
	direction direction
	reqMsg    string
	wantResp  string
	desc      string
}

func TestServer(t *testing.T) {
	t.Run("PUT request", func(t *testing.T) {
		addr := ":9999"
		srv := vcs.New()
		go func() {
			err := srv.Start(addr)
			fmt.Println(err)
		}()
		defer func() { _ = srv.Close() }()

		// Wait for server to start
		time.Sleep(500 * time.Millisecond)

		client, err := net.Dial("tcp", addr)
		require.NoError(t, err)

		testCases := []reqResp{
			{
				direction: recv,
				wantResp:  "READY\n",
				desc:      "initial 'READY' response",
			},
			{
				direction: send,
				reqMsg:    "PUT /test.txt 14\n",
				desc:      "PUT request part 1",
			},
			{
				direction: send,
				reqMsg:    "Hello, World!\n",
				desc:      "PUT request part 2",
			},
			{
				direction: recv,
				wantResp:  "OK r1\n",
				desc:      "PUT response",
			},
			{
				direction: recv,
				wantResp:  "READY\n",
				desc:      "PUT complete 'READY' response",
			},
			{
				direction: send,
				reqMsg:    "GET /test.txt\n",
				desc:      "GET request",
			},
			{
				direction: recv,
				wantResp:  "OK 14\n",
				desc:      "GET response part 1",
			},
			{
				direction: recv,
				wantResp:  "Hello, World!\n",
				desc:      "GET response part 2",
			},
			{
				direction: recv,
				wantResp:  "READY\n",
				desc:      "GET complete 'READY' response",
			},
		}

		rdr := bufio.NewReader(client)
		w := bufio.NewWriter(client)
		for _, tc := range testCases {
			t.Run(tc.desc, func(t *testing.T) {
				switch tc.direction {
				case recv:
					resp, err := rdr.ReadString('\n')
					require.NoError(t, err)
					require.Equal(t, tc.wantResp, resp, tc.desc)
				case send:
					_, err := w.WriteString(tc.reqMsg)
					require.NoError(t, err)
					err = w.Flush()
					require.NoError(t, err)
				}
			})
		}

	})
}

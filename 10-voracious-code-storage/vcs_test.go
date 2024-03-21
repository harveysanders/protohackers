package vcs_test

import (
	"bufio"
	"fmt"
	"net"
	"sync"
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

		clientA, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		rdrA := bufio.NewReader(clientA)
		wA := bufio.NewWriter(clientA)

		clientB, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		rdrB := bufio.NewReader(clientB)
		wB := bufio.NewWriter(clientB)

		clientAReqResp := []reqResp{
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

		clientBReqResp := []reqResp{
			{
				direction: recv,
				wantResp:  "READY\n",
				desc:      "initial 'READY' response",
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

		expectedBRespCount := 0
		for _, r := range clientBReqResp {
			if r.direction == recv {
				expectedBRespCount++
			}
		}

		clientBResponses := make([]string, 0, expectedBRespCount)

		for _, tc := range clientAReqResp {
			t.Run(tc.desc, func(t *testing.T) {
				switch tc.direction {
				case recv:
					resp, err := rdrA.ReadString('\n')
					require.NoError(t, err)
					require.Equal(t, tc.wantResp, resp, tc.desc)
				case send:
					_, err := wA.WriteString(tc.reqMsg)
					require.NoError(t, err)
					err = wA.Flush()
					require.NoError(t, err)
				}
			})
		}

		wg := sync.WaitGroup{}
		wg.Add(len(clientBReqResp))
		responseIndexes := make([]int, 0, len(clientBReqResp))
		go func() {
			for i, tc := range clientBReqResp {
				switch tc.direction {
				case recv:
					resp, err := rdrB.ReadString('\n')
					require.NoError(t, err)
					clientBResponses = append(clientBResponses, resp)
					responseIndexes = append(responseIndexes, i)
				case send:
					_, err := wB.WriteString(tc.reqMsg)
					require.NoError(t, err)
					err = wB.Flush()
					require.NoError(t, err)
				}
				wg.Done()
			}
		}()

		wg.Wait()
		for i, wantIndex := range responseIndexes {
			wantResp := clientBReqResp[wantIndex]
			gotResp := clientBResponses[i]
			if wantResp.direction == recv {
				require.Equal(t, wantResp.wantResp, gotResp, wantResp.desc)
			}
		}
	})
}

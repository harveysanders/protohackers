package jobcentre_test

import (
	"bufio"
	"log"
	"net"
	"testing"
	"time"

	jobcentre "github.com/harveysanders/protohackers/9-job-centre"
	"github.com/harveysanders/protohackers/9-job-centre/inmem"
	"github.com/harveysanders/protohackers/9-job-centre/jcp"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	addr := ":9999"
	store := inmem.NewQueue()
	srv := jobcentre.NewServer(store)

	go func() {
		err := jcp.ListenAndServe(addr, srv)
		if err != nil {
			log.Fatal(err)
		}
	}()

	time.Sleep(100 * time.Millisecond)
	client, err := net.Dial("tcp", addr)
	if err != nil {
		require.NoError(t, err)
	}

	requests := []string{
		`{"request":"put","queue":"queue1","job":{"title":"example-job"},"pri":123}`,
		`{"request":"get","queues":["queue1"]}`,
		`{"request":"abort","id":10001}`,
		`{"request":"get","queues":["queue1"]}`,
		`{"request":"delete","id":10001}`,
		`{"request":"get","queues":["queue1"]}`,
		// `{"request":"get","queues":["queue1"],"wait":true}`,
	}

	wantResp := []string{
		`{"status":"ok","id":10001}`,
		`{"status":"ok","id":10001,"job":{"title":"example-job"},"queue":"queue1","pri":123}`,
		`{"status":"ok"}`,
		`{"status":"ok","id":10001,"job":{"title":"example-job"}, "queue":"queue1","pri":123}`,
		`{"status":"ok"}`,
		`{"status":"no-job"}`,
	}

	bufRdr := bufio.NewReader(client)
	for i, req := range requests {
		_, err := client.Write([]byte(req + "\n"))
		require.NoError(t, err)

		gotResp, err := bufRdr.ReadBytes('\n')
		require.NoError(t, err)
		require.Equal(t, wantResp[i]+"\n", string(gotResp))
	}
}

package jobcentre_test

import (
	"bufio"
	"context"
	"log"
	"net"
	"sync"
	"testing"
	"time"

	jobcentre "github.com/harveysanders/protohackers/9-job-centre"
	"github.com/harveysanders/protohackers/9-job-centre/inmem"
	"github.com/harveysanders/protohackers/9-job-centre/jcp"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	addr := ":9999"
	store := inmem.NewStore()
	srv := &jcp.Server{
		Addr:    addr,
		Handler: jobcentre.NewApp(store),
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()

	defer srv.Close(context.Background())

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
		`{"status":"ok","id":10001,"job":{"title":"example-job"},"queue":"queue1","pri":123}`,
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

func TestErrors(t *testing.T) {
	type ReqWantResp struct {
		req      string
		wantResp string
	}
	t.Run("6errors.test", func(t *testing.T) {
		addr := ":9998"
		store := inmem.NewStore()
		handler := jobcentre.NewApp(store)

		srv := &jcp.Server{
			Addr:    addr,
			Handler: handler,
		}

		go func() {
			_ = srv.ListenAndServe()
		}()

		defer srv.Close(context.Background())

		time.Sleep(100 * time.Millisecond)

		clientAReqResps := []ReqWantResp{
			{
				req:      `{"queue":"q-qSaTxrlY","request":"put","job":{"title":"j-kWFumbG4"},"pri":100}`,
				wantResp: `{"status":"ok","id":10001}`,
			},
			{
				req:      `{"request":"abort","id":10201}`,
				wantResp: `{"status":"no-job"}`, // job is not be assigned yet
			},
			{
				req:      `{"queues":["q-qSaTxrlY"],"request":"get"}`,
				wantResp: `{"status":"ok","id":10002,"job":{"title":"j-fhLupEsm"},"queue":"q-qSaTxrlY","pri":100}`,
			},
		}

		clientBReqResps := []ReqWantResp{
			{
				req:      `{"queue":"q-qSaTxrlY","request":"put","job":{"title":"j-fhLupEsm"},"pri":100}`,
				wantResp: `{"status":"ok","id":10002}`,
			},
			{
				req:      `{"queues":["q-qSaTxrlY"],"request":"get"}`,
				wantResp: `{"status":"ok","id":10001,"job":{"title":"j-kWFumbG4"},"queue":"q-qSaTxrlY","pri":100}`,
			},
		}

		clientA, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		clientB, err := net.Dial("tcp", addr)
		require.NoError(t, err)

		bufRdrA := bufio.NewReader(clientA)
		bufRdrB := bufio.NewReader(clientB)

		gotBResponses := make([]string, 0, 2)
		wg := sync.WaitGroup{}
		wg.Add(len(clientBReqResps))
		go func() {
			for {
				gotRespB, err := bufRdrB.ReadBytes('\n')
				require.NoError(t, err)
				gotBResponses = append(gotBResponses, string(gotRespB))
				wg.Done()
			}
		}()

		for i, convoA := range clientAReqResps {
			_, err := clientA.Write([]byte(convoA.req + "\n"))
			require.NoError(t, err)

			if i > 0 && i < len(clientBReqResps)+1 {
				time.Sleep(100 * time.Millisecond)
				_, err := clientB.Write([]byte(clientBReqResps[i-1].req + "\n"))
				require.NoError(t, err)
			}

			gotResp, err := bufRdrA.ReadBytes('\n')
			require.NoError(t, err)
			require.JSONEq(t, convoA.wantResp+"\n", string(gotResp))
		}

		wg.Wait()
		for i, b := range clientBReqResps {
			require.JSONEq(t, b.wantResp+"\n", gotBResponses[i])
		}
	})

	t.Run("unable to delete aborted job", func(t *testing.T) {
		addr := ":9997"
		srv := &jcp.Server{
			Addr:    addr,
			Handler: jobcentre.NewApp(inmem.NewStore()),
		}

		go func() {
			_ = srv.ListenAndServe()
		}()

		defer srv.Close(context.Background())

		time.Sleep(100 * time.Millisecond)

		client18, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		client19, err := net.Dial("tcp", addr)
		require.NoError(t, err)

		bufRdr18 := bufio.NewReader(client18)
		bufRdr19 := bufio.NewReader(client19)

		_, err = client18.Write([]byte(`{"job":{"title":"j-e6vUG0t5"},"pri":100,"queue":"q-np36thox","request":"put"}` + "\n"))
		require.NoError(t, err)

		_, err = client19.Write([]byte(`{"pri":100,"job":{"title":"j-wJg3D6NQ"},"request":"put","queue":"q-np36thox"}` + "\n"))
		require.NoError(t, err)

		gotResp, err := bufRdr18.ReadBytes('\n')
		require.NoError(t, err)
		require.JSONEq(t, `{"status":"ok","id":10001}`+"\n", string(gotResp))

		gotResp, err = bufRdr19.ReadBytes('\n')
		require.NoError(t, err)
		require.JSONEq(t, `{"status":"ok","id":10002}`+"\n", string(gotResp))

		_, err = client18.Write([]byte(`{"request":"delete","id":10001}` + "\n"))
		require.NoError(t, err)
		_, err = client19.Write([]byte(`{"request":"delete","id":10001}` + "\n"))
		require.NoError(t, err)

		gotResp, err = bufRdr18.ReadBytes('\n')
		require.NoError(t, err)
		require.JSONEq(t, `{"status":"ok"}`+"\n", string(gotResp))
		gotResp, err = bufRdr19.ReadBytes('\n')
		require.NoError(t, err)
		require.JSONEq(t, `{"status":"no-job"}`+"\n", string(gotResp))

		_, err = client18.Write([]byte(`{"request":"abort","id":10001}` + "\n"))
		require.NoError(t, err)

		gotResp, err = bufRdr18.ReadBytes('\n')
		require.NoError(t, err)
		require.JSONEq(t, `{"status":"no-job"}`+"\n", string(gotResp))
	})
}

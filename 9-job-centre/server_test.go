package jobcentre_test

import (
	"bufio"
	"context"
	"fmt"
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
			log.Println(err)
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

}

func TestServerErrors(t *testing.T) {
	type (
		clientID   int
		ReqResPair struct {
			req      string
			wantResp string
			clientID clientID
			label    string
		}

		gotResp struct {
			clientID clientID
			resp     string
		}
	)
	t.Run("unable to abort already deleted job part 2", func(t *testing.T) {
		const (
			clientID0 clientID = iota
			clientID1
		)

		addr := ":9996"
		srv := &jcp.Server{
			Addr:    addr,
			Handler: jobcentre.NewApp(inmem.NewStore()),
		}

		go func() {
			_ = srv.ListenAndServe()
		}()

		defer srv.Close(context.Background())

		time.Sleep(100 * time.Millisecond)

		requests := []ReqResPair{
			{
				req:      `{"pri":100,"queue":"q-31hlLeih","request":"put","job":{"title":"j-PJrtLHI1"}}`,
				wantResp: `{"status":"ok","id":10001}`,
				clientID: clientID0,
				label:    "[0] PUT j-PJrtLHI1",
			},
			{
				req:      `{"queue":"q-31hlLeih","pri":100,"job":{"title":"j-5sDhysOG"},"request":"put"}`,
				wantResp: `{"status":"ok","id":10002}`,
				clientID: clientID1,
				label:    "[1] PUT j-5sDhysOG",
			},
			{
				req:      `{"request":"abort","id":10002}`,
				wantResp: `{"status":"no-job"}`,
				clientID: clientID1,
				label:    "[1] ABORT 10002 - not assigned",
			},
			{
				req:      `{"queues":["q-31hlLeih"],"request":"get"}`,
				wantResp: `{"status":"ok","id":10002,"job":{"title":"j-5sDhysOG"},"queue":"q-31hlLeih","pri":100}`,
				clientID: clientID1,
				label:    "[1] GET",
			},
			{
				req:      `{"queues":["q-31hlLeih"],"request":"get"}`,
				wantResp: `{"status":"ok","id":10001,"job":{"title":"j-PJrtLHI1"},"queue":"q-31hlLeih","pri":100}`,
				clientID: clientID0,
				label:    "[0] GET",
			},
			{
				req:      `{"request":"delete","id":10002}`,
				wantResp: `{"status":"ok"}`,
				clientID: clientID1,
				label:    "[0] DELETE 10002",
			},
			{
				req:      `{"request":"abort","id":10001}`,
				wantResp: `{"status":"no-job"}`,
				clientID: clientID1,
				label:    "[1] ABORT 10001 - assigned to another client",
			},
			{
				req:      `{"request":"delete","id":10001}`,
				wantResp: `{"status":"ok"}`,
				clientID: clientID0,
				label:    "[0] DELETE 10001",
			},
			{
				req:      `{"request":"abort","id":10001}`,
				wantResp: `{"status":"no-job"}`,
				clientID: clientID0,
				label:    "[0] ABORT 10001 - already deleted",
			},
		}

		client0, err := net.Dial("tcp", addr)
		require.NoError(t, err)

		client1, err := net.Dial("tcp", addr)
		require.NoError(t, err)

		bufRdr0 := bufio.NewReader(client0)
		bufRdr1 := bufio.NewReader(client1)

		var mu sync.Mutex
		clientResponses := make([]gotResp, 0, len(requests))

		wg := sync.WaitGroup{}
		wg.Add(len(requests))
		go func() {
			for {
				resp, err := bufRdr0.ReadBytes('\n')
				require.NoError(t, err)
				mu.Lock()
				clientResponses = append(clientResponses, gotResp{clientID: clientID0, resp: string(resp)})
				mu.Unlock()
				wg.Done()
			}
		}()

		go func() {
			for {
				resp, err := bufRdr1.ReadBytes('\n')
				require.NoError(t, err)
				mu.Lock()
				clientResponses = append(clientResponses, gotResp{clientID: clientID1, resp: string(resp)})
				mu.Unlock()
				wg.Done()
			}
		}()

		for _, reqResp := range requests {
			client := client0
			if reqResp.clientID == clientID1 {
				client = client1
			}
			_, err := client.Write([]byte(reqResp.req + "\n"))
			require.NoError(t, err)

			time.Sleep(10 * time.Millisecond)
		}

		wg.Wait()
		for i, got := range clientResponses {
			require.Equal(t, requests[i].clientID, got.clientID, requests[i].label)
			require.JSONEq(t, requests[i].wantResp+"\n", got.resp, requests[i].label)
		}
	})

	t.Run("assign job to only one client, even if one is waiting", func(t *testing.T) {
		const (
			clientAlpha clientID = iota
			clientBravo
		)

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

		requests := []ReqResPair{
			{
				req:      `{"pri":100,"queue":"q-31hlLeih","request":"put","job":{"title":"j-llEdLIEk"}}`,
				wantResp: `{"status":"ok","id":10001}`,
				clientID: clientAlpha,
				label:    "[0] PUT j-llEdLIEk",
			},
			{
				req:      `{"queues":["q-hotdogs"],"request":"get", "wait":true}`,
				wantResp: `{"status":"ok","id":10002,"job":{"title":"j-coney"},"queue":"q-hotdogs","pri":100}`,
				clientID: clientAlpha,
				label:    "[0] GET - wait 10002",
			},
			{
				req:      `{"id": 10002,"request":"delete"}`,
				wantResp: `{"status":"ok"}`,
				clientID: clientAlpha,
				label:    "[0] DELETE 10002",
			},
			{
				req:      `{"queue":"q-hotdogs","pri":100,"job":{"title":"j-coney"},"request":"put"}`,
				wantResp: `{"status":"ok","id":10002}`,
				clientID: clientBravo,
				label:    "[1] PUT j-coney",
			},
			{
				req:      `{"queues":["q-hotdogs"],"request":"get", "wait":false}`,
				wantResp: `{"status":"no-job"}`,
				clientID: clientBravo,
				label:    "[1] GET - wait",
			},
		}

		clientA, err := net.Dial("tcp", addr)
		require.NoError(t, err)

		clientB, err := net.Dial("tcp", addr)
		require.NoError(t, err)

		bufRdrA := bufio.NewReader(clientA)
		bufRdrB := bufio.NewReader(clientB)

		var mu sync.Mutex
		alphaResponses := make([]gotResp, 0, len(requests))
		bravoResponses := make([]gotResp, 0, len(requests))

		wg := sync.WaitGroup{}
		wg.Add(len(requests))
		go func() {
			for {
				resp, err := bufRdrA.ReadBytes('\n')
				require.NoError(t, err)
				mu.Lock()
				alphaResponses = append(alphaResponses, gotResp{clientID: clientAlpha, resp: string(resp)})
				mu.Unlock()
				wg.Done()
			}
		}()

		go func() {
			for {
				resp, err := bufRdrB.ReadBytes('\n')
				require.NoError(t, err)
				mu.Lock()
				bravoResponses = append(bravoResponses, gotResp{clientID: clientBravo, resp: string(resp)})
				mu.Unlock()
				wg.Done()
			}
		}()

		for _, reqResp := range requests {
			client := clientA
			if reqResp.clientID == clientBravo {
				client = clientB
			}
			_, err := client.Write([]byte(reqResp.req + "\n"))
			require.NoError(t, err)

			time.Sleep(10 * time.Millisecond)
		}

		wg.Wait()
		iAlpha := 0
		iBravo := 0
		for _, req := range requests {
			testLabel := fmt.Sprintf("%s\nwant: %s", req.label, req.wantResp)
			switch req.clientID {
			case clientAlpha:
				require.JSONEq(t, req.wantResp+"\n", alphaResponses[iAlpha].resp, testLabel)
				iAlpha++
			case clientBravo:
				require.JSONEq(t, req.wantResp+"\n", bravoResponses[iBravo].resp, testLabel)
				iBravo++
			}
		}
	})
}

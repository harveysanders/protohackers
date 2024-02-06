package isl_test

import (
	"context"
	"io"
	"net"
	"sync"
	"testing"

	isl "github.com/harveysanders/protohackers/8-insecure-sockets-layer"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	var wg sync.WaitGroup

	port := "9999"
	server := isl.Server{}
	err := server.Start(port)
	require.NoError(t, err)
	ctx := context.Background()

	wg.Add(1)
	go func() {
		defer wg.Done()

		err := server.Serve(ctx)
		if err != nil {
			t.Logf("server.Serve(): %v", err)
			return
		}
	}()

	conn, err := net.Dial("tcp", server.Address())
	require.NoError(t, err)

	msgs := []struct {
		req      []byte
		wantResp []byte
	}{
		{
			req: []byte{
				// xor(123),addpos,reversebits
				0x02, 0x7b, 0x05, 0x01, 0x00,
				//  4x dog,5x car\n
				0xf2, 0x20, 0xba, 0x44, 0x18, 0x84, 0xba, 0xaa, 0xd0, 0x26, 0x44, 0xa4, 0xa8, 0x7e,
			},
			// 5x car\n (encrypted)
			wantResp: []byte{0x72, 0x20, 0xba, 0xd8, 0x78, 0x70, 0xee},
		},
	}

	for _, m := range msgs {
		_, err := conn.Write(m.req)
		require.NoError(t, err)

		resp := make([]byte, 5000)
		n, err := conn.Read(resp)
		require.NotErrorIs(t, io.EOF, err)

		require.Equal(t, m.wantResp, resp[:n])
	}

	_ = conn.Close()
	_ = server.Stop()
	wg.Wait()
}

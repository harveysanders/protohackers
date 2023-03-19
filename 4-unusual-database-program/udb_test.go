package udb_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/harveysanders/protohackers/udb"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	remoteAddr := "localhost:9002"
	raddr, err := net.ResolveUDPAddr("udp", remoteAddr)
	require.NoError(t, err)

	srv := udb.NewServer()
	ctx, cancel := context.WithCancel(context.Background())
	go srv.ServeUDP(ctx, remoteAddr)

	t.Run("accepts an insert request", func(t *testing.T) {
		time.Sleep(time.Second / 2)
		conn, err := net.DialUDP("udp", nil, raddr)
		require.NoError(t, err)
		defer conn.Close()

		n, err := conn.Write([]byte("foo=bar"))
		require.NoError(t, err)
		require.Greater(t, n, 0)

		n, err = conn.Write([]byte("foo"))
		require.NoError(t, err)
		require.Greater(t, n, 0)

		want := []byte("foo=bar")
		resp := make([]byte, len(want))
		n, addr, err := conn.ReadFrom(resp)
		require.NoError(t, err)
		require.Greater(t, n, 0)
		require.Equal(t, "127.0.0.1:9002", addr.String())
		require.Equal(t, want, resp)

	})

	time.Sleep(time.Second)
	cancel()
}

func TestIsInsert(t *testing.T) {
	testCases := []struct {
		data []byte
		want bool
	}{
		{[]byte("hello=world"), true},
	}

	for _, tc := range testCases {
		t.Run("identify Inserts and Retrievals", func(t *testing.T) {
			got := udb.IsInsert(tc.data)
			require.Equal(t, tc.want, got)
		})
	}
}

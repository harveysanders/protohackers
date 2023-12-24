package udb_test

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	udb "github.com/harveysanders/protohackers/4-unusual-database-program"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	remoteAddr := "localhost:9002"
	raddr, err := net.ResolveUDPAddr("udp", remoteAddr)
	require.NoError(t, err)

	store := udb.NewStoreMap()
	srv := udb.NewServer(store)
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

	t.Run("handles multiple inserts", func(t *testing.T) {
		time.Sleep(time.Second / 2)

		inserts := []string{
			"foo=bar",
			"user.43947=LargeNewbie777",
			"user.43948=TinyEdward385",
			"user.43949=BigDev265",
			"user.43950=SmallBob270",
			"user.43951=RichCaster737",
			"user.43952=RichCaster360",
			"user.43953=CrazyFrank221",
			"user.43954=ProtoFred116",
			"user.43955=SmallDev210",
			"user.43956=CrazyNewbie967",
			"user.43957=PoorSmith169",
			"user.43958=SmallBob302",
			"user.43959=ProtoNewbie189",
			"user.43960=RedFred191",
			"user.43961=RedCoder738",
			"user.43962=GreenCoder299",
			"user.43963=GreenSmith559",
			"user.43964=BigHatter325",
			"user.43965=PoorDev203",
			"user.43966=BlueEdward926",
		}

		conn, err := net.DialUDP("udp", nil, raddr)
		require.NoError(t, err)
		defer conn.Close()

		for _, iq := range inserts {
			n, err := conn.Write([]byte(iq))
			require.NoError(t, err)
			require.Equal(t, n, len(iq))
		}

		time.Sleep(time.Second / 2)
		check := inserts[5]
		kv := strings.SplitN(check, "=", 2)
		query := kv[0]
		n, err := conn.Write([]byte(query))

		require.NoError(t, err)
		require.Equal(t, n, len(query))

		res := make([]byte, len(check))
		n, fromAddr, err := conn.ReadFrom(res)
		require.NoError(t, err)
		require.Equal(t, string([]byte(check)), string(res[:n]))
		require.Equal(t, raddr.String(), fromAddr.String())

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

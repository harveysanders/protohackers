package udb_test

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	remoteAddr := "localhost:9002"
	raddr, err := net.ResolveUDPAddr("udp", remoteAddr)
	require.NoError(t, err)

	t.Run("accepts an insert request", func(t *testing.T) {
		conn, err := net.DialUDP("udp", nil, raddr)
		require.NoError(t, err)
		defer conn.Close()

	})
}

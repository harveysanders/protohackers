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

func TestServer(t *testing.T) {
	t.Run("PUT request", func(t *testing.T) {
		addr := ":9999"
		srv := vcs.New()
		go func() {
			err := srv.Start(addr)
			fmt.Println(err)
		}()

		time.Sleep(500 * time.Millisecond)

		client, err := net.Dial("tcp", addr)
		require.NoError(t, err)

		rdr := bufio.NewReader(client)
		line, err := rdr.ReadBytes('\n')
		require.NoError(t, err)
		require.Equal(t, "READY\n", string(line))

		_, err = client.Write([]byte("PUT /test.txt 14\n"))
		require.NoError(t, err)

		_, err = client.Write([]byte("Hello, World!\n"))
		require.NoError(t, err)

		line, err = rdr.ReadBytes('\n')
		require.NoError(t, err)
		require.Equal(t, "OK r1\n", string(line))

		line, err = rdr.ReadBytes('\n')
		require.NoError(t, err)
		require.Equal(t, "READY\n", string(line))

		_ = srv.Close()
	})
}

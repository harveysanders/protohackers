package main

import (
	"bytes"
	"io"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSmokeTest(t *testing.T) {
	// TODO: Fix Test
	// Server code passes the protohacker tests, but this test times out because the client.Read() below blocks and never completes.
	// Need to investigate why.
	//  https://go.dev/play/p/hoRi3-gkei4
	// Discussion https://gophers.slack.com/archives/C02A8LZKT/p1678373886731539
	// https://go.dev/play/p/P68iiBHEC0Y
	// https://go.dev/play/p/pwg0LmkQoY_K
	t.Run("echos data sent from client", func(t *testing.T) {
		// Create a pipe to simulate a TCP network connection without starting a server.
		// https://stackoverflow.com/a/41668611/5275148
		client, server := net.Pipe()

		// Fail early for any deadlock issues.
		// Successful runs should complete well before this deadline.
		client.SetReadDeadline(time.Now().Add(time.Second / 2))

		// Run the server logic in another goroutine
		done := make(chan struct{})
		go func() {
			defer close(done)
			handleConnection(server, 0)
		}()

		want := []byte("helloooo")
		wantLen := int64(len(want))
		got := make([]byte, wantLen)

		// Write message to client (send to server)
		// LimitReader used to simulate io.EOF after writing the message.
		r := io.LimitReader(bytes.NewReader(want), wantLen)
		nbw, err := io.Copy(client, r)
		require.NoError(t, err)

		require.Equal(t, nbw, wantLen, "should write bytes to client")

		// Read response from server
		_, err = client.Read(got)
		require.NoError(t, err)

		require.Equal(t, want, got)

		client.Close()
		<-done
	})
}

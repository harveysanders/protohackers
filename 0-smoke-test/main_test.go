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
	t.Run("echos data sent from client", func(t *testing.T) {
		// Create a pipe to simulate a TCP network connection without starting a server.
		// https://stackoverflow.com/a/41668611/5275148
		client, server := net.Pipe()

		// Fail early for any deadlock issues.
		// Successful runs should complete well before this deadline.
		client.SetReadDeadline(time.Now().Add(time.Second / 2))

		// Run the server logic in another goroutine
		go func() {
			handleConnection(server, 0)
		}()

		want := []byte("helloooo")
		got := make([]byte, len(want))

		// Write message to client (send to server)
		// LimitReader used to simulate io.EOF after writing the message.
		r := io.LimitReader(bytes.NewReader(want), int64(len(want)))
		_, err := io.Copy(client, r)
		require.NoError(t, err)

		// Read response from server
		_, err = client.Read(got)
		require.NoError(t, err)

		require.Equal(t, want, got)

		client.Close()
	})
}

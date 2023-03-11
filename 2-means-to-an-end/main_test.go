package meanstoend_test

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"testing"
	"time"

	m2e "github.com/harveysanders/protohackers/meanstoend"
	"github.com/stretchr/testify/require"
)

func TestInsertMessageParse(t *testing.T) {
	testCases := []struct {
		raw  []byte
		want m2e.InsertMessage
	}{
		{
			raw: []byte{0x49, 0x00, 0x00, 0x30, 0x39, 0x00, 0x00, 0x00, 0x65},
			want: m2e.InsertMessage{
				Type:      "I",
				Timestamp: 12_345,
				Price:     101},
		},
	}

	for _, tc := range testCases {
		var got m2e.InsertMessage
		err := got.Parse(tc.raw)
		require.NoError(t, err)

		require.Equal(t, tc.want, got)
	}
}

func TestQueryMessageParse(t *testing.T) {
	testCases := []struct {
		raw  []byte
		want m2e.QueryMessage
	}{
		{
			raw: []byte{0x51, 0x00, 0x00, 0x03, 0xe8, 0x00, 0x01, 0x86, 0xa0},
			want: m2e.QueryMessage{
				Type:    "Q",
				MinTime: 1_000,
				MaxTime: 100_000},
		},
	}

	for _, tc := range testCases {
		var got m2e.QueryMessage
		err := got.Parse(tc.raw)
		require.NoError(t, err)

		require.Equal(t, tc.want, got)
	}
}

func TestServer(t *testing.T) {
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

		err := m2e.HandleConnection(server, 0)
		if !errors.Is(err, io.EOF) {
			log.Println("handle connection returned:", err)
		}
	}()

	want := int32(101)
	got := make([]byte, 4)
	messages := [][]byte{
		{0x49, 0x00, 0x00, 0x30, 0x39, 0x00, 0x00, 0x00, 0x65},
		{0x49, 0x00, 0x00, 0x30, 0x3a, 0x00, 0x00, 0x00, 0x66},
		{0x49, 0x00, 0x00, 0x30, 0x3b, 0x00, 0x00, 0x00, 0x64},
		{0x49, 0x00, 0x00, 0xa0, 0x00, 0x00, 0x00, 0x00, 0x05},
		{0x51, 0x00, 0x00, 0x30, 0x00, 0x00, 0x00, 0x40, 0x00},
	}
	for _, msg := range messages {
		nSent, err := io.Copy(client, bytes.NewReader(msg))
		if err != nil {
			require.NoError(t, err)
		}
		require.Equal(t, 9, nSent)
	}

	// Read response from server
	_, err := client.Read(got)
	if err != nil {
		require.NoError(t, err)
	}

	require.Equal(t, want, got)

	client.Close()

	<-done
}

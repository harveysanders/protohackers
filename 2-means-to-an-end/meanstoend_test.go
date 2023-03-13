package meanstoend_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
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

	defer client.Close()

	// Run the server logic in another goroutine
	go func() {
		ctx := context.WithValue(context.Background(), m2e.CONNECTION_ID, "test")
		err := m2e.HandleConnection(ctx, server)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Println("handle connection returned:", err)
		}
	}()

	// 	        Hexadecimal:         |  Decoded:

	// <-- 49 00 00 30 39 00 00 00 65 I 12345 101
	// <-- 49 00 00 30 3a 00 00 00 66 I 12346 102
	// <-- 49 00 00 30 3b 00 00 00 64 I 12347 100
	// <-- 49 00 00 a0 00 00 00 00 05 I 40960 5
	// <-- 51 00 00 30 00 00 00 40 00 Q 12288 1
	// --> 00 00 00 65                  101

	messages := [][]byte{
		{0x49, 0x00, 0x00, 0x30, 0x39, 0x00, 0x00, 0x00, 0x65},
		{0x49, 0x00, 0x00, 0x30, 0x3a, 0x00, 0x00, 0x00, 0x66},
		{0x49, 0x00, 0x00, 0x30, 0x3b, 0x00, 0x00, 0x00, 0x64},
		{0x49, 0x00, 0x00, 0xa0, 0x00, 0x00, 0x00, 0x00, 0x05},
		{0x51, 0x00, 0x00, 0x30, 0x00, 0x00, 0x00, 0x40, 0x00},
	}
	want := []byte{0x0, 0x0, 0x0, 0x65} // 101 mean

	t.Run("handle 9 byte messages", func(t *testing.T) {
		got := make([]byte, 4)

		for _, msg := range messages {
			nSent, err := io.Copy(client, bytes.NewReader(msg))
			if err != nil {
				require.NoError(t, err)
			}
			require.Equal(t, int64(9), nSent)
		}

		// Read response from server
		_, err := client.Read(got)
		if err != nil {
			require.NoError(t, err)
		}

		require.Equal(t, want, got)
	})

	t.Run("handle messages 1-byte at a time", func(t *testing.T) {
		got := make([]byte, 4)

		for _, msg := range messages {
			for _, b := range msg {
				_, err := io.Copy(client, bytes.NewReader([]byte{b}))
				if err != nil {
					require.NoError(t, err)
				}
			}
		}

		// Read response from server
		_, err := client.Read(got)
		if err != nil {
			require.NoError(t, err)
		}

		require.Equal(t, want, got)
	})

	t.Run("works with dump files", func(t *testing.T) {
		got := make([]byte, 4)
		dumpPath, err := filepath.Abs("./dumps")
		require.NoError(t, err)

		dumpDir, err := os.ReadDir(dumpPath)
		require.NoError(t, err)

		for _, entry := range dumpDir {
			dump, err := os.Open(path.Join(dumpPath, entry.Name()))
			require.NoError(t, err)
			_, err = io.Copy(client, dump)
			if err != nil {
				require.NoError(t, err)
			}

			// Read response from server
			_, err = client.Read(got)
			if err != nil {
				require.NoError(t, err)
			}

			require.Equal(t, want, got)
		}
	})
}

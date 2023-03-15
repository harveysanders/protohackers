package meanstoend_test

import (
	"bytes"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
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
	port := "9867"
	srv := m2e.Server{}
	defer srv.Stop()

	// Run the server logic in another goroutine
	go func() {
		err := srv.Start(port)
		require.NoError(t, err)
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
		// Wait for server to start
		// TODO: Better way to know if server is ready?
		time.Sleep(time.Second / 4)

		client, err := net.Dial("tcp", ":"+port)
		require.NoError(t, err)

		got := make([]byte, 4)

		for _, msg := range messages {
			nSent, err := io.Copy(client, bytes.NewReader(msg))
			if err != nil {
				require.NoError(t, err)
			}
			require.Equal(t, int64(9), nSent)
		}

		// Read response from server
		_, err = client.Read(got)
		if err != nil {
			require.NoError(t, err)
		}

		require.Equal(t, want, got)
	})

	t.Run("handle messages 1-byte at a time", func(t *testing.T) {
		// Wait for server to start
		time.Sleep(time.Second / 4)

		client, err := net.Dial("tcp", ":"+port)
		require.NoError(t, err)

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
		_, err = client.Read(got)
		if err != nil {
			require.NoError(t, err)
		}

		require.Equal(t, want, got)
	})

	t.Run("works with dump files", func(t *testing.T) {
		wg := sync.WaitGroup{}

		// Wait for server to start
		time.Sleep(time.Second / 4)

		dumpPath, err := filepath.Abs("./dumps")
		require.NoError(t, err)

		dumpDir, err := os.ReadDir(dumpPath)
		require.NoError(t, err)

		for _, entry := range dumpDir {
			wg.Add(1)

			go func(entry fs.DirEntry) {
				defer wg.Done()

				client, err := net.Dial("tcp", ":"+port)
				require.NoError(t, err)
				defer client.Close()

				err = client.SetReadDeadline(time.Now().Add(time.Second * 5))
				if err != nil {
					log.Printf("%s read timeout", entry.Name())
				}

				got := make([]byte, 4)

				dump, err := os.Open(path.Join(dumpPath, entry.Name()))
				require.NoError(t, err)
				defer dump.Close()

				n, err := io.Copy(client, dump)
				if err != nil {
					require.NoError(t, err)
				}

				log.Printf("copied %d bytes from %s\n", n, entry.Name())

				// Read response from server
				_, err = client.Read(got)
				if err != nil && strings.Contains(err.Error(), "timeout") {
					require.NoError(t, err)
				}

				log.Printf("got: %v for %q\n", got, entry.Name())
				// TODO: Write better assertion
			}(entry)
		}

		// Wait for all clients to get their responses
		wg.Wait()
	})

}

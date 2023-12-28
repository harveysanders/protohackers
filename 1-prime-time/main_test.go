package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var srv *Server

func TestMain(m *testing.M) {
	go func() {
		srv = NewServer()
		err := srv.Run("9876")
		if err != nil {
			log.Fatal(err)
		}
	}()

	code := m.Run()
	srv.Close()
	os.Exit(code)
}

func TestServer(t *testing.T) {
	testCases := []struct {
		label   string
		payload []byte
		want    []byte
	}{
		{
			label:   "number:13",
			payload: []byte(`{"number":13,"method":"isPrime"}`),
			want:    []byte(`{"method":"isPrime","prime":true}`),
		},
		{
			label:   "number:3424891",
			payload: []byte(`{"number":3424891,"method":"isPrime"}`),
			want:    []byte(`{"method":"isPrime","prime":false}`),
		},
		{
			label:   "number:50726007",
			payload: []byte(`{"method":"isPrime","number":50726007}`),
			want:    []byte(`{"method":"isPrime","prime":false}`),
		},
		{
			label:   "number:19860059",
			payload: []byte(`{"number":19860059,"method":"isPrime"}`),
			want:    []byte(`{"method":"isPrime","prime":true}`),
		},
		{
			label:   "number:44941019",
			payload: []byte(`{"method":"isPrime","number":44941019}`),
			want:    []byte(`{"method":"isPrime","prime":false}`),
		},
		{
			label:   "number:910400.1234",
			payload: []byte(`{"method":"isPrime","number":910400.1234}`),
			want:    []byte(`{"method":"isPrime","prime":false}`),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			conn, err := net.Dial("tcp", ":9876")
			require.NoError(t, err)
			defer conn.Close()

			conn.SetReadDeadline(time.Now().Add(time.Second * 2))

			_, err = io.CopyN(conn, bytes.NewReader(append(tc.payload, "\n"...)), int64(len(tc.payload)+1))
			require.NoError(t, err, "could not write payload to TCP server")

			var got bytes.Buffer
			_, err = io.CopyN(&got, conn, int64(len(tc.want)+1))
			require.NoError(t, err)
			require.Equal(t, string(append(tc.want, "\n"...)), got.String())
		})
	}
}

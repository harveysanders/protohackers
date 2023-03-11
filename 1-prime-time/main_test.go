package main

import (
	"bytes"
	"fmt"
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
			payload: []byte(`{"number":13,"method":"isPrime"}\n\n`),
			want:    []byte(`{"method":"isPrime","prime":true}\n`),
		},
		// {
		// 	payload: []byte(`{"number":3424891,"method":"isPrime"}`),
		// 	want:    []byte(`{"method":"isPrime","prime":false}`),
		// },
		// {
		// 	payload: []byte(`{"method":"isPrime","number":50726007}`),
		// 	want:    []byte(`{"method":"isPrime","prime":false}`),
		// },
		// {
		// 	payload: []byte(`{"number":19860059,"method":"isPrime"}`),
		// 	want:    []byte(`{"method":"isPrime","prime":true}`),
		// },
		// {
		// 	payload: []byte(`{"method":"isPrime","number":44941019}`),
		// 	want:    []byte(`{"method":"isPrime","prime":true}`),
		// },
	}
	for _, tc := range testCases {
		t.Run(tc.label, func(t *testing.T) {
			conn, err := net.Dial("tcp", ":9876")
			require.NoError(t, err)
			defer conn.Close()

			conn.SetReadDeadline(time.Now().Add(time.Second * 2))

			if _, err := io.CopyN(conn, bytes.NewReader(tc.payload), int64(len(tc.payload)-1)); err != nil {
				t.Error("could not write payload to TCP server:", err)
			}

			var got bytes.Buffer
			_, err = io.CopyN(&got, conn, 20)
			require.NoError(t, err)
			fmt.Printf("buff %s", got.String())
		})
	}
}

package spdaemon_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/harveysanders/protohackers/spdaemon"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	port := "9999"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go spdaemon.NewServer().Start(ctx, port)

	t.Run("Example session", func(t *testing.T) {
		// wait for server to start
		time.Sleep(time.Second / 2)
		clients := []struct {
			name      string
			messages  [][]byte
			responses [][]byte
		}{
			{
				name: "cam[mile8]",
				messages: [][]byte{
					// IAmCamera{road: 123, mile: 8, limit: 60}
					{0x80, 0x00, 0x7b, 0x00, 0x08, 0x00, 0x3c},
					// Plate{plate: "UN1X", timestamp: 0}
					{0x20, 0x04, 0x55, 0x4e, 0x31, 0x58, 0x00, 0x00, 0x00, 0x00},
				}},
			{
				name: "cam[mile9]",
				messages: [][]byte{
					// IAmCamera{road: 123, mile: 9, limit: 60}
					{0x80, 0x00, 0x7b, 0x00, 0x09, 0x00, 0x3c},
					// Plate{plate: "UN1X", timestamp: 45}
					{0x20, 0x04, 0x55, 0x4e, 0x31, 0x58, 0x00, 0x00, 0x00, 0x2d},
				}},
			{
				name: "dispatcher",
				messages: [][]byte{
					//  IAmDispatcher{roads: [123]}
					{0x81, 0x01, 0x00, 0x7b},
					// Plate{plate: "UN1X", timestamp: 45}
					{0x20, 0x04, 0x55, 0x4e, 0x31, 0x58, 0x00, 0x00, 0x00, 0x2d},
				},
				responses: [][]byte{
					// Ticket{plate: "UN1X", road: 123, mile1: 8, timestamp1: 0, mile2: 9, timestamp2: 45, speed: 8000}
					{0x21, 0x04, 0x55, 0x4e, 0x31, 0x58, 0x00, 0x7b, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x2d, 0x1f, 0x40},
				},
			},
		}

		addr := fmt.Sprintf("localhost:%s", port)
		for _, client := range clients {
			conn, err := net.Dial("tcp", addr)
			require.NoError(t, err)

			for _, msg := range client.messages {
				n, err := conn.Write(msg)
				require.NoError(t, err)
				require.Greater(t, n, 0)
			}

			for _, want := range client.responses {
				got := make([]byte, len(want))
				_, err := conn.Read(got)
				require.NoError(t, err)
				require.Equal(t, want, got)
			}
		}
	})

}
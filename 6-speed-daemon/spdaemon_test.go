package spdaemon_test

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"testing"
	"time"

	"github.com/harveysanders/protohackers/spdaemon"
	"github.com/harveysanders/protohackers/spdaemon/message"
	"github.com/stretchr/testify/require"
)

func TestHeartbeat(t *testing.T) {
	port := "9999"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go spdaemon.NewServer().Start(ctx, port)

	addr := fmt.Sprintf("localhost:%s", port)

	t.Run("handle 0", func(t *testing.T) {
		// wait for server to start
		time.Sleep(time.Second / 2)

		conn, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		defer conn.Close()

		msg := []byte{0x40, 0x00, 0x00, 0x00, 0x00}

		_, err = conn.Write(msg)
		require.NoError(t, err)

	})

	t.Run("heartbeat interval", func(t *testing.T) {
		// wait for server to start
		time.Sleep(time.Second / 2)

		conn, err := net.Dial("tcp", addr)
		require.NoError(t, err)
		defer conn.Close()

		// an interval of "25" would mean a Heartbeat message every 2.5 seconds
		interval := uint32(5) // one every .5 sec
		msg := []byte{byte(message.TypeWantHeartbeat)}

		_, err = conn.Write(binary.BigEndian.AppendUint32(msg, interval))
		require.NoError(t, err)

		beatCount := 0
		timeout := time.Millisecond * 1100
		wantCount := 2
		timer := time.NewTimer(timeout)
		respChan := make(chan []byte)
		errChan := make(chan error)

		go func() {
			b := make([]byte, 1)
			for {
				if _, err := conn.Read(b); err != nil {
					errChan <- err
				}
				respChan <- b
			}
		}()

		for {
			select {
			case <-timer.C:
				require.InDelta(t, 2, beatCount, 1)
			case <-respChan:
				beatCount++
				if beatCount > wantCount {
					t.Fatalf("want %d ticks, got: %d", wantCount, beatCount)
				}
			case err := <-errChan:
				require.NoError(t, err)
			}
		}
	})
}

func TestServer(t *testing.T) {
	port := "45678"
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go spdaemon.NewServer().Start(ctx, port)

	addr := fmt.Sprintf("localhost:%s", port)

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
				},
				responses: [][]byte{
					// Ticket{plate: "UN1X", road: 123, mile1: 8, timestamp1: 0, mile2: 9, timestamp2: 45, speed: 8000}
					{0x21, 0x04, 0x55, 0x4e, 0x31, 0x58, 0x00, 0x7b, 0x00, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x09, 0x00, 0x00, 0x00, 0x2d, 0x1f, 0x40},
				},
			},
		}

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

	t.Run("does not issue multiple tickets for the same day", func(t *testing.T) {
		// wait for server to start
		time.Sleep(time.Second / 2)
		clients := []struct {
			name      string
			messages  [][]byte
			responses [][]byte
		}{
			{
				name: "Cam442",
				messages: [][]byte{
					{0x80, 0x01, 0x5b, 0x01, 0xba, 0x00, 0x50},
					{0x20, 0x07, 0x5a, 0x5a, 0x30, 0x34, 0x41, 0x42, 0x4e, 0x01, 0x85, 0x57, 0x04},
				},
			},
			{
				name: "Cam452",
				messages: [][]byte{
					{0x80, 0x01, 0x5b, 0x01, 0xc4, 0x00, 0x50},
					{0x20, 0x07, 0x5a, 0x5a, 0x30, 0x34, 0x41, 0x42, 0x4e, 0x01, 0x85, 0x58, 0x32},
				},
			},
			{
				name: "Cam461",
				messages: [][]byte{
					{0x80, 0x01, 0x5b, 0x01, 0xcd, 0x00, 0x50},
					{0x20, 0x07, 0x5a, 0x5a, 0x30, 0x34, 0x41, 0x42, 0x4e, 0x01, 0x85, 0x59, 0x6c},
				},
			},
			{
				name: "dispatcher",
				messages: [][]byte{
					//  IAmDispatcher{roads: [347]}
					{0x81, 0x01, 0x01, 0x5b},
				},
			},
		}

		for _, client := range clients {
			conn, err := net.Dial("tcp", addr)
			require.NoError(t, err)

			for _, msg := range client.messages {
				n, err := conn.Write(msg)
				require.NoError(t, err)
				require.Greater(t, n, 0)
			}

			if client.name == "dispatcher" {
				err := conn.SetReadDeadline(time.Now().Add(time.Second * 2))
				require.NoError(t, err)
				rawTickets := make([]byte, 0)

				for {
					ticketMsg := make([]byte, 25)
					n, err := conn.Read(ticketMsg)
					if errors.Is(err, os.ErrDeadlineExceeded) || len(rawTickets) > 25 {
						break
					}
					require.NoError(t, err)
					rawTickets = append(rawTickets, ticketMsg[:n]...)
				}

				require.Len(t, rawTickets, 25, "should only issue one ticket for the same day. (Each ticket is 25 bytes)")
			}
		}
	})

	t.Run("[Recreated Scenario] does not issue multiple tickets for the same day", func(t *testing.T) {
		// wait for server to start
		time.Sleep(time.Second / 2)
		clients := []struct {
			name      string
			messages  [][]byte
			responses [][]byte
		}{
			// 			2023/04/08 14:13:23 ticket history:
			// ** [MP68GVG] START **
			// [info]Day: 231080.000000: &{Plate:MP68GVG Road:19840 Mile1:596 Mile2:1 Timestamp1:61005287 Timestamp2:61034167 Speed:7416 retries:1}
			// [info]Day: 231190.000000: &{Plate:MP68GVG Road:19840 Mile1:596 Mile2:1 Timestamp1:61005287 Timestamp2:61034167 Speed:7416 retries:1}
			// ** [MP68GVG] END **
			// 2023/04/08 14:13:23 Ticket issued: &{MP68GVG 19840 542 346 61014691 61021747 10000 1}
			{
				name: "Cam_Rd19840_Mi01",
				messages: [][]byte{
					{0x80, 0x4D, 0x80, 0x00, 0x01, 0x00, 0x50},
					// {plate: "MP68GVG", timestamp: 61034167}
					{0x20,
						0x07, 0x4D, 0x50, 0x36, 0x38, 0x47, 0x56, 0x47,
						0x03, 0xA3, 0x4E, 0xB7,
					},
				},
			},
			{
				name: "Cam_Rd19840_Mi596",
				messages: [][]byte{
					{0x80, 0x4D, 0x80, 0x02, 0x54, 0x00, 0x50},
					// {plate: "MP68GVG", timestamp: 61005287}
					{0x20,
						0x07, 0x4D, 0x50, 0x36, 0x38, 0x47, 0x56, 0x47,
						0x03, 0xA2, 0xDD, 0xE7,
					},
				},
			},
			{
				name: "Cam_Rd19840_Mi542",
				messages: [][]byte{
					{0x80, 0x4D, 0x80, 0x02, 0x1E, 0x00, 0x50},
					// {plate: "MP68GVG", timestamp: 61014691}
					{0x20,
						0x07, 0x4D, 0x50, 0x36, 0x38, 0x47, 0x56, 0x47,
						0x03, 0xA3, 0x02, 0xA3,
					},
				},
			},
			{
				name: "Cam_Rd19840_Mi346",
				messages: [][]byte{
					{0x80, 0x4D, 0x80, 0x01, 0x5A, 0x00, 0x50},
					// {plate: "MP68GVG", timestamp: 61021747}
					{0x20,
						0x07, 0x4D, 0x50, 0x36, 0x38, 0x47, 0x56, 0x47,
						0x03, 0xA3, 0x1E, 0x33,
					},
				},
			},
			{
				name: "dispatcher",
				messages: [][]byte{
					//  IAmDispatcher{roads: [347, 19840]}
					{0x81, 0x02, 0x01, 0x5b, 0x4D, 0x80},
				},
			},
		}

		for _, client := range clients {
			conn, err := net.Dial("tcp", addr)
			require.NoError(t, err)

			for _, msg := range client.messages {
				n, err := conn.Write(msg)
				require.NoError(t, err)
				require.Greater(t, n, 0)
			}

			if client.name == "dispatcher" {
				err := conn.SetReadDeadline(time.Now().Add(time.Second * 2))
				require.NoError(t, err)
				rawTickets := make([]byte, 0)

				for {
					ticketMsg := make([]byte, 25)
					n, err := conn.Read(ticketMsg)
					if errors.Is(err, os.ErrDeadlineExceeded) || len(rawTickets) > 25 {
						break
					}
					require.NoError(t, err)
					rawTickets = append(rawTickets, ticketMsg[:n]...)
				}

				require.Len(t, rawTickets, 25, "should only issue one ticket for the same day. (Each ticket is 25 bytes)")
			}
		}
	})
}

func TestMetrics(t *testing.T) {
	t.Run("sends back JSON metrics", func(t *testing.T) {
		net.Dial("tcp6", "[2a09:8280:1::f:5ec]:8080")
	})
}

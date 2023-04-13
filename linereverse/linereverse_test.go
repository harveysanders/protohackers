package linereverse_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/harveysanders/protohackers/linereverse"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	serverAddress := "localhost:9002"

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		err := linereverse.New().Run(ctx, serverAddress)
		require.NoError(t, err)
	}()

	t.Run("sample session", func(t *testing.T) {
		time.Sleep(time.Second / 2)

		sessionID := 123456789
		var outBuff bytes.Buffer

		fmt.Fprintf(&outBuff, "/connect/%d/", sessionID)
		// First message is thrown away for some reason?s
		fmt.Fprintf(&outBuff, "/connect/%d/", sessionID)

		ctx, cancel := context.WithCancel(context.Background())

		client(ctx, serverAddress, &outBuff)
		defer cancel()

	})

	cancel()
}

// ****** https://hashnode.com/post/a-udp-server-and-client-in-go-cjn3fm10s00is25s1bm12gd22 ****

// client wraps the whole functionality of a UDP client that sends
// a message and waits for a response coming back from the server
// that it initially targeted.
func client(ctx context.Context, address string, reader io.Reader) (err error) {
	// Resolve the UDP address so that we can make use of DialUDP
	// with an actual IP and port instead of a name (in case a
	// hostname is specified).
	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return
	}

	// Although we're not in a connection-oriented transport,
	// the act of `dialing` is analogous to the act of performing
	// a `connect(2)` syscall for a socket of type SOCK_DGRAM:
	// - it forces the underlying socket to only read and write
	// to and from a specific remote address.
	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return
	}

	// Closes the underlying file descriptor associated with the,
	// socket so that it no longer refers to any file.
	defer conn.Close()

	doneChan := make(chan error, 1)

	go func() {
		// It is possible that this action blocks, although this
		// should only occur in very resource-intensive situations:
		// - when you've filled up the socket buffer and the OS
		// can't dequeue the queue fast enough.
		n, err := io.Copy(conn, reader)
		if err != nil {
			doneChan <- err
			return
		}

		fmt.Printf("packet-written: bytes=%d\n", n)

		buffer := make([]byte, 1024)

		// Set a deadline for the ReadOperation so that we don't
		// wait forever for a server that might not respond on
		// a reasonable amount of time.
		deadline := time.Now().Add(time.Minute)
		err = conn.SetReadDeadline(deadline)
		if err != nil {
			doneChan <- err
			return
		}

		nRead, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			doneChan <- err
			return
		}

		fmt.Printf("packet-received: bytes=%d from=%s\n",
			nRead, addr.String())

		doneChan <- nil
	}()

	select {
	case <-ctx.Done():
		fmt.Println("cancelled")
		err = ctx.Err()
	case err = <-doneChan:
		fmt.Println("done")
	}

	return
}

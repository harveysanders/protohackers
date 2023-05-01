package linereverse_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/harveysanders/protohackers/linereverse"
	"github.com/stretchr/testify/require"
)

func TestServer(t *testing.T) {
	serverAddress := "localhost:9002"

	ctx, stopServer := context.WithCancel(context.Background())
	go func() {
		err := linereverse.New().Run(ctx, serverAddress)
		require.NoError(t, err)
	}()

	t.Run(`The client connects with session token 12345, sends "Hello, world!" and then closes the session.`, func(t *testing.T) {
		time.Sleep(time.Second / 2)

		// Pass client address so it remains the same through UDP dials.
		clientAddress := "localhost:9998"

		sessionID := 12345
		var outBuf bytes.Buffer
		var inBuf bytes.Buffer
		ctx, stopClient := context.WithCancel(context.Background())
		defer stopClient()

		// <-- /connect/12345/
		fmt.Fprintf(&outBuf, "/connect/%d/", sessionID)
		client(ctx, &serverAddress, &clientAddress, &outBuf, &inBuf)

		// --> /ack/12345/0/
		require.Equal(t, fmt.Sprintf("/ack/%d/0/", sessionID), inBuf.String())

		// Reset buffers between calls
		outBuf.Reset()
		inBuf.Reset()

		// <-- /data/12345/0/Hello, world!/
		data := "Hello, world!"
		fmt.Fprintf(&outBuf, "/data/%d/0/%s/", sessionID, data)
		client(ctx, &serverAddress, &clientAddress, &outBuf, &inBuf)

		// --> /ack/12345/13/
		require.Equal(t, fmt.Sprintf("/ack/%d/%d/", sessionID, len(data)), inBuf.String())

		outBuf.Reset()
		inBuf.Reset()

		// <-- /close/12345/
		closeMsg := fmt.Sprintf("/close/%d/", sessionID)
		client(ctx, &serverAddress, &clientAddress, strings.NewReader(closeMsg), &inBuf)
		// --> /close/12345/
		require.Equal(t, closeMsg, inBuf.String())

	})

	t.Run("Reverses a line in an LRCP message", func(t *testing.T) {
		time.Sleep(time.Second / 2)
		sessionID := randPort()
		clientAddress := fmt.Sprintf("localhost:%s", sessionID)

		var outBuf bytes.Buffer
		var inBuf bytes.Buffer
		ctx, stopClient := context.WithCancel(context.Background())
		defer stopClient()

		testCases := []struct {
			msg         string
			wantReplies []string
			msgDesc     string
			replyDesc   string
		}{
			{
				msgDesc:     "<-- /connect/12345/",
				msg:         fmt.Sprintf("/connect/%s/", sessionID),
				replyDesc:   "--> /ack/12345/0/",
				wantReplies: []string{fmt.Sprintf("/ack/%s/0/", sessionID)},
			},
			{
				msgDesc:   "<-- /data/12345/0/hello\n/",
				msg:       fmt.Sprintf("/data/%s/0/hello\n/", sessionID),
				replyDesc: "reply with ack and reversed data",
				wantReplies: []string{
					fmt.Sprintf("/ack/%s/6/", sessionID),
					fmt.Sprintf("/data/%s/0/olleh\n", sessionID),
				},
			},
			// x <-- /connect/12345/
			// x --> /ack/12345/0/
			// x <-- /data/12345/0/hello\n/
			// x --> /ack/12345/6/
			// TODO:
			// --> /data/12345/0/olleh\n/
			// <-- /ack/12345/6/
			// <-- /data/12345/6/Hello, world!\n/
			// --> /ack/12345/20/
			// --> /data/12345/6/!dlrow ,olleH\n/
			// <-- /ack/12345/20/
			// <-- /close/12345/
			// --> /close/12345/
		}

		for _, tc := range testCases {
			t.Run(tc.msgDesc, func(t *testing.T) {
				// Reset buffers between calls
				outBuf.Reset()
				inBuf.Reset()

				fmt.Fprintf(&outBuf, tc.msg)
				client(ctx, &serverAddress, &clientAddress, &outBuf, &inBuf)

				require.Equal(t, tc.wantReplies[0], inBuf.String())

				if len(tc.wantReplies) > 1 {
					outBuf.Reset()
					inBuf.Reset()
					client(ctx, &serverAddress, &clientAddress, &outBuf, &inBuf)

					require.Equal(t, tc.wantReplies[1], inBuf.String())
				}
			})
		}

	})
	stopServer()
}

// ****** https://hashnode.com/post/a-udp-server-and-client-in-go-cjn3fm10s00is25s1bm12gd22 ****

// client wraps the whole functionality of a UDP client that sends
// a message and waits for a response coming back from the server
// that it initially targeted.
func client(ctx context.Context, remoteAddress, localAddress *string, r io.Reader, w io.Writer) (err error) {
	// Resolve the UDP address so that we can make use of DialUDP
	// with an actual IP and port instead of a name (in case a
	// hostname is specified).
	remoteAddr, err := net.ResolveUDPAddr("udp", *remoteAddress)
	if err != nil {
		return err
	}

	// Allow setting the local address if needed. If nil passed, choose a random address.
	var localAddr *net.UDPAddr
	if localAddress != nil {
		localAddr, err = net.ResolveUDPAddr("udp", *localAddress)
		if err != nil {
			return err
		}
	}

	// Although we're not in a connection-oriented transport,
	// the act of `dialing` is analogous to the act of performing
	// a `connect(2)` syscall for a socket of type SOCK_DGRAM:
	// - it forces the underlying socket to only read and write
	// to and from a specific remote address.
	conn, err := net.DialUDP("udp", localAddr, remoteAddr)
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
		n, err := io.Copy(conn, r)
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

		if _, err := io.CopyN(w, bytes.NewBuffer(buffer), int64(nRead)); err != nil {
			doneChan <- err
			return
		}
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

func randPort() string {
	min := 10000
	max := 12000
	port := rand.Intn(max-min) + min
	return fmt.Sprint(port)
}

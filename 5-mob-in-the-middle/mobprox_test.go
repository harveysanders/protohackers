package mobprox_test

import (
	"net"
	"testing"
	"time"

	mobprox "github.com/harveysanders/protohackers/5-mob-in-the-middle"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
)

type mockChatServer struct {
	recvMsgs [][]byte
}

func (m *mockChatServer) start(t *testing.T) (string, error) {
	l, err := nettest.NewLocalListener("tcp")
	if err != nil {
		return "", err
	}

	go func() {
		conn, err := l.Accept()
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		conn.Write([]byte(`Welcome to the chat server!
What should we call you?
`))
		for {
			buf := make([]byte, 1024)
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			m.recvMsgs = append(m.recvMsgs, buf[:n])
		}

	}()

	return l.Addr().String(), nil
}

func TestServer(t *testing.T) {
	port := "9876"
	upstreamServer := &mockChatServer{}
	upstreamAddr, err := upstreamServer.start(t)
	require.NoError(t, err)

	time.Sleep(10 * time.Millisecond)
	go func() {
		srv := mobprox.NewServer(upstreamAddr, "tonyBcoinAddress")
		err := srv.Start(port)
		require.NoError(t, err)
	}()

	t.Run("does not send message to upstream if there is no trailing newline", func(t *testing.T) {
		// Wait for server to start
		time.Sleep(500 * time.Millisecond)

		conn, err := net.Dial("tcp", ":"+port)
		require.NoError(t, err)
		defer conn.Close()

		// Read welcome message
		buf := make([]byte, 1024)
		_, err = conn.Read(buf)
		require.NoError(t, err)

		// Send message to server without a trailing newline
		_, err = conn.Write([]byte("robot_tester123"))
		require.NoError(t, err)

		// Wait for server to process message
		time.Sleep(100 * time.Millisecond)

		require.Equal(t, 0, len(upstreamServer.recvMsgs))
	})
}

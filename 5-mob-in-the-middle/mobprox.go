package mobprox

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/textproto"
)

type (
	Server struct {
		listener     net.Listener
		upstreamAddr string
		interceptor  interceptor
	}

	interceptor interface {
		intercept([]byte) []byte
	}

	client struct {
		id string
	}

	ctxKey string

	direction int
)

const CLIENT_ID ctxKey = "CLIENT_ID"
const (
	FROM_CLIENT direction = iota
	FROM_UPSTREAM
)

func NewServer(upstreamAddr, boguscoinAddress string) *Server {
	return &Server{
		upstreamAddr: upstreamAddr,
		interceptor:  newbcoinReplacer(boguscoinAddress),
	}
}

func (s *Server) Start(port string) error {
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return err
	}
	defer l.Close()

	s.listener = l

	clientID := 0
	for {
		clientID++
		conn, err := s.listener.Accept()
		if err != nil {
			log.Printf("accept: %v", err)
			continue
		}
		ctx := context.WithValue(context.Background(), ctxKey(CLIENT_ID), fmt.Sprintf("%d", clientID))

		go func(conn net.Conn, clientID int) {
			if err := s.handleConnection(ctx, conn); err != nil {
				log.Printf("downstream error: %v", err)
				if err := conn.Close(); err != nil {
					log.Printf("close: %v", err)
				}
			}
		}(conn, clientID)
	}
}

func (s *Server) handleConnection(ctx context.Context, down net.Conn) error {
	up, err := net.Dial("tcp", s.upstreamAddr)
	if err != nil {
		return err
	}

	clientID := ctx.Value(CLIENT_ID).(string)
	client := newClient(clientID)

	go client.proxy(ctx, up, down, s.interceptor, FROM_CLIENT)
	go client.proxy(ctx, down, up, s.interceptor, FROM_UPSTREAM)

	return nil
}

func newClient(id string) *client {
	client := &client{id: id}
	return client
}

// proxy reads from the src connection,
func (c *client) proxy(
	ctx context.Context,
	dst io.WriteCloser,
	src io.ReadCloser,
	interceptor interceptor,
	dir direction,
) {
	dstName := "CLIENT"
	srcName := "UPSTREAM"
	if dir == FROM_CLIENT {
		dstName = "UPSTREAM"
		srcName = "CLIENT"
	}

	// Read from the source connection
	for {
		conn := textproto.NewReader(bufio.NewReader(src))
		msg, err := conn.ReadLineBytes()
		if err != nil {
			log.Printf("[%s|%s]read error: %v\n", c.id, srcName, err)
			dst.Close()
			src.Close()
			return
		}

		// Replace the newline removed from ReadLineBytes()
		msg = append(msg, '\n')
		log.Printf("\n← [%s|%s] (%d B):\n%s\n", c.id, srcName, len(msg), msg)

		// Replace message contents
		msg = interceptor.intercept(msg)

		// Proxy the message out to the destination
		nWrote, err := dst.Write(msg)
		if err != nil {
			log.Printf("[%s|%s]write error: %v\n", c.id, dstName, err)
			dst.Close()
			src.Close()
			return
		}

		log.Printf("\n→ [%s|%s] (%d B):\n%s\n", c.id, dstName, nWrote, msg)
	}
}

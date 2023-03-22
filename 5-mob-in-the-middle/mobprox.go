package mobprox

import (
	"context"
	"io"
	"log"
	"net"
)

type (
	Server struct {
		listener     net.Listener
		upstreamAddr string
	}

	client struct {
		ctx  context.Context
		up   net.Conn
		down net.Conn
	}

	ctxKey string

	direction int
)

const CLIENT_ID ctxKey = "CLIENT_ID"
const (
	FROM_CLIENT direction = iota
	FROM_UPSTREAM
)

func NewServer(upstreamAddr string) *Server {
	return &Server{
		upstreamAddr: upstreamAddr,
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
		ctx := context.WithValue(context.Background(), ctxKey(CLIENT_ID), clientID)

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

func (s *Server) handleConnection(ctx context.Context, conn net.Conn) error {
	usConn, err := net.Dial("tcp", s.upstreamAddr)
	if err != nil {
		return err
	}
	newClient(ctx, usConn, conn)

	return nil
}

func newClient(ctx context.Context, up, down net.Conn) *client {

	client := &client{
		ctx:  ctx,
		up:   up,
		down: down,
	}

	go client.proxy(ctx, up, down, hijackMsg, FROM_CLIENT)
	go client.proxy(ctx, down, up, hijackMsg, FROM_UPSTREAM)

	return client
}

// proxy reads from the src connection,
func (c *client) proxy(ctx context.Context, dst io.WriteCloser, src io.ReadCloser, transform func([]byte) []byte, dir direction) {
	// TODO: Use transform

	dstName := "CLIENT"
	srcName := "UPSTREAM"
	if dir == FROM_CLIENT {
		dstName = "UPSTREAM"
		srcName = "CLIENT"
	}

	// Read from the source connection
	buf := make([]byte, 1024)
	nRead, err := src.Read(buf)
	if err != nil {
		log.Printf("[%s]read error: %v\n", srcName, err)
		dst.Close()
		src.Close()
		return
	}

	// Grab the only the message bytes from the buffer
	msg := make([]byte, nRead)
	copy(msg, buf)
	log.Printf("\n← [%s]:\n%s\n", srcName, msg)

	// Proxy the message out to the destination
	nWrote, err := dst.Write(msg)
	if err != nil {
		log.Printf("[%s]write error: %v\n", dstName, err)
		dst.Close()
		src.Close()
		return
	}

	log.Printf("\n→ [%s] (%d B):\n%s\n", dstName, nWrote, msg)
}

func hijackMsg(in []byte) []byte {
	out := make([]byte, len(in))
	// TODO: Find and replace Boguscoin addresses
	copy(out, in)
	return out
}

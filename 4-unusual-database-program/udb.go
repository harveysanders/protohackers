package udb

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"
)

type (
	server struct {
		readTimeout   time.Duration
		writeTimeout  time.Duration
		maxBufferSize int
	}
)

func NewServer() *server {
	return &server{
		readTimeout:   time.Second * 10,
		writeTimeout:  time.Second * 10,
		maxBufferSize: 1024,
	}
}

func (s *server) ServeUDP(ctx context.Context, address string) error {
	pConn, err := net.ListenPacket("udp", address)
	if err != nil {
		return fmt.Errorf("listenPacket: %w", err)
	}
	defer pConn.Close()

	done := make(chan error, 1)
	incoming := make([]byte, s.maxBufferSize)

	go func() {
		for {
			n, fromAddr, err := pConn.ReadFrom(incoming)
			if err != nil {
				done <- fmt.Errorf("readFrom: %w", err)
				return
			}

			log.Printf("incoming packet from %s..len: %d\ncontents: %s\n", fromAddr.String(), n, incoming)

			err = pConn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
			if err != nil {
				done <- err
				return
			}

			pConn.WriteTo(incoming[:n], fromAddr)
			if err != nil {
				done <- err
				return
			}

			log.Printf("wrote echo packet to %s..len: %d\n", fromAddr.String(), n)
		}
	}()

	select {
	case <-ctx.Done():
		log.Printf("cancelled with err: %v", ctx.Err())
	case err = <-done:
		log.Printf("err: %v", err)
	}

	return nil
}

package udb

import (
	"context"
	"io"
	"net"
)

// Inspired by blog post, "A UDP server and client in Go", by Ciro S. Costa
// https://dev.to/cirowrc/a-udp-server-and-client-in-go-3g8n

type (
	Client struct {
		conn       *net.UDPConn
		done       chan error
		remoteAddr *net.UDPAddr
	}
)

func NewClient(ctx context.Context, address string, rdr io.Reader) (*Client, error) {
	client := &Client{
		done: make(chan error, 1),
	}
	raddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return client, err
	}

	client.remoteAddr = raddr

	conn, err := net.DialUDP("udp", nil, raddr)
	if err != nil {
		return client, err
	}

	client.conn = conn
	return client, nil

	// go func() {
	// 	n, err := io.Copy(conn, rdr)
	// 	if err != nil {
	// 		done <- err
	// 		return
	// 	}

	// 	log.Printf("wrote packet. len: %d bytes\n", n)

	// 	incoming := make([]byte, 1024)

	// 	nRead, fromAddr, err := conn.ReadFrom(incoming)
	// 	if err != nil {
	// 		done <- err
	// 		return
	// 	}

	// 	log.Printf("recv packet from %s len: %d bytes\n", fromAddr, nRead)

	// 	done <- nil
	// }()

	// select {
	// case <-ctx.Done():
	// 	log.Println("cancelled")
	// 	err := ctx.Err()
	// 	if err != nil {
	// 		log.Println(err)
	// 	}
	// case err := <-done:
	// 	log.Println(err)
	// }

	// return nil
}

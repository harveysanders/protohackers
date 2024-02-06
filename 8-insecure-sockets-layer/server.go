package isl

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/harveysanders/protohackers/8-insecure-sockets-layer/orders"
)

type Server struct {
	l net.Listener
}

func (s *Server) Start(port string) error {
	l, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		return err
	}
	s.l = l
	return nil
}

func (s *Server) Stop() error {
	return s.l.Close()
}

func (s *Server) Serve(ctx context.Context) error {
	for {
		conn, err := s.l.Accept()
		if err != nil {
			return fmt.Errorf("l.Accept: %w", err)
		}

		go handleConnection(ctx, conn)
	}
}

func (s *Server) Address() string {
	return s.l.Addr().String()
}

func handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	var (
		nRead int64
		// nWritten int
	)
	buf := bytes.NewBuffer(make([]byte, 0, 5000))
	tr := io.TeeReader(conn, buf)
	// Read cipher spec
	cip := NewCipher()
	n, err := cip.ReadFrom(tr)
	if err != nil {
		fmt.Printf("newCipher: %v", err)
		return
	}

	nRead += n
	// Discard cipher spec
	_, err = io.CopyN(io.Discard, buf, n)
	if err != nil {
		fmt.Printf("io.CopyN: %v", err)
		return
	}
	sd := NewStreamDecoder(buf, *cip, int(nRead))

	scr := bufio.NewScanner(sd)
	for scr.Scan() {
		line := scr.Bytes()
		toy, err := orders.MostCopies(line)
		log.Print(toy)
		if err != nil {
			fmt.Printf("orders.MostCopies: %v", err)
			return
		}

	}
	if err := scr.Err(); err != nil {
		fmt.Printf("scr.Err(): %v", err)
		return
	}
}

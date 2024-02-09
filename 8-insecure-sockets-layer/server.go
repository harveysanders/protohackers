package isl

import (
	"bufio"
	"context"
	"fmt"
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
	fmt.Printf("Server listening on %s\n", s.Address())
	clientID := 0
	for {
		clientID++
		conn, err := s.l.Accept()
		if err != nil {
			log.Print("Client connection closed\n")
			return nil
		}

		go handleConnection(ctx, conn, clientID)
	}
}

func (s *Server) Address() string {
	return s.l.Addr().String()
}

func handleConnection(ctx context.Context, conn net.Conn, clientID int) {
	defer func() {
		fmt.Printf("[%d]: handler complete\n", clientID)
		conn.Close()
	}()
	const maxMessageLen = 5000
	var (
		nRead    int // Total bytes read from the stream, not including the cipher spec.
		nWritten int // Total bytes written to the stream.
	)

	cipherSpec := NewCipher()
	n, err := cipherSpec.ReadFrom(conn)
	fmt.Printf("read %d bytes from cipher spec\n", n)
	if err != nil {
		if err == ErrNoOpCipher {
			fmt.Printf("cipher spec is a no-op")
			return
		}
		fmt.Printf("newCipher: %v", err)
		return
	}

	// Stream start pos begins immediately after cipher spec.
	// nRead is 0 at this point.
	sd := NewStreamDecoder(conn, *cipherSpec, nRead)
	scr := bufio.NewScanner(sd)
	scr.Buffer(make([]byte, maxMessageLen), maxMessageLen)

	for scr.Scan() {
		line := scr.Bytes()
		fmt.Printf("[%d]: Received: %s\n", clientID, string(line))
		// Re add the newline stripped by the scanner
		line = append(line, '\n')
		nRead += len(line)
		toy, err := orders.MostCopies(line)
		if err != nil {
			fmt.Printf("orders.MostCopies: %v", err)
			return
		}

		fmt.Printf("[%d]: Sending: %s\n", clientID, string(toy))
		encoded := cipherSpec.Encode(toy, nWritten)
		n, err := conn.Write(encoded)
		nWritten += n
		if err != nil {
			fmt.Printf("conn.Write: %v", err)
			return
		}
	}

	if err := scr.Err(); err != nil {
		fmt.Printf("scr.Err(): %v", err)
		return
	}
}

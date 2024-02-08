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

	connBuf := bytes.NewBuffer(make([]byte, 0, maxMessageLen))
	// Split the stream into two readers so we can read the cipher spec
	// and then decode the rest of the stream.
	// The cipher spec has an unknown length, so we can't use a fixed size buffer. If we reused the same reader passed to cipherSpec.ReadFrom,
	// We may read past the end of the cipher spec and into the message.
	// TODO: There may be a more efficient way to do this.
	tr := io.TeeReader(conn, connBuf)
	cipherSpec := NewCipher()
	n, err := cipherSpec.ReadFrom(tr)
	if err != nil {
		if err == ErrNoOpCipher {
			fmt.Printf("cipher spec is a no-op")
			return
		}
		fmt.Printf("newCipher: %v", err)
		return
	}

	// Discard cipher spec from the connection buffer,
	// since we've already read it from the tee reader.
	_, err = io.CopyN(io.Discard, connBuf, n)
	if err != nil {
		fmt.Printf("io.CopyN: %v", err)
		return
	}

	// Stream start pos begins immediately after cipher spec.
	// nRead is 0 at this point.
	sd := NewStreamDecoder(connBuf, *cipherSpec, nRead)
	scr := bufio.NewScanner(sd)

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

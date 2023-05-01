package vcs

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
)

type (
	Server struct{}

	Message struct {
		method     string // "PUT"
		filePath   string // "/test.txt"
		contents   []byte // ASCII
		contentLen int    // 14
	}
)

func New() *Server {
	return &Server{}
}

func (s *Server) Start(port string) (net.Listener, error) {
	l, err := net.Listen("tcp", port)
	if err != nil {
		return l, err
	}

	for {
		c, err := l.Accept()
		if err != nil {
			log.Printf("CLIENT: %v", err)
			continue
		}
		go s.handleConnection(c)
	}
}

func (s *Server) handleConnection(c net.Conn) {
	n, err := c.Write([]byte("READY\n"))
	if err != nil {
		log.Printf("write: %v", err)
		return
	}
	fmt.Printf("wrote %d bytes to client\n", n)

	// TODO: Read line by line
	rawMsg, err := io.ReadAll(c)
	if err != nil {
		log.Printf("readAll: %v", err)
		return
	}

	fmt.Printf("incoming:\n%s\n", rawMsg)
	fmt.Printf("bytes:\n%v\n", rawMsg)

	var m Message
	err = m.parse(rawMsg)
	if err != nil {
		log.Printf("parse: %v", err)
		return
	}

}

func (m *Message) parse(raw []byte) error {
	lines := bytes.SplitN(raw, []byte("\n"), 2)
	if len(lines) < 2 {
		return fmt.Errorf("expected at least 2 lines, got %d", len(lines))
	}

	fields := bytes.Fields(lines[0])

	fmt.Printf("fields: %+v\n", fields)
	m.method = string(fields[0])
	m.filePath = string(fields[1])
	contentLen, err := strconv.Atoi(string(fields[2]))
	if err != nil {
		return fmt.Errorf("atoi: %w", err)
	}
	m.contentLen = contentLen

	contents := lines[1]
	if len(contents) < contentLen {
		return fmt.Errorf("expected content length of %d bytes, got %d", contentLen, len(contents))
	}
	m.contents = contents[:contentLen]
	return nil
}

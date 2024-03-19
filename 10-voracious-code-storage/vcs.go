package vcs

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"strconv"
)

type (
	Server struct {
		listener net.Listener
	}

	Message struct {
		method       string // "PUT"
		filePath     string // "/test.txt"
		contents     []byte // ASCII
		contentLen   int    // 14
		needsContent bool
	}
)

func New() *Server {
	return &Server{}
}

func (s *Server) Start(address string) error {
	l, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}

	s.listener = l

	for {
		c, err := l.Accept()
		if err != nil {
			log.Printf("CLIENT: %v", err)
			continue
		}
		go s.handleConnection(c)
	}
}

func (s *Server) Close() error {
	return s.listener.Close()
}

func (s *Server) handleConnection(c net.Conn) {
	n, err := c.Write([]byte("READY\n"))
	if err != nil {
		log.Printf("write: %v", err)
		return
	}
	fmt.Printf("wrote %d bytes to client\n", n)

	var m Message
	scr := bufio.NewScanner(c)
	for scr.Scan() {
		if scr.Err() != nil {
			log.Printf("scan: %v", scr.Err())
			return
		}

		line := scr.Bytes()
		// Replace the newline
		line = append(line, '\n')
		if err != nil {
			log.Printf("readAll: %v", err)
			return
		}

		fmt.Printf("incoming:\n%s\n", line)
		if m.needsContent {
			err = m.parseContent(line)
		} else {
			err = m.parseMeta(line)
		}
		if err != nil {
			log.Printf("parse: %v", err)
			return
		}

		fmt.Printf("msg: %+v\n", m)
		n, err := c.Write([]byte("READY\n"))
		if err != nil {
			log.Printf("write: %v", err)
			return
		}
		fmt.Printf("wrote %d bytes to client\n", n)
	}
}

func (m *Message) parseMeta(line []byte) error {
	fields := bytes.Fields(line)

	fmt.Printf("fields: %+v\n", fields)
	m.method = string(fields[0])
	m.filePath = string(fields[1])
	contentLen, err := strconv.Atoi(string(fields[2]))
	if err != nil {
		return fmt.Errorf("atoi: %w", err)
	}
	m.contentLen = contentLen
	if m.contentLen > 0 {
		m.needsContent = true
	}
	return nil
}

func (m *Message) parseContent(line []byte) error {
	contents := line
	if len(contents) < m.contentLen {
		return fmt.Errorf("expected content length of %d bytes, got %d", m.contentLen, len(contents))
	}
	m.contents = contents[:m.contentLen]
	m.needsContent = false
	return nil
}

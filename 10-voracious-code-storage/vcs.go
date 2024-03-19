package vcs

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/harveysanders/protohackers/10-voracious-code-storage/inmem"
)

type (
	Server struct {
		listener net.Listener
		store    inmem.Store
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
	return &Server{
		store: *inmem.New(),
	}
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
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("CLIENT: %v", err)
			}
			return err
		}
		go s.handleConnection(c)
	}
}

func (s *Server) Close() error {
	return s.listener.Close()
}

func (s *Server) handleConnection(c net.Conn) {
	_, err := c.Write([]byte("READY\n"))
	if err != nil {
		log.Printf("write: %v", err)
		return
	}

	var msg Message
	scr := bufio.NewScanner(c)
	for scr.Scan() {
		line := scr.Bytes()
		// Replace the newline
		line = append(line, '\n')

		log.Printf("incoming:\n%s\n", line)

		// Should be on 2nd line of a PUT request
		if msg.needsContent {
			if err = msg.parseContent(line); err != nil {
				log.Printf("parseContent: %v", err)
				return
			}
			msg.needsContent = false
			_, rev, err := s.store.CreateRevision(msg.filePath, bytes.NewReader(msg.contents))
			if err != nil {
				log.Printf("CreateRevision: %v", err)
				return
			}
			_, err = c.Write([]byte(fmt.Sprintf("OK %s\n", rev)))
			if err != nil {
				log.Printf("write: %v", err)
				return
			}
		} else {
			// First pass
			// Reset the message
			msg = Message{}
			err = msg.parseMeta(line)
			if err != nil {
				log.Printf("parseMeta: %v", err)
				return
			}
		}

		if msg.needsContent {
			// Get the next line to read the contents
			continue
		}

		_, err := c.Write([]byte("READY\n"))
		if err != nil {
			log.Printf("write: %v", err)
			return
		}

	}
	if scr.Err() != nil {
		log.Printf("scan: %v", scr.Err())
		return
	}
}

func (m *Message) parseMeta(line []byte) error {
	fields := bytes.Fields(line)

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

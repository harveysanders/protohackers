package vcs

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/harveysanders/protohackers/10-voracious-code-storage/inmem"
)

const (
	ReqTypeGet  = "GET"
	ReqTypePut  = "PUT"
	ReqTypeList = "LIST"
	ReqTypeHelp = "HELP"
)

type (
	Server struct {
		listener net.Listener
		store    inmem.Store
	}

	RequestPut struct {
		method     string // "PUT"
		filePath   string // "/test.txt"
		contents   []byte // ASCII
		contentLen int    // 14
	}

	RequestGet struct {
		method   string // "GET"
		filePath string // "/test.txt"
	}

	Conn struct {
		conn net.Conn
		s    *Server
		rdr  *bufio.Reader
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

func (s *Server) handleConnection(nc net.Conn) {
	c := &Conn{
		conn: nc,
		s:    s,
		rdr:  bufio.NewReader(nc),
	}

	defer c.conn.Close()

	// Send the initial READY message
	_, err := c.conn.Write([]byte("READY\n"))
	if err != nil {
		log.Printf("write: %v", err)
		return
	}

	for {
		line, err := c.rdr.ReadBytes('\n')
		if err != nil {
			log.Printf("rdr.RadBytes: %v", err)
			return
		}

		fields := bytes.Fields(line)
		if len(fields) == 0 {
			continue
		}

		reqType := string(fields[0])
		switch reqType {
		case ReqTypeHelp:
			c.handleHelp()
		case ReqTypePut:
			c.handlePut(line)
		case ReqTypeGet:
			c.handleGet(line)
		case ReqTypeList:
			c.handleList(line)
		default:
			_, err := nc.Write([]byte("ERROR unknown command\n"))
			if err != nil {
				log.Printf("write: %v", err)
			}
		}

		// Write "READY" message after handling each request
		_, err = nc.Write([]byte("READY\n"))
		if err != nil {
			log.Printf("write: %v", err)
			return
		}
	}
}

func (m *RequestPut) unmarshal(line []byte) error {
	fields := bytes.Fields(line)

	m.method = string(fields[0])
	m.filePath = string(fields[1])
	contentLen, err := strconv.Atoi(string(fields[2]))
	if err != nil {
		return fmt.Errorf("atoi: %w", err)
	}
	m.contentLen = contentLen
	return nil
}

func (c *Conn) handlePut(line []byte) {
	var req RequestPut
	err := req.unmarshal(line)
	if err != nil {
		log.Printf("parseMeta: %v", err)
		return
	}

	req.contents = make([]byte, 0, req.contentLen)
	for bytesRead := 0; bytesRead < req.contentLen; {
		line, err := c.rdr.ReadBytes('\n')
		if err != nil {
			log.Printf("rdr.ReadBytes: %v", err)
			return
		}
		req.contents = append(req.contents, line...)
		bytesRead += len(line)
	}

	_, rev, err := c.s.store.CreateRevision(req.filePath, bytes.NewReader(req.contents))
	if err != nil {
		log.Printf("CreateRevision: %v", err)
		return
	}
	_, err = c.conn.Write([]byte(fmt.Sprintf("OK %s\n", rev)))
	if err != nil {
		log.Printf("write: %v", err)
		return
	}
}

func (m *RequestGet) unmarshal(line []byte) error {
	fields := bytes.Fields(line)
	if len(fields) != 2 {
		return fmt.Errorf("invalid request: %s", line)
	}
	m.method = string(fields[0])
	m.filePath = string(fields[1])
	return nil
}

func (c *Conn) handleGet(line []byte) {
	var req RequestGet
	if err := req.unmarshal(line); err != nil {
		log.Printf("unmarshal: %v", err)
		return
	}

	log.Print(req)
}

func (c *Conn) handleList(line []byte) {
	// TODO
}

func (c *Conn) handleHelp() {
	methods := strings.Join(
		[]string{ReqTypePut, ReqTypeGet, ReqTypeList, ReqTypeHelp},
		"|")
	_, err := c.conn.Write([]byte(fmt.Sprintf("OK usage: %s\n", methods)))
	if err != nil {
		log.Printf("write: %v", err)
		return
	}
}

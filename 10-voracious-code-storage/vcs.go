package vcs

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
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
		method     string // Method type, always "PUT".
		filePath   string // "/test.txt"
		contents   []byte // ASCII encoded file contents.
		contentLen int    // 14
	}

	RequestGet struct {
		method   string // Method Type, always "GET".
		filePath string // File path Ex: "/test.txt"
		rev      string // Optional revision number. If omitted, use latest. Ex: "r1"
	}

	Conn struct {
		conn net.Conn
		s    *Server
		rdr  *bufio.Reader
		w    *bufio.Writer
		id   int
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
	connID := 0
	for {
		c, err := l.Accept()
		if err != nil {
			if !errors.Is(err, net.ErrClosed) {
				log.Printf("[%d] CLIENT: %v", connID, err)
			}
			return err
		}
		go s.handleConnection(c, connID)
		connID++
	}
}

func (s *Server) Close() error {
	return s.listener.Close()
}

func (s *Server) handleConnection(nc net.Conn, id int) {
	c := &Conn{
		id:   id,
		conn: nc,
		s:    s,
		rdr:  bufio.NewReader(nc),
		w:    bufio.NewWriter(nc),
	}

	defer c.conn.Close()

	// Send the initial READY message
	if _, err := c.w.WriteString("READY\n"); err != nil {
		log.Printf("[%d] write: %v", c.id, err)
		return
	}
	if err := c.w.Flush(); err != nil {
		log.Printf("[%d] Flush: %v", c.id, err)
		return
	}

	for {
		line, err := c.rdr.ReadBytes('\n')
		if err != nil && err != io.EOF {
			log.Printf("[%d] rdr.ReadBytes: %v", c.id, err)
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
			if _, err := c.w.WriteString("ERROR unknown command\n"); err != nil {
				log.Printf("[%d] write: %v", c.id, err)
			}
		}

		// Write "READY" message after handling each request
		if _, err := c.w.WriteString("READY\n"); err != nil {
			log.Printf("[%d] write: %v", c.id, err)
			return
		}
		if err := c.w.Flush(); err != nil {
			log.Printf("[%d] Flush: %v", c.id, err)
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
		log.Printf("[%d] parseMeta: %v", c.id, err)
		return
	}

	req.contents = make([]byte, 0, req.contentLen)
	for bytesRead := 0; bytesRead < req.contentLen; {
		line, err := c.rdr.ReadBytes('\n')
		if err != nil {
			log.Printf("[%d] rdr.ReadBytes: %v", c.id, err)
			return
		}
		req.contents = append(req.contents, line...)
		bytesRead += len(line)
	}

	_, rev, err := c.s.store.CreateRevision(req.filePath, bytes.NewReader(req.contents))
	if err != nil {
		log.Printf("[%d] CreateRevision: %v", c.id, err)
		return
	}
	_, err = c.conn.Write([]byte(fmt.Sprintf("OK %s\n", rev)))
	if err != nil {
		log.Printf("[%d] write: %v", c.id, err)
		return
	}
}

func (r *RequestGet) unmarshal(line []byte) error {
	fields := bytes.Fields(line)
	if len(fields) < 2 {
		return fmt.Errorf("invalid request: %s", line)
	}
	r.method = string(fields[0])
	r.filePath = string(fields[1])
	if len(fields) == 3 {
		r.rev = string(fields[2])
	}
	return nil
}

func (c *Conn) handleGet(line []byte) {
	var req RequestGet
	if err := req.unmarshal(line); err != nil {
		log.Printf("[%d] unmarshal: %v", c.id, err)
		return
	}

	file, err := c.s.store.GetRevision(req.filePath, req.rev)
	if err != nil {
		log.Printf("[%d] GetRevision: %v", c.id, err)
		return
	}
	defer func() {
		_ = file.Close()
	}()
	stat, err := file.Stat()
	if err != nil {
		log.Printf("[%d] Stat: %v", c.id, err)
		return
	}
	okMsg := fmt.Sprintf("OK %d\n", stat.Size())
	_, err = c.w.WriteString(okMsg)
	if err != nil {
		log.Printf("[%d] WriteString: %v", c.id, err)
		return
	}
	if _, err := io.Copy(c.w, file); err != nil {
		log.Printf("[%d] io.Copy: %v", c.id, err)
		return
	}
	if err = c.w.Flush(); err != nil {
		log.Printf("[%d] Flush: %v", c.id, err)
		return
	}
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
		log.Printf("[%d] write: %v", c.id, err)
		return
	}
}

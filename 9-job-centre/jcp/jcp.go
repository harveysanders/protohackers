// Package jcp provides a simple "Job Centre Protocol" server. It is heavily based on Go's net/http package. The server listens for TCP connections and serves requests.
package jcp

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
)

type (
	conn struct {
		rwc    net.Conn      // Read/write TCP connection.
		id     uint64        // Unique connection ID.
		bufr   *bufio.Reader // Buffered reader.
		bufw   *bufio.Writer // Buffered writer.
		server *Server       // Associated server.
	}

	response struct {
		conn *conn
		req  *Request
	}

	Request struct {
		Body io.Reader
	}

	Server struct {
		log     *log.Logger
		Addr    string
		Handler JCPHandler
	}

	JCPResponseWriter interface {
		io.Writer
	}

	JCPHandler interface {
		ServeJCP(ctx context.Context, w JCPResponseWriter, r *Request)
	}
)

func ListenAndServe(addr string, handler JCPHandler) error {
	server := &Server{
		log:     log.Default(),
		Addr:    addr,
		Handler: handler,
	}

	return server.ListenAndServe()
}

func (s *Server) ListenAndServe() error {
	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		return err
	}
	return s.Serve(l)
}

func (s *Server) Serve(ln net.Listener) error {
	defer func() {
		s.log.Println("closing server")
		ln.Close()
	}()

	ctx := context.Background()
	var connID uint64

	for {
		tcpConn, err := ln.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// TODO: Handle timeouts with retry logic
				s.log.Println("timeout TCP connection")
				return err
			}
			return fmt.Errorf("ln.Accept: %w", err)
		}

		c := conn{
			rwc:    tcpConn,
			id:     connID,
			server: s,
		}
		go c.serve(ctx)

		connID++
	}
}

func (c *conn) serve(ctx context.Context) {
	defer c.rwc.Close()
	c.bufr = bufio.NewReader(c.rwc)
	c.bufw = bufio.NewWriter(c.rwc)

	for {
		w, err := c.readRequest(ctx)
		if err != nil {
			c.server.log.Println("readRequest:", err)
			return
		}

		c.server.Handler.ServeJCP(ctx, w, w.req)
	}
}

func (c *conn) readRequest(ctx context.Context) (*response, error) {
	line, err := c.bufr.ReadBytes('\n')
	if err != nil {
		return &response{}, fmt.Errorf("bufr.ReadBytes: %w", err)
	}

	w := &response{
		conn: c,
		req: &Request{
			Body: bytes.NewReader(line),
		},
	}
	return w, nil
}

func (w *response) Write(data []byte) (int, error) {
	return w.conn.rwc.Write(data)
}
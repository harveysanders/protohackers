package budgetchat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/textproto"
	"regexp"
	"strings"
)

type (
	Server struct {
		listener net.Listener
		hub      *hub
	}

	client struct {
		joined bool
		name   string
		conn   net.Conn
		send   chan []byte
		hub    *hub
	}

	ctxKey string
)

var (
	ErrNameTooShort = "name much be at least 1 character"
	ErrInvalidChar  = "contains non alphanumeric character"
)

const CONNECTION_ID ctxKey = "CONNECTION_ID"

func (s *Server) HandleConnection(ctx context.Context, conn net.Conn) error {
	if _, err := conn.Write([]byte("New chat server. Who dis?\n")); err != nil {
		return err
	}

	c := textproto.NewConn(conn)
	rawName, err := c.ReadLineBytes()
	if err != nil {
		return err
	}

	rawName = bytes.TrimSpace(rawName)
	if err := ValidateName(rawName); err != nil {
		if _, err := conn.Write([]byte(err.Error())); err != nil {
			log.Printf("write invalid name: %v", err)
		}
		return err
	}

	if s.hub.isNameTaken(string(rawName)) {
		err := fmt.Errorf("username %q is unavailable", rawName)
		if _, err := conn.Write([]byte(err.Error() + ". Got another?\n")); err != nil {
			log.Printf("write invalid name: %v", err)
		}
		return err
	}

	newClient(string(rawName), conn, s.hub)

	return nil
}

func ValidateName(name []byte) error {
	if len(name) == 0 {
		return errors.New(ErrNameTooShort)
	}
	re := regexp.MustCompile(`[^(a-zA-Z0-9)]`)
	invalidChar := re.Find(name)
	if invalidChar != nil {
		return fmt.Errorf("%s: %s", ErrInvalidChar, string(invalidChar))
	}
	return nil
}

func newClient(name string, conn net.Conn, hub *hub) *client {
	c := &client{
		name:   name,
		joined: true,
		conn:   conn,
		send:   make(chan []byte, 1024),
		hub:    hub,
	}

	c.hub.join <- c

	go c.readPump()
	go c.writePump()

	return c
}

// ReadPump reads messages from the client's connection.
func (c *client) readPump() {
	defer func() {
		c.hub.leave <- c
		c.conn.Close()
	}()

	for {
		conn := textproto.NewConn(c.conn)
		msg, err := conn.ReadLineBytes()
		if err != nil {
			if err == io.EOF {
				log.Printf("*** EOF ***")
				break
			}
			log.Printf("[%s] readLineBytes: %v", c.name, err)
			break
		}
		var m strings.Builder
		m.WriteString(fmt.Sprintf("[%s] ", c.name))
		m.Write(msg)
		m.WriteByte('\n')

		c.hub.broadcast <- message{from: c.name, payload: []byte(m.String())}
	}
}

func (c *client) writePump() {
	defer func() {
		c.conn.Close()
	}()

	for {
		msg := <-c.send
		n, err := c.conn.Write(msg)
		if err != nil {
			log.Printf("[%s] write: %v", c.name, err)
			break
		}
		log.Printf("[%s] wrote %d bytes", c.name, n)
	}
}

func NewServer() *Server {
	return &Server{
		hub: newHub(),
	}
}

func (s *Server) Start(port string) error {
	go s.hub.run()

	l, err := net.Listen("tcp", ":"+port)
	s.listener = l

	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	clientID := 0
	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}

		clientID++
		go func(conn net.Conn, clientID int) {
			ctx := context.WithValue(context.Background(), CONNECTION_ID, clientID)

			if err := s.HandleConnection(ctx, conn); err != nil {
				log.Printf("client [%d] cause error:\n%v\nclosing connection..", clientID, err)
				if err := conn.Close(); err != nil {
					log.Printf("close: %x\n", err)
				}
			}

		}(conn, clientID)
	}
}

func (s *Server) Stop() error {
	return s.listener.Close()
}

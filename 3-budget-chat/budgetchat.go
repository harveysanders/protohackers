package budgetchat

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/textproto"
	"regexp"
)

type (
	Server struct {
		listener net.Listener
	}

	client struct {
		joined bool
		name   string
		conn   net.Conn
	}

	ctxKey string
)

var (
	ErrNameTooShort = "name much be at least 1 character"
	ErrInvalidChar  = "contains non alphanumeric character"
)

const CONNECTION_ID ctxKey = "CONNECTION_ID"

func HandleConnection(ctx context.Context, conn net.Conn) error {
	if _, err := conn.Write([]byte("New chat server. Who dis?\n")); err != nil {
		return err
	}

	c := textproto.NewConn(conn)
	userName, err := c.ReadLineBytes()
	if err != nil {
		return err
	}

	userName = bytes.TrimSpace(userName)
	if err := ValidateName(userName); err != nil {
		if _, err := conn.Write([]byte(err.Error())); err != nil {
			log.Printf("write invalid name: %v", err)
		}
		return err
	}

	client := newClient(string(userName), conn)
	log.Printf("client: %+v", client)
	// TODO: Add client to hub
	// TODO: Announce Presence
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

func newClient(name string, conn net.Conn) *client {
	return &client{
		name:   name,
		joined: true,
		conn:   conn,
	}
}

func (s *Server) Start(port string) error {
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

			if err := HandleConnection(ctx, conn); err != nil {
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

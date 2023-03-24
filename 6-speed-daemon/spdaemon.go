package spdaemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"

	"github.com/harveysanders/protohackers/spdaemon/message"
)

type (
	Server struct {
		listener    net.Listener
		cams        []*Camera
		dispatchers []*TicketDispatcher
	}

	Camera struct {
		message.IAmCamera
	}

	TicketDispatcher struct {
	}

	ctxKey string

	ServerError struct {
		Msg string
	}

	ClientError struct {
		Msg string
	}
)

const CONNECTION_ID ctxKey = "CONNECTION_ID"

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Start(port string) error {
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.listener = l

	clientID := 0
	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}

		clientID++
		go func(conn net.Conn, clientID int) {
			ctx := context.WithValue(context.Background(), ctxKey(CONNECTION_ID), fmt.Sprintf("%d", clientID))

			if err := s.HandleConnection(ctx, conn); err != nil {
				log.Printf("client [%d] cause error:\n%v\nclosing connection..", clientID, err)
				if err := conn.Close(); err != nil {
					log.Printf("close: %x\n", err)
				}
			}

		}(conn, clientID)
	}
}

func (s *Server) HandleConnection(ctx context.Context, conn net.Conn) error {
	// Identify the client
	err := s.addClient(ctx, conn)
	if err != nil {
		var clientErr *ClientError
		switch {
		case errors.As(err, &clientErr):
			// TODO: Marshall message.Error and send back to client
		default: // Server Error
			log.Printf("addClient: %v", err)
		}
		return conn.Close()
	}
	return nil
}

// AddClient identifies a client from it's message type and add them to the appropriate client bucket (cams or dispatchers).
func (s *Server) addClient(ctx context.Context, conn net.Conn) error {
	var msg []byte
	_, err := conn.Read(msg)
	if err != nil {
		return &ServerError{fmt.Sprintf("read: %w", err)}
	}

	msgType, err := message.ParseType(msg[0])
	if err != nil {
		return &ClientError{fmt.Sprintf("read: %w", err)}
	}
	switch msgType {
	case message.TypeIAmCamera:
		s.cams = append(s.cams, &Camera{ /* TODO: Add field values */ })
	case message.TypeIAmDispatcher:
		s.dispatchers = append(s.dispatchers, &TicketDispatcher{ /* TODO: Add field values */ })
	}
	return nil
}

func (e *ServerError) Error() string {
	return e.Msg
}

func (e *ClientError) Error() string {
	return e.Msg
}

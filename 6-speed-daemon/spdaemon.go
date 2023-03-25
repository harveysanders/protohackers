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

func (s *Server) Start(ctx context.Context, port string) error {
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Printf("Speed Daemon listening @ %s", l.Addr().String())

	s.listener = l

	clientID := 0
	for {
		conn, err := l.Accept()
		if err != nil {
			return fmt.Errorf("accept: %w", err)
		}

		clientID++
		go func(conn net.Conn, clientID int) {
			ctx := context.WithValue(ctx, ctxKey(CONNECTION_ID), fmt.Sprintf("%d", clientID))

			if err := s.HandleConnection(ctx, conn); err != nil {
				log.Printf("client [%d] cause error:\n%v\nclosing connection..", clientID, err)
				if err := conn.Close(); err != nil {
					log.Printf("close: %x\n", err)
				}
			}

		}(conn, clientID)

		select {
		case <-ctx.Done():
			log.Printf("cancelled with err: %v", ctx.Err())
			l.Close()
		}
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
	buf := make([]byte, 1024)
	clientID := ctx.Value(CONNECTION_ID)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return &ServerError{fmt.Sprintf("read: %v", err)}
		}
		for offset := 0; offset < n; {
			// Read the first byte to get the message type
			msgType, err := message.ParseType(buf[offset])
			if err != nil {
				return &ClientError{fmt.Sprintf("read: %v", err)}
			}

			// Get the expected length of the message
			// Start at the 2nd byte, since first is the message type
			msgLen := msgType.Len(buf[offset:])
			// Create a byte slice for the message size
			msg := make([]byte, msgLen)
			// Copy needed bytes from buffer
			copy(msg, buf[offset:])
			// Move offset
			offset += msgLen
			// Handle message

			switch msgType {
			case message.TypeIAmCamera:
				log.Printf("[%s]TypeIAmCamera: %x", clientID, msg)
				s.addCamera(ctx, msg, conn)
			case message.TypeIAmDispatcher:
				log.Printf("[%s]TypeIAmDispatcher: %x", clientID, msg)
				s.dispatchers = append(s.dispatchers, &TicketDispatcher{ /* TODO: Add field values */ })
			case message.TypePlate:
				log.Printf("[%s]TypePlate: %x", clientID, msg)
			case message.TypeTicket:
				log.Printf("[%s]TypeTicket: %x", clientID, msg)
			case message.TypeWantHeartbeat:
				log.Printf("[%s]TypeWantHeartbeat: %x", clientID, msg)
			case message.TypeHeartbeat:
				log.Printf("[%s]TypeHeartbeat: %x", clientID, msg)
			}
		}
	}
}

func (s *Server) addCamera(ctx context.Context, msg []byte, conn net.Conn) error {
	cam := Camera{conn: conn}
	err := cam.UnmarshalBinary(msg)
	if err != nil {
		return fmt.Errorf("unmarshalBinary: %w", err)
	}
	s.cams = append(s.cams, &cam)
	return nil
}

func (e *ServerError) Error() string {
	return e.Msg
}

func (e *ClientError) Error() string {
	return e.Msg
}

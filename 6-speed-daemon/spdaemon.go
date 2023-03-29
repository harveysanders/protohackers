package spdaemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/harveysanders/protohackers/spdaemon/message"
)

type (
	Server struct {
		listener    net.Listener
		cams        map[uint16]map[uint16]*Camera        // [Road ID][mile]:cam
		dispatchers map[uint16]*TicketDispatcher         // [road ID]:dispatcher
		plates      map[uint16]map[string][]*observation // [road ID][plate]
		ticketQueue ticketQueue
	}

	// Observation represents an event when a car's plate was captured on a certain road at a specific time and location.
	observation struct {
		plate     string
		mile      uint16
		timestamp time.Time
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
	return &Server{
		cams:        make(map[uint16]map[uint16]*Camera, 0),
		dispatchers: make(map[uint16]*TicketDispatcher, 0),
		plates:      make(map[uint16]map[string][]*observation, 0),
		ticketQueue: make(ticketQueue),
	}
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
		default:
			continue
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
	// Client will be a cam or a dispatcher
	var meCam Camera

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
				s.addCamera(ctx, msg, &meCam)
			case message.TypeIAmDispatcher:
				log.Printf("[%s]TypeIAmDispatcher: %x", clientID, msg)
			case message.TypePlate:
				log.Printf("[%s]TypePlate: %x", clientID, msg)
				s.handlePlate(ctx, msg, meCam)
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

func (s *Server) addCamera(ctx context.Context, msg []byte, cam *Camera) error {
	if err := cam.UnmarshalBinary(msg); err != nil {
		return fmt.Errorf("unmarshalBinary: %w", err)
	}
	if _, ok := s.cams[cam.Road]; !ok {
		s.cams[cam.Road] = make(map[uint16]*Camera, 0)
	}
	s.cams[cam.Road][cam.Mile] = cam
	return nil
}

func (s *Server) handlePlate(ctx context.Context, msg []byte, cam Camera) {
	p := message.Plate{}
	p.UnmarshalBinary(msg)
	if _, ok := s.plates[cam.Road]; !ok {
		s.plates[cam.Road] = make(map[string][]*observation)
	}

	// Check if plate has been seen on the same road before
	obs, ok := s.plates[cam.Road][p.Plate]
	latest := observation{
		plate:     p.Plate,
		timestamp: p.Timestamp,
		mile:      cam.Mile,
	}
	if !ok {
		// If not, register the plate
		s.plates[cam.Road][p.Plate] = []*observation{&latest}
		return
	}
	// If seen before
	// iterate over the records and calculate the average speed
	if v := checkViolation(latest, obs, float64(cam.Limit)); v != nil {
		v.Road = cam.Road
		log.Printf("violation: %+v", v)
		s.ticketQueue.add(cam.Road, v)
	}
	// Add observation
	s.plates[cam.Road][p.Plate] = append(s.plates[cam.Road][p.Plate], &latest)
}

func (e *ServerError) Error() string {
	return e.Msg
}

func (e *ClientError) Error() string {
	return e.Msg
}

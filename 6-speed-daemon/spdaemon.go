package spdaemon

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"time"

	"github.com/harveysanders/protohackers/spdaemon/message"
)

type (
	Server struct {
		listener    net.Listener
		mu          sync.Mutex
		dispatchers map[uint16][]*TicketDispatcher       // [road ID]:dispatcher
		plates      map[uint16]map[string][]*observation // [road ID][plate]
		ticketQueue ticketQueue
	}

	// Observation represents an event when a car's plate was captured on a certain road at a specific time and location.
	observation struct {
		plate     string
		mile      uint16
		timestamp time.Time
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
		dispatchers: make(map[uint16][]*TicketDispatcher, 0),
		plates:      make(map[uint16]map[string][]*observation, 0),
		ticketQueue: make(ticketQueue, 2048),
	}
}

func (s *Server) Start(ctx context.Context, port string) error {
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Printf("Speed Daemon listening @ %s", l.Addr().String())

	s.listener = l

	go s.ticketListen()

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
	clientID := ctx.Value(CONNECTION_ID)
	err := s.addClient(ctx, conn)
	if err != nil {
		var clientErr *ClientError
		switch {
		case errors.As(err, &clientErr):
			// TODO: Marshall message.Error and send back to client
		default: // Server Error
			log.Printf("[%s] addClient: %v", clientID, err)
		}
		return conn.Close()
	}
	return nil
}

// AddClient identifies a client from it's message type and add them to the appropriate client bucket (cams or dispatchers).
func (s *Server) addClient(ctx context.Context, conn net.Conn) error {
	clientID := ctx.Value(CONNECTION_ID)
	// Client will be a cam or a dispatcher
	var meCam Camera

	var heartbeatTicker *time.Ticker
	defer func() {
		if heartbeatTicker != nil {
			heartbeatTicker.Stop()
		}
		// TODO: Unregister dispatcher
	}()

	r := bufio.NewReader(conn)
	for {
		msgHdr, err := r.Peek(1)
		if err != nil {
			return &ServerError{fmt.Sprintf("msg header peek: %v", err)}
		}
		// Read the first byte to get the message type
		msgType, err := message.ParseType(msgHdr[0])
		if err != nil {
			invalidMsg, err := r.Peek(10)
			if err != nil {
				log.Printf("problem peek invalid message: %v", err)
			}
			log.Printf("invalid message type: %v\n%x", err, invalidMsg)
			return &ClientError{fmt.Sprintf("invalid message type: %v", err)}
		}

		// Calc the expected length of the message.
		// The next 2 bytes contain enough info to calc the length of the complete message.
		lenHdr, err := r.Peek(2)
		if err != nil {
			return &ServerError{fmt.Sprintf("length header peak: %v", err)}
		}

		// Read the message
		msgLen := msgType.Len(lenHdr)
		msg := make([]byte, msgLen)
		n, err := r.Read(msg)
		if err != nil {
			return &ServerError{fmt.Sprintf("read: %v", err)}
		}
		if n < msgLen {
			log.Printf("[%s]** expected to read %d bytes, but only recv'd: %d\nmsg: %x", clientID, msgLen, n, msg)
		}

		// Handle message
		switch msgType {
		case message.TypeIAmCamera:
			meCam.UnmarshalBinary(msg)
			log.Printf("[%s]TypeIAmCamera: %+v\nraw: %x", clientID, meCam, msg)
		case message.TypeIAmDispatcher:
			td := TicketDispatcher{conn: conn}
			s.registerDispatcher(ctx, msg, &td)
			log.Printf("[%s]TypeIAmDispatcher: %+v\n%x", clientID, td, msg)
		case message.TypePlate:
			log.Printf("[%s]TypePlate: %x", clientID, msg)
			s.handlePlate(ctx, msg, meCam)
		case message.TypeWantHeartbeat:
			log.Printf("[%s]TypeWantHeartbeat: %x", clientID, msg)
			if heartbeatTicker != nil {
				return &ClientError{"WantHeartbeat already sent"}
			}
			if err := s.startHeartbeat(msg, conn, heartbeatTicker); err != nil {
				return fmt.Errorf("startHeartbeat: %w", err)
			}
		}
	}
}

func (s *Server) registerDispatcher(ctx context.Context, msg []byte, td *TicketDispatcher) error {
	td.UnmarshalBinary(msg)
	s.mu.Lock()
	defer s.mu.Unlock()
	// TODO: Need to refactor the registrations so a dispatcher can be easily unregistered if/when it's connection is closed.
	for _, rid := range td.Roads {
		_, ok := s.dispatchers[rid]
		if !ok {
			s.dispatchers[rid] = make([]*TicketDispatcher, 0)
		}
		s.dispatchers[rid] = append(s.dispatchers[rid], td)
	}
	return nil
}

func (s *Server) handlePlate(ctx context.Context, msg []byte, cam Camera) {
	p := message.Plate{}
	p.UnmarshalBinary(msg)

	clientID := ctx.Value(CONNECTION_ID)
	log.Printf("[%s]Plate: %+v", clientID, p)

	s.mu.Lock()
	defer s.mu.Unlock()
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
		s.ticketQueue <- v
	}
	// Add observation
	s.plates[cam.Road][p.Plate] = append(s.plates[cam.Road][p.Plate], &latest)
}

func (s *Server) ticketListen() {
	for {
		// Wait for dispatchers to come online
		time.Sleep(time.Millisecond * 500)

		ticket := <-s.ticketQueue
		log.Printf("picked up ticket from queue: %+v\n", ticket)
		ticket.Retry()
		// Look up dispatcher for road
		ds, ok := s.dispatchers[ticket.Road]
		if !ok && ticket.Retries() < 5 {
			log.Printf("no dispatchers available for road %d. Requeuing ticket..\n", ticket.Road)
			// Put the ticket back in the queue
			s.ticketQueue <- ticket
			continue
		}
		if len(ds) == 0 {
			continue
		}
		// pick random dispatcher
		td := ds[rand.Intn(len(ds))]
		// Send ticket
		if err := td.send(ticket); err != nil {
			// TODO: Try another dispatcher
			log.Printf("ticket dispatcher could not send ticket: %v\n", err)
			continue
		}
	}
}

func (s *Server) startHeartbeat(msg []byte, conn net.Conn, ticker *time.Ticker) error {
	// in deciseconds
	interval := binary.BigEndian.Uint32(msg[1:])
	if interval < 1 {
		return nil
	}
	ticker = time.NewTicker(time.Millisecond * time.Duration(interval) * 100)

	go func() {
		for {
			<-ticker.C
			hb := []byte{byte(message.TypeHeartbeat)}
			if _, err := conn.Write(hb); err != nil {
				log.Printf("write heartbeat err: %v\n", err)
				ticker.Stop()
			}
			log.Println("tick")
		}
	}()
	return nil
}

func (e *ServerError) Error() string {
	return e.Msg
}

func (e *ClientError) Error() string {
	return e.Msg
}

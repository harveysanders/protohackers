package spdaemon

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/harveysanders/protohackers/6-speed-daemon/message"
)

type (
	Server struct {
		listener    net.Listener
		mu          sync.Mutex
		dispatchers map[uint16]map[*TicketDispatcher]bool // [road ID]:dispatcher
		plates      map[uint16]map[string][]*observation  // [road ID][plate]
		ticketQueue ticketQueue
		ih          issueHistory
		metrics     metrics
	}

	metrics struct {
		Plates struct {
			Total  int
			Unique int
		}
		Tickets struct {
			Issued   int
			Queued   int
			Failed   int
			Attempts int
			Requeued int
			Dropped  int
		}
	}

	issueHistory interface {
		add(t *message.Ticket)

		lookupForDate(plate string, timestamp1, timestamp2 message.UnixTime) *message.Ticket

		printHistory(plate string) string
	}

	// Observation represents an event when a car's plate was captured on a certain road at a specific time and location.
	observation struct {
		plate     string
		mile      uint16
		timestamp time.Time
	}

	ctxKey string

	ClientError struct {
		Err error
	}
)

const CONNECTION_ID ctxKey = "CONNECTION_ID"

func NewServer() *Server {
	return &Server{
		dispatchers: make(map[uint16]map[*TicketDispatcher]bool, 0),
		plates:      make(map[uint16]map[string][]*observation, 0),
		ticketQueue: make(ticketQueue, 8192),
		ih:          newHistory(),
	}
}

func (s *Server) Start(ctx context.Context, port string) error {
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	log.Printf("Speed Daemon listening @ %s", l.Addr().String())

	s.listener = l

	go s.ticketListen(ctx)

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
			if !errors.Is(err, io.EOF) {
				log.Printf("[%s] Conn ERR: %v", clientID, err)
			}
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
	var dispatcher TicketDispatcher
	var heartbeatTicker *time.Ticker
	defer func() {
		if heartbeatTicker != nil {
			heartbeatTicker.Stop()
		}
		s.unregisterDispatcher(ctx, &dispatcher)
	}()

	r := bufio.NewReader(conn)

	for {
		msgHdr, err := r.Peek(1)
		if err != nil {
			return fmt.Errorf("msg header peek: %w", err)
		}
		// Read the first byte to get the message type
		msgType, err := message.ParseType(msgHdr[0])
		if err != nil {
			invalidMsg, err := r.Peek(10)
			if err != nil {
				log.Printf("problem peek invalid message: %v", err)
			}
			log.Printf("invalid message type: %v\n%x", err, invalidMsg)
			return &ClientError{fmt.Errorf("invalid message type: %w", err)}
		}

		if msgType == message.TypeWantMetrics {
			if err := json.NewEncoder(conn).Encode(s.metrics); err != nil {
				return fmt.Errorf("error sending metrics response: %w", err)
			}
			if _, err := r.Discard(r.Buffered()); err != nil {
				return fmt.Errorf("discard: %w", err)
			}
			continue
		}

		// Calc the expected length of the message.
		// The next 2 bytes contain enough info to calc the length of the complete message.
		lenHdr, err := r.Peek(2)
		if err != nil {
			return fmt.Errorf("length header peek: %w", err)
		}

		// Read the message
		msgLen := msgType.Len(lenHdr)
		msg := make([]byte, msgLen)
		n, err := io.ReadFull(r, msg)
		if err != nil {
			if err == io.ErrUnexpectedEOF {
				log.Printf("[%s]** expected to read %d bytes, but only recv'd: %d\nmsg: %x", clientID, msgLen, n, msg)
			}
			return fmt.Errorf("read: %w", err)
		}

		// Handle message
		switch msgType {
		case message.TypeIAmCamera:
			meCam.UnmarshalBinary(msg)
			// log.Printf("[%s]TypeIAmCamera: %+v\nraw: %x", clientID, meCam, msg)
		case message.TypeIAmDispatcher:
			dispatcher.conn = conn
			s.registerDispatcher(ctx, msg, &dispatcher)
			// log.Printf("[%s]TypeIAmDispatcher: %+v\n%x", clientID, dispatcher, msg)
		case message.TypePlate:
			// log.Printf("[%s]TypePlate: %x", clientID, msg)
			s.handlePlate(ctx, msg, meCam)
		case message.TypeWantHeartbeat:
			// log.Printf("[%s]TypeWantHeartbeat: %x", clientID, msg)
			if heartbeatTicker != nil {
				return &ClientError{errors.New("wantHeartbeat already sent")}
			}
			if err := s.startHeartbeat(ctx, msg, conn, heartbeatTicker); err != nil {
				return fmt.Errorf("startHeartbeat: %w", err)
			}
		}
	}
}

func (s *Server) registerDispatcher(ctx context.Context, msg []byte, td *TicketDispatcher) error {
	td.UnmarshalBinary(msg)
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, rid := range td.Roads {
		_, ok := s.dispatchers[rid]
		if !ok {
			s.dispatchers[rid] = make(map[*TicketDispatcher]bool, 0)
		}
		s.dispatchers[rid][td] = true
	}
	return nil
}

func (s *Server) unregisterDispatcher(ctx context.Context, td *TicketDispatcher) {
	if td == nil {
		return
	}
	for _, rid := range td.Roads {
		delete(s.dispatchers[rid], td)
	}
}

func (s *Server) handlePlate(ctx context.Context, msg []byte, cam Camera) {
	p := message.Plate{}
	p.UnmarshalBinary(msg)

	clientID := ctx.Value(CONNECTION_ID)
	log.Printf("[%s] Plate: %+v", clientID, p)

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.plates[cam.Road]; !ok {
		s.plates[cam.Road] = make(map[string][]*observation)
	}

	s.metrics.Plates.Total++

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
		s.metrics.Plates.Unique++
		return
	}
	// If seen before
	// iterate over the records and calculate the average speed
	if v := checkViolation(latest, obs, float64(cam.Limit)); v != nil {
		v.Road = cam.Road
		log.Print("____________________")
		log.Printf("violation: %+v", v)
		log.Print("____________________")
		s.metrics.Tickets.Queued++
		s.ticketQueue <- v
	}
	// Add observation
	s.plates[cam.Road][p.Plate] = append(s.plates[cam.Road][p.Plate], &latest)
}

func (s *Server) ticketListen(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			log.Printf("context closed: %v\n", ctx.Err())
			return
		case ticket := <-s.ticketQueue:
			time.Sleep(time.Millisecond)

			ticket.IncAttempts()

			s.metrics.Tickets.Attempts++

			// Look up dispatcher for road
			td, err := s.nextDispatcher(ticket.Road)
			if err != nil {
				// log.Printf("%v.\n", err)
				if ticket.Retries() < 50 {
					// log.Print("%Requeuing ticket..\n")
					// Put the ticket back in the queue
					s.metrics.Tickets.Requeued++
					s.ticketQueue <- ticket
				} else {
					s.metrics.Tickets.Dropped++
					log.Printf("Retried to find dispatcher %d times. Dropping ticket...\n", ticket.Retries())
				}
				continue
			}

			// Double check ticket not already issued for same day
			if issued := s.ih.lookupForDate(ticket.Plate, ticket.Timestamp1, ticket.Timestamp2); issued != nil {
				log.Printf("Ticket already issued: %+v", ticket)
				s.metrics.Tickets.Dropped++
				// Don't requeue and move on to next
				continue
			}

			// Send ticket
			if err := td.send(ticket); err != nil {
				s.metrics.Tickets.Failed++
				log.Printf("Ticket dispatcher could not send ticket: %v\n", err)
				// Try again later
				s.ticketQueue <- ticket
				continue
			}
			s.metrics.Tickets.Issued++
			s.ih.add(ticket)
			log.Printf("Ticket issued: %+v\n", ticket)
			log.Printf("%d left in queue.\n", len(s.ticketQueue))
		}
	}
}

func (s *Server) nextDispatcher(roadID uint16) (*TicketDispatcher, error) {
	dispatchers, ok := s.dispatchers[roadID]
	if !ok {
		return nil, fmt.Errorf("no dispatchers available for road %d", roadID)
	}
	for dispatcher := range dispatchers {
		return dispatcher, nil
	}
	return nil, fmt.Errorf("no dispatchers available for road %d", roadID)
}

func (s *Server) startHeartbeat(ctx context.Context, msg []byte, conn net.Conn, ticker *time.Ticker) error {
	// in deciseconds
	interval := binary.BigEndian.Uint32(msg[1:])
	if interval < 1 {
		return nil
	}
	ticker = time.NewTicker(time.Millisecond * time.Duration(interval) * 100)

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
			case <-ticker.C:
				hb := []byte{byte(message.TypeHeartbeat)}
				if _, err := conn.Write(hb); err != nil {
					ticker.Stop()
				}
			}
		}
	}()
	return nil
}

func (e *ClientError) Error() string {
	return e.Err.Error()
}

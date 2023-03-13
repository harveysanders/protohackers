package meanstoend

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
)

type (
	// "I" for insert, or "Q" for query
	messageType string

	InsertMessage struct {
		Type      messageType
		Timestamp int32
		Price     int32
	}

	QueryMessage struct {
		Type    messageType
		MinTime int32
		MaxTime int32
	}

	store struct {
		prices prices
	}

	price struct {
		Timestamp int32
		Price     int32
	}

	prices []price

	Server struct {
		listener net.Listener
	}

	ContextKey int
)

const (
	CONNECTION_ID ContextKey = iota
)

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
		go func(conn net.Conn) {
			ctx := context.WithValue(context.Background(), CONNECTION_ID, clientID)

			if err := HandleConnection(ctx, conn); err != nil {
				log.Printf("client cause error:\n%v\nclosing connection..", err)
				if err := conn.Close(); err != nil {
					log.Printf("close: %x\n", err)
				}
			}

		}(conn)
	}
}

func (s *Server) Stop() error {
	return s.listener.Close()
}

func (i *InsertMessage) Parse(raw []byte) error {
	i.Type = messageType(raw[0])
	i.Timestamp = int32(binary.BigEndian.Uint32(raw[1:5]))
	i.Price = int32(binary.BigEndian.Uint32(raw[5:9]))
	return nil
}

func (i *QueryMessage) Parse(raw []byte) error {
	i.Type = messageType(raw[0])
	i.MinTime = int32(binary.BigEndian.Uint32(raw[1:5]))
	i.MaxTime = int32(binary.BigEndian.Uint32(raw[5:9]))
	return nil
}

func HandleConnection(ctx context.Context, conn net.Conn) error {
	msgLen := 9
	rawMsg := make([]byte, msgLen)
	store := newStore()
	mean := int32(0)

	var rdr io.Reader
	rdr = conn
	if len(os.Getenv("DUMP")) > 0 {
		dumpFile, err := dumpWriter(ctx)
		if err != nil {
			log.Fatal(err)
		}
		rdr = io.TeeReader(conn, dumpFile)
	}

	for {
		n, err := io.ReadAtLeast(rdr, rawMsg, msgLen)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			if err == io.ErrUnexpectedEOF {
				log.Printf("expected 9 bytes, got %d\nmessage: %x", n, rawMsg[:n])
				// return fmt.Errorf("expected 9 bytes, got %d\nmessage: %x", n, rawMsg[:n])
				continue
			}
			return fmt.Errorf("read: %w", err)
		}

		typ := messageType(rawMsg[0])
		switch typ {
		case "I":
			msg := InsertMessage{}
			if err := msg.Parse(rawMsg); err != nil {
				return err
			}
			store.Insert(ctx, price{msg.Timestamp, msg.Price})
		case "Q":
			msg := QueryMessage{}
			if err := msg.Parse(rawMsg); err != nil {
				return err
			}
			mean = store.calcMean(ctx, msg.MinTime, msg.MaxTime)
			fmt.Printf("mean: %d\n", mean)

			_, err := conn.Write(binary.BigEndian.AppendUint32([]byte{}, uint32(mean)))
			if err != nil {
				return err
			}
			return conn.Close()
		default:
			return fmt.Errorf(`expected type "I" or "Q", got %q`, typ)
		}
	}

}

func newStore() *store {
	return &store{
		prices: make(prices, 0),
	}
}

// Insert inserts the p in chronological ascending order.
func (s *store) Insert(ctx context.Context, p price) {
	// log.Printf("[%v] p: %+v\n", ctx.Value(CONNECTION_ID), p)
	for i, prev := range s.prices {
		if p.Timestamp == prev.Timestamp {
			return
		}
		if p.Timestamp < prev.Timestamp {
			s.prices = s.prices.insert(i, p)
			return
		}
	}
	// append to end if all prev are greater than current
	s.prices = append(s.prices, p)
}

func (p prices) insert(index int, value price) prices {
	if len(p) == index { // nil or empty slice or after last element
		return append(p, value)
	}
	p = append(p[:index+1], p[index:]...) // index < len(a)
	p[index] = value
	return p
}

func (s *store) calcMean(ctx context.Context, minTime, maxTime int32) int32 {
	// log.Printf("[%s] Q: min:%d, max: %d\nlist: %v", ctx.Value(CONNECTION_ID), minTime, maxTime, s.prices)
	if minTime > maxTime {
		return 0
	}
	mean := float64(0)
	n := 0
	for _, v := range s.prices {
		if v.Timestamp >= maxTime {
			break
		}
		if v.Timestamp >= minTime {
			n++
			mean = mean + (float64(v.Price)-mean)/float64(n)
		}

	}
	return int32(mean)
}

func dumpWriter(ctx context.Context) (io.Writer, error) {
	filename := fmt.Sprintf("%d.txt", ctx.Value(CONNECTION_ID))
	dumpPath, err := filepath.Abs(path.Join("../dumps", filename))
	if err != nil {
		return nil, err
	}

	return os.Create(dumpPath)
}

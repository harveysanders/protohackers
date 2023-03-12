package meanstoend

import (
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
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
		go func(c net.Conn) {
			if err := HandleConnection(c, clientID); err != nil {
				log.Printf("client cause error:\n%v\nclosing connection..", err)
				if err := c.Close(); err != nil {
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

func HandleConnection(c net.Conn, clientID int) error {
	msgLen := 9
	rawMsg := make([]byte, msgLen)
	store := newStore()
	mean := int32(0)
	for {
		n, err := io.ReadAtLeast(c, rawMsg, msgLen)
		if err != nil {
			if err == io.EOF {
				return io.EOF
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
			store.Insert(price{msg.Timestamp, msg.Price})
		case "Q":
			msg := QueryMessage{}
			if err := msg.Parse(rawMsg); err != nil {
				return err
			}
			mean = store.calcMean(msg.MinTime, msg.MaxTime)
			fmt.Printf("mean: %d", mean)

			_, err := c.Write(binary.BigEndian.AppendUint32([]byte{}, uint32(mean)))
			if err != nil {
				return err
			}
			return c.Close()
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
func (s *store) Insert(p price) {
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

func (s *store) calcMean(minTime, maxTime int32) int32 {
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

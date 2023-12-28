package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/textproto"
	"os"
)

type (
	Server struct {
		listener net.Listener
	}

	request struct {
		Method string   `json:"method"`
		Number *float64 `json:"number,omitempty"`
	}

	response struct {
		Method string `json:"method"`
		Prime  bool   `json:"prime"`
	}
)

func NewServer() *Server {
	return &Server{}
}

func (s *Server) Run(port string) (err error) {
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	s.listener = l
	fmt.Printf("Listening on port: %s\n", port)

	return s.handleConnections()
}

func (s *Server) Close() error {
	return s.listener.Close()
}

func main() {
	port := "9000"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}

	srv := NewServer()

	err := srv.Run(port)
	if err != nil {
		log.Fatal(err)
	}
}

func (s *Server) handleConnections() (err error) {
	clientID := 0
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return fmt.Errorf("accept: %s", err.Error())
		}
		clientID++
		go handleConnection(conn, clientID)
	}
}

func handleConnection(c net.Conn, clientID int) {
	conn := textproto.NewReader(bufio.NewReader(c))
	err := (func(clientID int) error {
		reqID := 0
		for {
			reqID++
			line, err := conn.ReadLineBytes()
			if err != nil {
				if err == io.EOF {
					return err
				}
				return fmt.Errorf("readLineBytes: %v", err)
			}

			fmt.Printf("[%d, %d] request json: %s\n", clientID, reqID, string(line))

			var req request
			err = json.Unmarshal(line, &req)
			if err != nil {
				return fmt.Errorf("unmarshal: %v", err)
			}

			if !validateReq(req) {
				return fmt.Errorf("invalid request: %+v", req)
			}

			resp := response{
				Method: "isPrime",
				Prime:  isPrime(*req.Number),
			}
			respData, err := json.Marshal(&resp)
			if err != nil {
				return fmt.Errorf("marshal: %v", err)
			}

			respData = append(respData, []byte("\n")...)
			fmt.Printf("[%d, %d] response: %s\n", clientID, reqID, respData)

			_, err = c.Write(respData)
			if err != nil {
				fmt.Printf("[%d, %d] write: %v\n", clientID, reqID, err)
				return nil
			}
		}
	})(clientID)

	if err != nil {
		if err == io.EOF {
			if err = c.Close(); err != nil {
				fmt.Printf("[%d] close: %v", clientID, err)
			}
			return
		}
		fmt.Printf("[%d] request error: %v \nwriting error response...\n", clientID, err)
		_, err := c.Write([]byte("{}\n"))
		if err != nil {
			fmt.Printf("[%d] write: %v\n", clientID, err)
			return
		}

	}
}

func validateReq(r request) bool {
	if r.Method != "isPrime" {
		return false
	}

	// Differentiate from zero value for missing field.
	// See custom unmarshal.
	if r.Number == nil {
		return false
	}
	// TODO: Add more rules?
	return true
}

func isPrime(n float64) bool {
	if n <= 1 {
		return false
	}
	if n == 2 || n == 3 {
		return true
	}
	if !isInteger(n) {
		return false
	}
	if math.Mod(n, 2) == 0 || math.Mod(n, 3) == 0 {
		return false
	}

	for i := 5; i*i <= int(n); i += 6 {
		if math.Mod(n, float64(i)) == 0 || math.Mod(n, (float64(i)+2)) == 0 {
			return false
		}
	}
	return true
}

func isInteger(n float64) bool {
	return n == float64(int(n))
}

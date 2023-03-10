package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/textproto"
	"os"
)

type (
	request struct {
		Method string  `json:"method"`
		Number float64 `json:"number"`
	}

	response struct {
		Method string `json:"method"`
		Prime  bool   `json:"prime"`
	}
)

func main() {
	port := "9000"
	if PORT := os.Getenv("PORT"); PORT != "" {
		port = PORT
	}

	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("listen: %s", err.Error())
	}
	defer l.Close()

	fmt.Printf("Listening on port: %s\n", port)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("accept: %s", err.Error())
		}

		go handleConn(conn)
	}
}

func handleConn(c net.Conn) {
	lr := io.LimitReader(c, 4096)
	conn := textproto.NewReader(bufio.NewReader(lr))
	err := (func() error {
		for {
			line, err := conn.ReadLineBytes()
			if err != nil {
				if errors.Is(err, io.EOF) {
					if err := c.Close(); err != nil {
						fmt.Printf("close: %v\n", err)
						return nil
					}
				}
				return fmt.Errorf("readLineBytes: %v", err)
			}

			fmt.Printf("request json: %s\n", string(line))

			var req request
			err = json.Unmarshal(line, &req)
			if err != nil {
				return fmt.Errorf("unmarshal: %v", err)
			}

			fmt.Printf("request parsed: %+v\n", req)

			if !validateReq(req) {
				return fmt.Errorf("invalid request: %+v", req)
			}

			resp := response{
				Method: "isPrime",
				Prime:  isPrime(req.Number),
			}
			respData, err := json.Marshal(&resp)
			if err != nil {
				return fmt.Errorf("marshal: %v", err)
			}

			respData = append(respData, []byte("\n")...)
			fmt.Printf("response: %s\n", respData)

			_, err = c.Write(respData)
			if err != nil {
				fmt.Printf("write: %v\n", err)
				return nil
			}
		}
	})()

	if err != nil {
		_, err := c.Write([]byte("{}\n"))
		if err != nil {
			fmt.Printf("write: %v\n", err)
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
	if r.Number == math.Inf(-1) {
		return false
	}
	// TODO: Add more rules?
	return true
}

func isPrime(n float64) bool {
	if n == 2 || n == 3 {
		return true
	}

	if n <= 1 || math.Mod(n, 2) == 0 || math.Mod(n, 3) == 0 {
		return false
	}

	for i := 5; i*i <= int(n); i += 6 {
		if math.Mod(n, float64(i)) == 0 || math.Mod(n, (float64(i)+2)) == 0 {
			return false
		}
	}
	return true
}

func (r *request) UnmarshalJSON(data []byte) error {
	type alias request
	aux := alias(*r)
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	// Differentiate literal 0 from a missing "number" field
	if !bytes.Contains(data, []byte("number")) {
		aux.Number = math.Inf(-1)
	}
	*r = request(aux)
	return nil
}

package main

import (
	"bufio"
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
					return nil
				}
				return fmt.Errorf("readLineBytes: %v", err)
			}
			// if len(line) == 0 {
			// 	return nil
			// }

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
				Prime:  isPrime(req.Number),
			}
			respData, err := json.Marshal(&resp)
			if err != nil {
				return fmt.Errorf("marshal: %v", err)
			}

			respData = append(respData, []byte("\n")...)
			_, err = c.Write(respData)
			if err != nil {
				fmt.Printf("write: %v", err)
				return nil
			}
		}
	})()

	if err != nil {
		_, err := c.Write([]byte("{}\n"))
		if err != nil {
			fmt.Printf("write: %v", err)
			return
		}
		if err := c.Close(); err != nil {
			fmt.Printf("close: %v", err)
			return
		}
	}
}

func validateReq(r request) bool {
	if r.Method != "isPrime" {
		return false
	}

	// Assume Number will not intentionally be set to 0.
	// TODO: If assumption is incorrect, write a custom unmarshaler.
	if r.Number == 0 {
		return false
	}
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

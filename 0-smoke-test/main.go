package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"os"
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

	host, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Listening on host: %s, port: %s\n", host, port)

	clientID := 0

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("accept: %s", err.Error())
		}

		clientID++
		go handleConnection(conn, clientID)
	}
}

func handleConnection(conn net.Conn, clientID int) {
	var buf bytes.Buffer
	bytesRead, err := io.Copy(&buf, conn)
	if err != nil {
		log.Printf("[%d]: copy: %s", clientID, err.Error())
		return
	}

	log.Printf("[%d]: read in %d bytes", clientID, bytesRead)

	bytesWritten, err := conn.Write(buf.Bytes())
	if err != nil {
		log.Printf("[%d]: write: %s", clientID, err.Error())
		return
	}

	log.Printf("[%d]: wrote out %d bytes", clientID, bytesWritten)

	err = conn.Close()
	if err != nil {
		log.Printf("[%d]: close: %s", clientID, err.Error())
		return
	}
}

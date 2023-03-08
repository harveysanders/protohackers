package main

import (
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
		clientID++
		conn, err := l.Accept()
		if err != nil {
			log.Fatalf("accept: %s", err.Error())
		}

		go handleConnection(conn, clientID)
	}
}

func handleConnection(conn net.Conn, clientID int) {
	msgLen := 2048
	buf := make([]byte, msgLen)
	bytesRead, err := conn.Read(buf)
	if err != nil {
		if err == io.EOF {
			conn.Close()
			return
		}
		log.Printf("id: %d: conn.read: %s", clientID, err.Error())
		return
	}

	log.Printf("id: %d: read in %d bytes", clientID, bytesRead)

	bytesWritten, err := conn.Write(buf)
	if err != nil {
		log.Printf("id: %d: conn.write: %s", clientID, err.Error())
		return
	}

	log.Printf("id: %d: wrote out %d bytes", clientID, bytesWritten)
}

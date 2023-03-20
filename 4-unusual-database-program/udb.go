package udb

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type (
	server struct {
		readTimeout   time.Duration
		writeTimeout  time.Duration
		maxBufferSize int
		store         store
		version       string
	}

	store interface {
		Insert(key []byte, value []byte)
		Retrieve(key []byte) (value []byte, ok bool)
		fmt.Stringer
	}

	storeSyncMap struct {
		store sync.Map
	}

	storeMap struct {
		mu    sync.Mutex
		store map[string][]byte
	}
)

func NewServer(store store) *server {
	version := "alpha"
	if UDB_VERSION := os.Getenv("UDB_VERSION"); UDB_VERSION != "" {
		version = UDB_VERSION
	}

	return &server{
		readTimeout:   time.Second * 10,
		writeTimeout:  time.Second * 10,
		maxBufferSize: 1024,
		store:         store,
		version:       version,
	}
}

func (s *server) ServeUDP(ctx context.Context, address string) error {
	pConn, err := net.ListenPacket("udp", address)
	if err != nil {
		return fmt.Errorf("listenPacket: %w", err)
	}
	defer pConn.Close()

	done := make(chan error, 1)
	buffer := make([]byte, s.maxBufferSize)

	go func() {
		for {
			n, fromAddr, err := pConn.ReadFrom(buffer)
			if err != nil {
				done <- fmt.Errorf("readFrom: %w", err)
				return
			}

			// Copy contents to new slice so we don't reference the data in the ever-changing buffer.
			msg := make([]byte, n)
			copy(msg, buffer[:n])
			msg = bytes.Trim(msg, "\n")

			if IsInsert(msg) {
				// log.Printf("** INSERT **\nfrom: %s\ncontents: %s\n*************\n", fromAddr.String(), msg)
				if err := s.handleInsert(msg); err != nil {
					done <- fmt.Errorf("handleInsert: %w", err)
					return
				}
				continue
			}
			// Assume it's a retrieve request
			// log.Printf("** QUERY **\nfrom: %s\ncontents: %s\n*************\n", fromAddr.String(), msg)
			resp := s.handleQuery(msg)

			err = pConn.SetWriteDeadline(time.Now().Add(s.writeTimeout))
			if err != nil {
				done <- err
				return
			}

			pConn.WriteTo(resp, fromAddr)
			if err != nil {
				done <- err
				return
			}
			// log.Printf("STORE: %s\n", s.store.String())
			// log.Printf("** RESPONSE SENT **\nto: %s\ncontents: %s\n*************\n", fromAddr.String(), resp)
		}
	}()

	select {
	case <-ctx.Done():
		log.Printf("cancelled with err: %v", ctx.Err())
	case err = <-done:
		log.Printf("err: %v", err)
	}

	return nil
}

func IsInsert(data []byte) bool {
	return bytes.ContainsRune(data, '=')
}

func (s *server) handleInsert(q []byte) error {
	pair := bytes.SplitN(q, []byte("="), 2)
	if len(pair) != 2 {
		return fmt.Errorf("could not parse query: %s\n%+v", q, pair)
	}
	key := pair[0]
	value := pair[1]
	s.store.Insert(key, value)
	return nil
}

func (s *server) handleQuery(key []byte) (resp []byte) {
	if bytes.EqualFold(key, []byte("version")) {
		return []byte("version=" + s.version)
	}
	v, ok := s.store.Retrieve(key)
	if !ok {
		// If a request attempts to retrieve a key for which no value exists, the server can return a response as if the key had the empty value (e.g. "key=")
		return append(key, '=')
	}
	resp = append(resp, key...)
	resp = append(resp, '=')
	resp = append(resp, v...)
	return resp
}

func NewStoreSyncMap() *storeSyncMap {
	return &storeSyncMap{
		store: sync.Map{},
	}
}

func (s *storeSyncMap) Insert(k []byte, v []byte) {
	s.store.Store(string(k), v)
}

func (s *storeSyncMap) Retrieve(k []byte) (value []byte, ok bool) {
	v, ok := s.store.Load(string(k))
	if !ok {
		return []byte{}, false
	}
	bv, ok := v.([]byte)
	if !ok {
		// log.Printf("could not convert value, %+v, to byte slice", v)
		return []byte{}, false
	}
	return bv, true
}

func (s *storeSyncMap) String() string {
	var res strings.Builder
	s.store.Range(func(key, value any) bool {
		res.WriteString(key.(string) + "=")
		if _, err := res.Write(value.([]byte)); err != nil {
			log.Printf("string() write: %v", err)
			return false
		}
		res.WriteRune('\n')
		return true
	})
	return res.String()
}

func NewStoreMap() *storeMap {
	return &storeMap{
		mu:    sync.Mutex{},
		store: map[string][]byte{},
	}
}

func (s *storeMap) Insert(k []byte, v []byte) {
	s.mu.Lock()
	s.store[string(k)] = v
	s.mu.Unlock()
}

func (s *storeMap) Retrieve(k []byte) (value []byte, ok bool) {
	s.mu.Lock()
	v, ok := s.store[string(k)]
	s.mu.Unlock()
	return v, ok
}

func (s *storeMap) String() string {
	var str strings.Builder
	for k, v := range s.store {
		str.WriteString(k + "=")
		str.Write(v)
		str.WriteRune('\n')
	}
	return str.String()
}

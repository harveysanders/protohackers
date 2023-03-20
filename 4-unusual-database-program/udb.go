package udb

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

type (
	server struct {
		readTimeout   time.Duration
		writeTimeout  time.Duration
		maxBufferSize int
		store         store
	}

	store interface {
		Insert(key []byte, value []byte)
		Retrieve(key []byte) (value []byte, ok bool)
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
	return &server{
		readTimeout:   time.Second * 10,
		writeTimeout:  time.Second * 10,
		maxBufferSize: 1024,
		store:         store,
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

			msg := bytes.Trim(buffer[:n], "\n")

			if IsInsert(msg) {
				log.Printf("** INSERT **\nfrom: %s\ncontents: %s\n*************\n", fromAddr.String(), msg)
				if err := s.handleInsert(msg); err != nil {
					done <- fmt.Errorf("handleInsert: %w", err)
					return
				}
				continue
			}
			// Assume it's a retrieve request
			log.Printf("** QUERY **\nfrom: %s\ncontents: %s\n*************\n", fromAddr.String(), msg)
			resp := s.handleRetrieve(msg)

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

			log.Printf("** RESPONSE SENT **\nto: %s\ncontents: %s\n*************\n", fromAddr.String(), resp)
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

func (s *server) handleRetrieve(key []byte) (resp []byte) {
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
		log.Printf("could not convert value, %+v, to byte slice", v)
		return []byte{}, false
	}
	return bv, true
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

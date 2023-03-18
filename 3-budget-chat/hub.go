package budgetchat

import (
	"fmt"
	"sync"
)

// Inspired by Gorilla WS Chat Example
// https://github.com/gorilla/websocket/tree/master/examples/chat

type (
	hub struct {
		mu      sync.Mutex
		clients map[string]*client

		// Messages to be broadcast to all chat clients.
		broadcast chan []byte

		// Requests to join the chat room.
		join chan *client

		// Requests to leave the chat room.
		leave chan *client
	}
)

func newHub() *hub {
	return &hub{
		clients:   map[string]*client{},
		join:      make(chan *client),
		leave:     make(chan *client),
		broadcast: make(chan []byte, 1024),
	}
}

func (h *hub) run() {
	for {
		select {
		// Incoming join request
		case client := <-h.join:
			h.addClient(client)
			msg := fmt.Sprintf("%s joined the chat!", client.name)
			h.broadcast <- []byte(msg)

		case client := <-h.leave:
			h.removeClient(client)

		case message := <-h.broadcast:
			for _, client := range h.clients {
				select {
				case client.send <- message:

				default:
					// Unable to send on channel for some reason.
					// Buffered channel could be full. If full, assume the client is not reading fast enough (bad or closed connection..)
					close(client.send)
					h.removeClient(client)
				}
			}
		}
	}
}

func (h *hub) addClient(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c.name] = c
}

func (h *hub) removeClient(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c.name)
}

func (h *hub) isNameTaken(name string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	_, ok := h.clients[name]
	return ok
}

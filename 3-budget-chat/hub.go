package budgetchat

import (
	"fmt"
	"strings"
	"sync"
)

// Inspired by Gorilla WS Chat Example
// https://github.com/gorilla/websocket/tree/master/examples/chat

type (
	message struct {
		from    string
		payload []byte
	}

	hub struct {
		mu      sync.Mutex
		clients map[string]*client

		// Messages to be broadcast to all chat clients.
		broadcast chan message

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
		broadcast: make(chan message, 1024),
	}
}

func (h *hub) run() {
	for {
		select {
		// Incoming join request
		case client := <-h.join:
			h.addClient(client)
			msg := fmt.Sprintf("* %s joined the chat!\n", client.name)
			h.broadcast <- message{
				from:    client.name,
				payload: []byte(msg),
			}
			client.send <- []byte(h.joinRespMsg(client.name))

		case client := <-h.leave:
			msg := fmt.Sprintf("* %s has left the building!\n", client.name)
			h.broadcast <- message{
				from:    client.name,
				payload: []byte(msg),
			}
			h.removeClient(client)

		case message := <-h.broadcast:
			for _, client := range h.clients {
				// Don't send message back to sender
				if message.from == client.name {
					continue
				}
				select {
				case client.send <- message.payload:

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

func (h *hub) joinRespMsg(username string) string {
	usernames := []string{}
	for name := range h.clients {
		if name != username {
			usernames = append(usernames, name)
		}
	}
	if len(usernames) == 0 {
		return "* you're the first one here!\n"
	}
	return fmt.Sprintf("* connected users: %s\n", strings.Join(usernames, ", "))
}

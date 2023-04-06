package spdaemon

import (
	"sync"

	"github.com/harveysanders/protohackers/spdaemon/message"
)

type (
	history struct {
		mu sync.Mutex
		// {[plate]: {
		//		[( floor(ticket.Timestamp) / 264 )]: Ticket }
		// }
		issued map[string]map[float64]*message.Ticket
	}
)

func newHistory() *history {
	return &history{
		issued: make(map[string]map[float64]*message.Ticket),
	}
}

func (h *history) add(t *message.Ticket) {
	day1 := t.Timestamp1.Day()
	day2 := t.Timestamp2.Day()

	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.issued[t.Plate]; !ok {
		h.issued[t.Plate] = make(map[float64]*message.Ticket)
	}

	h.issued[t.Plate][day1] = t
	if day1 != day2 {
		h.issued[t.Plate][day2] = t
	}
}

func (h *history) lookupForDate(plate string, timestamp1, timestamp2 message.UnixTime) *message.Ticket {
	h.mu.Lock()
	defer h.mu.Unlock()
	day1 := timestamp1.Day()
	day2 := timestamp2.Day()
	for i := day1; i <= day2; i++ {
		issuedDays, ok := h.issued[plate]
		if !ok {
			return nil
		}
		ticket, ok := issuedDays[i]
		if ok {
			return ticket
		}
	}
	return nil
}

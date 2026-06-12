package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan []byte]struct{}
}

func NewHub() *Hub {
	return &Hub{subscribers: make(map[string]map[chan []byte]struct{})}
}

func (h *Hub) Broadcast(orderID string, payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.subscribers[orderID] {
		select {
		case ch <- data:
		default:
		}
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request, orderID string, initial any) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.subscribe(orderID)
	defer h.unsubscribe(orderID, ch)

	if initial != nil {
		data, _ := json.Marshal(initial)
		writeEvent(w, "snapshot", data)
		flusher.Flush()
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case data := <-ch:
			writeEvent(w, "order.updated", data)
			flusher.Flush()
		}
	}
}

func (h *Hub) subscribe(orderID string) chan []byte {
	h.mu.Lock()
	defer h.mu.Unlock()

	ch := make(chan []byte, 16)
	if h.subscribers[orderID] == nil {
		h.subscribers[orderID] = make(map[chan []byte]struct{})
	}
	h.subscribers[orderID][ch] = struct{}{}
	return ch
}

func (h *Hub) unsubscribe(orderID string, ch chan []byte) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.subscribers[orderID], ch)
	close(ch)
	if len(h.subscribers[orderID]) == 0 {
		delete(h.subscribers, orderID)
	}
}

func writeEvent(w http.ResponseWriter, event string, data []byte) {
	_, _ = fmt.Fprintf(w, "event: %s\n", event)
	_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
}

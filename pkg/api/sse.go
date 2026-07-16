package api

import (
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/ArianAr/Gantry/pkg/s3"
	"github.com/gin-gonic/gin"
)

// Hub broadcasts engine events to SSE subscribers.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan []byte]struct{}
}

// NewHub creates an SSE hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[chan []byte]struct{})}
}

// Emit implements s3.EventEmitter.
func (h *Hub) Emit(ev s3.Event) {
	if ev.Timestamp.IsZero() {
		ev.Timestamp = time.Now().UTC()
	}
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- data:
		default:
			// Drop if client is slow
		}
	}
}

func (h *Hub) subscribe() chan []byte {
	ch := make(chan []byte, 32)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

func (h *Hub) unsubscribe(ch chan []byte) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

// Stream handles GET /api/jobs/stream.
func (h *Hub) Stream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	ch := h.subscribe()
	defer h.unsubscribe(ch)

	// Initial hello
	hello, _ := json.Marshal(s3.Event{
		Type:      s3.EventLog,
		Message:   "sse connected",
		Timestamp: time.Now().UTC(),
	})
	writeSSE(c.Writer, hello)

	clientGone := c.Request.Context().Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-clientGone:
			return
		case <-ticker.C:
			// keepalive comment
			_, _ = io.WriteString(c.Writer, ": keepalive\n\n")
			c.Writer.Flush()
		case msg, ok := <-ch:
			if !ok {
				return
			}
			writeSSE(c.Writer, msg)
		}
	}
}

func writeSSE(w gin.ResponseWriter, data []byte) {
	_, _ = io.WriteString(w, "data: ")
	_, _ = w.Write(data)
	_, _ = io.WriteString(w, "\n\n")
	w.Flush()
}

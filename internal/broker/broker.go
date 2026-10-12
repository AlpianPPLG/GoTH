package broker

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
)

// Broker mengelola koneksi SSE aktif dan memancarkan event ke semua client (Fan-out)
type Broker struct {
	mu         sync.RWMutex
	clients    map[chan []byte]bool
	register   chan chan []byte
	unregister chan chan []byte
	broadcast  chan []byte
}

func NewBroker() *Broker {
	return &Broker{
		clients:    make(map[chan []byte]bool),
		register:   make(chan chan []byte),
		unregister: make(chan chan []byte),
		broadcast:  make(chan []byte, 100),
	}
}

// Start menjalankan loop broker di background goroutine
func (b *Broker) Start() {
	for {
		select {
		case client := <-b.register:
			b.mu.Lock()
			b.clients[client] = true
			log.Printf("[SSE Broker] Client tersambung. Total aktif: %d\n", len(b.clients))
			b.mu.Unlock()

		case client := <-b.unregister:
			b.mu.Lock()
			if _, ok := b.clients[client]; ok {
				delete(b.clients, client)
				close(client)
				log.Printf("[SSE Broker] Client terputus. Total aktif: %d\n", len(b.clients))
			}
			b.mu.Unlock()

		case message := <-b.broadcast:
			b.mu.RLock()
			for client := range b.clients {
				select {
				case client <- message:
				default:
					// Mencegah slow client memperlambat seluruh goroutine
					log.Println("[SSE Broker] Drop message ke client lambat")
				}
			}
			b.mu.RUnlock()
		}
	}
}

// Broadcast mengirimkan payload raw byte ke seluruh client
func (b *Broker) Broadcast(message []byte) {
	b.broadcast <- message
}

// BroadcastEvent memformat payload menjadi format SSE spesifikasi W3C:
// event: <eventName>
// data: <line1>
// data: <line2>
// \n
func (b *Broker) BroadcastEvent(eventName string, htmlFragment string) {
	var sb strings.Builder
	sb.WriteString("event: " + eventName + "\n")
	lines := strings.Split(htmlFragment, "\n")
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r")
		if trimmed != "" {
			sb.WriteString("data: " + trimmed + "\n")
		}
	}
	sb.WriteString("\n")
	b.broadcast <- []byte(sb.String())
}

func (b *Broker) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}

// ServeHTTP menangani request endpoint SSE dari HTMX (/events/stream)
func (b *Broker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported by browser/connection", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Channel privat untuk client ini
	clientChan := make(chan []byte, 30)
	b.register <- clientChan

	defer func() {
		b.unregister <- clientChan
	}()

	// Kirim initial comment/heartbeat agar koneksi langsung establish
	fmt.Fprintf(w, ": ping\n\n")
	flusher.Flush()

	notify := r.Context().Done()

	for {
		select {
		case <-notify:
			return
		case msg, ok := <-clientChan:
			if !ok {
				return
			}
			w.Write(msg)
			flusher.Flush()
		}
	}
}

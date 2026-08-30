// Package hub, bağlı WebSocket istemcilerinin kaydını ve yayınını yönetir.
//
// Faz 0'da tek oda vardır; Faz 1'de oda başına hub'a ve NATS JetStream
// dağıtımına evrilecek.
package hub

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const writeTimeout = 5 * time.Second

// Client, tek bir WebSocket bağlantısını sarar. gorilla/websocket aynı anda
// tek yazıcıya izin verdiği için tüm yazımlar mu ile sıralanır; saat senkronu
// yanıtları da bu yoldan geçer ki t2 damgası yazımın hemen öncesini yansıtsın.
type Client struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

// NewClient, bağlantıyı saran bir istemci oluşturur.
func NewClient(conn *websocket.Conn) *Client {
	return &Client{conn: conn}
}

// Send, tek bir metin çerçevesi yazar; yazım süresi writeTimeout ile sınırlıdır.
func (c *Client) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// SendLazy, çerçeveyi yazma kilidi ALINDIKTAN sonra build ile üretir ve hemen
// yazar. Saat senkronu yanıtı bunu kullanır: t2 damgası build içinde atıldığı
// için kilidin (örn. eşzamanlı ping yazımının) beklettiği süre t2'ye yansır;
// aksi halde t2 gerçek gönderimden erken kalır ve ofseti saptırırdı.
func (c *Client) SendLazy(build func() ([]byte, error)) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	data, err := build()
	if err != nil {
		return err
	}
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.conn.WriteMessage(websocket.TextMessage, data)
}

// Ping, keepalive ping çerçevesi yazar.
func (c *Client) Ping() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.conn.WriteMessage(websocket.PingMessage, nil)
}

// Close, bağlantıyı kapatır.
func (c *Client) Close() error { return c.conn.Close() }

// Hub, bağlı istemcilerin kaydıdır.
type Hub struct {
	log     *slog.Logger
	mu      sync.RWMutex
	clients map[*Client]struct{}
}

// New, boş bir hub oluşturur.
func New(log *slog.Logger) *Hub {
	return &Hub{log: log, clients: make(map[*Client]struct{})}
}

// Register, istemciyi kayda ekler.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	n := len(h.clients)
	h.mu.Unlock()
	h.log.Info("istemci bağlandı", "toplam", n)
}

// Unregister, istemciyi kayıttan düşürür ve bağlantısını kapatır.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	_, ok := h.clients[c]
	delete(h.clients, c)
	n := len(h.clients)
	h.mu.Unlock()
	if ok {
		_ = c.Close()
		h.log.Info("istemci ayrıldı", "toplam", n)
	}
}

// Count, bağlı istemci sayısıdır.
func (h *Hub) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast, çerçeveyi tüm istemcilere eşzamanlı gönderir. Yavaş ya da kopuk
// bir istemci diğerlerinin kue almasını geciktiremez; yazımı başarısız olan
// istemci kayıttan düşürülür.
func (h *Hub) Broadcast(data []byte) {
	h.mu.RLock()
	targets := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	var wg sync.WaitGroup
	for _, c := range targets {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			if err := c.Send(data); err != nil {
				h.log.Warn("yayın yazımı başarısız, istemci düşürülüyor", "hata", err)
				h.Unregister(c)
			}
		}(c)
	}
	wg.Wait()
}

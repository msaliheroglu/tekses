// Package hub, bağlı WebSocket istemcilerinin kaydını ve yayınını yönetir.
//
// Faz 0'da tek oda vardır; Faz 1'de oda başına hub'a ve NATS JetStream
// dağıtımına evrilecek.
package hub

import (
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

const writeTimeout = 5 * time.Second

// Frame, aynı mesajın iki kodlamasını taşır: v1 istemciler JSON metin
// çerçevesi, v2 istemciler protobuf ikili çerçevesi alır. Yayın öncesi bir
// kez kodlanır; istemci başına seçim Send sırasında yapılır.
type Frame struct {
	JSON   []byte
	Binary []byte
}

// Client, tek bir WebSocket bağlantısını sarar. gorilla/websocket aynı anda
// tek yazıcıya izin verdiği için tüm yazımlar mu ile sıralanır; saat senkronu
// yanıtları da bu yoldan geçer ki t2 damgası yazımın hemen öncesini yansıtsın.
type Client struct {
	conn *websocket.Conn
	mu   sync.Mutex

	// binary: istemci ikili (protobuf) telde mi? Hello alınınca okuma
	// döngüsü yazar; yayınlar eşzamanlı okur.
	binary atomic.Bool

	// room yalnızca hub kilidi altında okunur/yazılır.
	room string
}

// SetBinary, istemcinin kodeğini işaretler (hello çerçevesinin biçiminden).
func (c *Client) SetBinary(binary bool) { c.binary.Store(binary) }

// IsBinary, istemcinin ikili telde olup olmadığını döndürür.
func (c *Client) IsBinary() bool { return c.binary.Load() }

func frameType(binary bool) int {
	if binary {
		return websocket.BinaryMessage
	}
	return websocket.TextMessage
}

// NewClient, bağlantıyı saran bir istemci oluşturur.
func NewClient(conn *websocket.Conn) *Client {
	return &Client{conn: conn}
}

// Send, istemcinin kodeğine uygun tek bir çerçeve yazar; yazım süresi
// writeTimeout ile sınırlıdır.
func (c *Client) Send(data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(writeTimeout))
	return c.conn.WriteMessage(frameType(c.IsBinary()), data)
}

// SendFrame, çift kodlamalı yayın çerçevesinden istemciye uygun olanı yazar.
func (c *Client) SendFrame(f Frame) error {
	if c.IsBinary() {
		if f.Binary == nil {
			return nil // bu mesajın ikili kodlaması yok; v2 istemciye düşmez
		}
		return c.Send(f.Binary)
	}
	if f.JSON == nil {
		return nil
	}
	return c.Send(f.JSON)
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
	return c.conn.WriteMessage(frameType(c.IsBinary()), data)
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

// DefaultRoom, hello'sunda katılım kodu olmayan istemcilerin odasıdır
// (Faz 0 denemesi ve loadgen bu odada yaşar).
const DefaultRoom = "faz0"

// Hub, bağlı istemcilerin oda bazlı kaydıdır. Faz 2'de oda dağıtımı NATS
// JetStream'e taşınınca hub tek düğümün yerel kaydı olarak kalacak.
type Hub struct {
	log     *slog.Logger
	mu      sync.RWMutex
	clients map[*Client]struct{}
	rooms   map[string]map[*Client]struct{}
}

// New, boş bir hub oluşturur.
func New(log *slog.Logger) *Hub {
	return &Hub{
		log:     log,
		clients: make(map[*Client]struct{}),
		rooms:   make(map[string]map[*Client]struct{}),
	}
}

// Register, istemciyi varsayılan odaya kaydeder.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.placeLocked(c, DefaultRoom)
	n := len(h.clients)
	h.mu.Unlock()
	h.log.Info("istemci bağlandı", "toplam", n)
}

// JoinRoom, istemciyi (varsa eski odasından çıkarıp) odaya taşır.
func (h *Hub) JoinRoom(c *Client, room string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.clients[c]; !ok {
		return
	}
	h.removeFromRoomLocked(c)
	h.placeLocked(c, room)
}

func (h *Hub) placeLocked(c *Client, room string) {
	c.room = room
	if h.rooms[room] == nil {
		h.rooms[room] = make(map[*Client]struct{})
	}
	h.rooms[room][c] = struct{}{}
}

func (h *Hub) removeFromRoomLocked(c *Client) {
	if members, ok := h.rooms[c.room]; ok {
		delete(members, c)
		if len(members) == 0 {
			delete(h.rooms, c.room)
		}
	}
}

// Unregister, istemciyi kayıttan düşürür ve bağlantısını kapatır.
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	_, ok := h.clients[c]
	delete(h.clients, c)
	h.removeFromRoomLocked(c)
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

// RoomCounts, oda başına istemci sayısını döndürür.
func (h *Hub) RoomCounts() map[string]int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]int, len(h.rooms))
	for room, members := range h.rooms {
		out[room] = len(members)
	}
	return out
}

// Broadcast, çift kodlamalı çerçeveyi tüm istemcilere gönderir.
func (h *Hub) Broadcast(f Frame) { h.broadcast("", f) }

// BroadcastRoom, çerçeveyi yalnızca verilen odadaki istemcilere gönderir.
func (h *Hub) BroadcastRoom(room string, f Frame) { h.broadcast(room, f) }

// broadcast eşzamanlı gönderir: yavaş ya da kopuk bir istemci diğerlerinin
// kue almasını geciktiremez; yazımı başarısız olan istemci düşürülür.
func (h *Hub) broadcast(room string, f Frame) {
	h.mu.RLock()
	var targets []*Client
	if room == "" {
		targets = make([]*Client, 0, len(h.clients))
		for c := range h.clients {
			targets = append(targets, c)
		}
	} else {
		members := h.rooms[room]
		targets = make([]*Client, 0, len(members))
		for c := range members {
			targets = append(targets, c)
		}
	}
	h.mu.RUnlock()

	var wg sync.WaitGroup
	for _, c := range targets {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()
			if err := c.SendFrame(f); err != nil {
				h.log.Warn("yayın yazımı başarısız, istemci düşürülüyor", "hata", err)
				h.Unregister(c)
			}
		}(c)
	}
	wg.Wait()
}

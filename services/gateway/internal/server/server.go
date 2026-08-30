// Package server, Faz 0 gateway'inin HTTP/WebSocket yüzeyini sağlar:
//
//	GET  /                     — moderatör konsol sayfası (Faz 0 mini konsol)
//	GET  /join                 — tarayıcı katılımcı deneme sayfası (telefon kurulumsuz)
//	GET  /healthz              — sağlık ve bağlı istemci sayısı
//	GET  /ws                   — katılımcı WebSocket'i (hello, saat senkronu, kue alımı)
//	POST /api/v0/cue           — kue tetikle
//	POST /api/v0/intervention  — HOLD / STOP / SKIP / BLACKOUT yayınla
package server

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/msaliheroglu/tekses/packages/proto/wire"
	"github.com/msaliheroglu/tekses/services/gateway/internal/clock"
	"github.com/msaliheroglu/tekses/services/gateway/internal/hub"
)

const (
	roomID = "faz0"

	// Okuma sınırı: telde küçük kontrol mesajlarından başka bir şey akmaz.
	maxMessageBytes = 4096

	// İstemci en az 1–2 dakikada bir saat senkronu yapar; bu sürede hiçbir
	// çerçeve (pong dahil) gelmezse bağlantı ölü sayılır.
	readTimeout  = 5 * time.Minute
	pingInterval = 50 * time.Second

	// Kue tekrarları: aynı run_id, paket kaybına karşı 3 kez.
	cueRepeats        = 3
	cueRepeatInterval = 250 * time.Millisecond

	// Işığa duyarlılık kuralı: sürekli yanıp sönme <= 3 Hz.
	maxFlashHz = 3

	minCueDelayMs = 500
	maxCueDelayMs = 10 * 60 * 1000
	maxDurationMs = 10 * 60 * 1000
)

var colorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

// Faz 0 deneme sayfaları: moderatör mini konsolu ve tarayıcı katılımcısı.
// Katılımcı ürünü Flutter'dır (apps/participant); /join yalnızca kurulumsuz
// deneme içindir — Faz 3'teki tarayıcı yedeği kararından bağımsızdır.
//
//go:embed static
var staticFS embed.FS

// Server, gateway'in HTTP yüzeyini taşır.
type Server struct {
	log        *slog.Logger
	clock      *clock.ServerClock
	hub        *hub.Hub
	adminToken string
	upgrader   websocket.Upgrader
}

// New, bir gateway sunucusu kurar. adminToken boş değilse /api/* uçları
// "Authorization: Bearer <token>" başlığı ister.
func New(log *slog.Logger, adminToken string) *Server {
	return &Server{
		log:        log,
		clock:      clock.New(),
		hub:        hub.New(log),
		adminToken: adminToken,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// Faz 0 yerel ağ denemesi: origin denetimi yok. Faz 1'de
			// katılım kodu doğrulaması ve origin listesi eklenecek.
			CheckOrigin: func(*http.Request) bool { return true },
		},
	}
}

// Handler, yol tablosunu döndürür.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.staticPage("static/moderator.html"))
	mux.HandleFunc("GET /join", s.staticPage("static/join.html"))
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	mux.HandleFunc("GET /ws", s.handleWS)
	mux.HandleFunc("POST /api/v0/cue", s.requireAdmin(s.handleCue))
	mux.HandleFunc("POST /api/v0/intervention", s.requireAdmin(s.handleIntervention))
	return mux
}

func (s *Server) staticPage(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		data, err := staticFS.ReadFile(path)
		if err != nil {
			http.Error(w, "sayfa bulunamadı", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(data)
	}
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":         "ok",
		"clients":        s.hub.Count(),
		"server_time_ms": s.clock.NowMs(),
	})
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Content-Type zorunluluğu ucuz bir CSRF önlemidir: tarayıcı,
		// preflight'sız çapraz-site isteklerde application/json gönderemez.
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "Content-Type: application/json gerekli"})
			return
		}
		if s.adminToken != "" && r.Header.Get("Authorization") != "Bearer "+s.adminToken {
			writeJSON(w, http.StatusUnauthorized, map[string]any{"error": "geçersiz veya eksik yönetici token'ı"})
			return
		}
		next(w, r)
	}
}

// --- WebSocket ---

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.log.Warn("websocket yükseltmesi başarısız", "hata", err)
		return
	}
	client := hub.NewClient(conn)
	s.hub.Register(client)
	defer s.hub.Unregister(client)

	conn.SetReadLimit(maxMessageBytes)
	resetDeadline := func() { _ = conn.SetReadDeadline(time.Now().Add(readTimeout)) }
	resetDeadline()
	conn.SetPongHandler(func(string) error { resetDeadline(); return nil })

	// Keepalive ping döngüsü; okuma döngüsü bitince kapanır.
	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTicker(pingInterval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if err := client.Ping(); err != nil {
					return
				}
			}
		}
	}()

	for {
		_, raw, err := conn.ReadMessage()
		// t1 olabildiğince erken, çözümlemeden önce damgalanır.
		recvMs := s.clock.NowMs()
		if err != nil {
			return
		}
		resetDeadline()

		env, err := wire.Decode(raw)
		if err != nil {
			s.log.Warn("bozuk çerçeve", "hata", err)
			continue
		}

		switch env.Type {
		case wire.TypeHello:
			var hello wire.Hello
			if err := json.Unmarshal(env.Data, &hello); err != nil {
				s.log.Warn("bozuk hello", "hata", err)
				continue
			}
			if hello.ProtocolVersion != wire.ProtocolVersion {
				s.log.Warn("uyumsuz protokol sürümü", "istemci", hello.ProtocolVersion)
				return
			}
			s.send(client, wire.TypeWelcome, wire.Welcome{
				ServerTimeMs:    s.clock.NowMs(),
				ProtocolVersion: wire.ProtocolVersion,
				RoomID:          roomID,
			})

		case wire.TypeClockSyncRequest:
			var req wire.ClockSyncRequest
			if err := json.Unmarshal(env.Data, &req); err != nil {
				s.log.Warn("bozuk saat senkronu isteği", "hata", err)
				continue
			}
			// t2, yazma kilidi alındıktan sonra (SendLazy içinde) damgalanır:
			// kilidin beklettiği süre t2'ye yansır, ofset saptırılmaz.
			err := client.SendLazy(func() ([]byte, error) {
				return wire.Encode(wire.TypeClockSyncResponse, wire.ClockSyncResponse{
					Seq:          req.Seq,
					ClientMonoMs: req.ClientMonoMs,
					ServerRecvMs: recvMs,
					ServerSendMs: s.clock.NowMs(),
				})
			})
			if err != nil {
				s.hub.Unregister(client)
				return
			}

		default:
			s.log.Warn("beklenmeyen mesaj türü", "tür", env.Type)
		}
	}
}

func (s *Server) send(c *hub.Client, msgType string, msg any) {
	data, err := wire.Encode(msgType, msg)
	if err != nil {
		s.log.Error("mesaj kodlanamadı", "tür", msgType, "hata", err)
		return
	}
	if err := c.Send(data); err != nil {
		s.hub.Unregister(c)
	}
}

// --- Kontrol API'si ---

type cueRequest struct {
	CueID      string `json:"cue_id"`
	DelayMs    int64  `json:"delayMs"`
	DurationMs uint32 `json:"durationMs"`
	Color      string `json:"color"`
	Torch      bool   `json:"torch"`
	FlashHz    uint32 `json:"flashHz"`
}

func (s *Server) handleCue(w http.ResponseWriter, r *http.Request) {
	var req cueRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxMessageBytes)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("gövde çözülemedi: %v", err)})
		return
	}

	if req.CueID == "" {
		req.CueID = "faz0-flash"
	}
	if req.DelayMs == 0 {
		req.DelayMs = 3000
	}
	if req.DelayMs < minCueDelayMs || req.DelayMs > maxCueDelayMs {
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error": fmt.Sprintf("delayMs %d–%d aralığında olmalı", minCueDelayMs, maxCueDelayMs)})
		return
	}
	if req.DurationMs == 0 {
		req.DurationMs = 3000
	}
	if req.DurationMs > maxDurationMs {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("durationMs en çok %d olabilir", maxDurationMs)})
		return
	}
	if req.Color == "" {
		req.Color = "#FF2A2A"
	}
	if !colorRe.MatchString(req.Color) {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "color #RRGGBB biçiminde olmalı"})
		return
	}
	if req.FlashHz > maxFlashHz {
		// Işığa duyarlılık sınırı: sessizce kırpmak yerine reddet ki
		// moderatör sınırı bilerek tasarlasın.
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("flashHz en çok %d olabilir (ışığa duyarlılık sınırı)", maxFlashHz)})
		return
	}

	runID, err := newRunID()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "run_id üretilemedi"})
		return
	}

	cue := wire.CueStart{
		RunID:          runID,
		CueID:          req.CueID,
		FireAtServerMs: s.clock.NowMs() + req.DelayMs,
		Payload: wire.CuePayload{
			Color:      req.Color,
			Torch:      req.Torch,
			FlashHz:    req.FlashHz,
			DurationMs: req.DurationMs,
		},
	}
	s.broadcastCueWithRepeats(cue)

	s.log.Info("kue yayınlandı",
		"run_id", cue.RunID, "cue_id", cue.CueID,
		"fire_at", cue.FireAtServerMs, "istemci", s.hub.Count())

	writeJSON(w, http.StatusOK, map[string]any{
		"run_id":            cue.RunID,
		"cue_id":            cue.CueID,
		"fire_at_server_ms": cue.FireAtServerMs,
		"server_time_ms":    s.clock.NowMs(),
		"clients":           s.hub.Count(),
	})
}

// broadcastCueWithRepeats, aynı kueyi cueRepeats kez yayınlar; istemciler
// run_id ile tekilleştirir. İlk tekrar hemen, sonrakiler aralıklarla gider.
func (s *Server) broadcastCueWithRepeats(cue wire.CueStart) {
	for i := uint32(1); i <= cueRepeats; i++ {
		repeat := cue
		repeat.RepeatSeq = i
		data, err := wire.Encode(wire.TypeCueStart, repeat)
		if err != nil {
			s.log.Error("kue kodlanamadı", "hata", err)
			return
		}
		delay := time.Duration(i-1) * cueRepeatInterval
		time.AfterFunc(delay, func() { s.hub.Broadcast(data) })
	}
}

type interventionRequest struct {
	RunID string `json:"run_id"`
	Kind  string `json:"kind"`
}

func (s *Server) handleIntervention(w http.ResponseWriter, r *http.Request) {
	var req interventionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxMessageBytes)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": fmt.Sprintf("gövde çözülemedi: %v", err)})
		return
	}
	if !wire.InterventionKinds[req.Kind] {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": "kind HOLD | STOP | SKIP | BLACKOUT olmalı"})
		return
	}

	msg := wire.Intervention{
		RunID:            req.RunID,
		Kind:             req.Kind,
		IssuedAtServerMs: s.clock.NowMs(),
	}
	data, err := wire.Encode(wire.TypeIntervention, msg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "mesaj kodlanamadı"})
		return
	}
	s.hub.Broadcast(data)
	s.log.Info("müdahale yayınlandı", "kind", req.Kind, "run_id", req.RunID)
	writeJSON(w, http.StatusOK, map[string]any{"kind": req.Kind, "clients": s.hub.Count()})
}

// --- yardımcılar ---

func newRunID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

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
	"context"
	"crypto/rand"
	"crypto/subtle"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/msaliheroglu/tekses/packages/proto/wire"
	"github.com/msaliheroglu/tekses/services/gateway/internal/clock"
	"github.com/msaliheroglu/tekses/services/gateway/internal/hub"
	"github.com/msaliheroglu/tekses/services/gateway/internal/rooms"
)

const (
	// Katılım kodu çözümlemesi için control-api'ye tanınan süre.
	resolveTimeout = 3 * time.Second

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
	resolver   rooms.Resolver
	sessions   rooms.SessionValidator
	upgrader   websocket.Upgrader

	// Doğrulanmış panel oturumlarının kısa süreli önbelleği: konsolun 4 sn'de
	// bir attığı runs sorgusu her seferinde control-api'ye gitmesin.
	// token → önbellek son kullanma anı.
	sessMu    sync.Mutex
	sessCache map[string]time.Time

	// Son çalıştırmaların halka kaydı (asgari telemetri; en yenisi başta).
	// Kalıcı Run kaydı ve panolar Faz 2 telemetri işine devredildi.
	runsMu sync.Mutex
	runs   []runRecord
}

// runRecord, tek bir kue yayını ya da müdahalenin izidir.
type runRecord struct {
	RunID            string `json:"run_id,omitempty"`
	Kind             string `json:"kind"` // "cue" | HOLD | STOP | SKIP | BLACKOUT
	CueID            string `json:"cue_id,omitempty"`
	RoomID           string `json:"room_id,omitempty"`
	FireAtServerMs   int64  `json:"fire_at_server_ms,omitempty"`
	IssuedAtServerMs int64  `json:"issued_at_server_ms"`
	Clients          int    `json:"clients"`
}

const maxRunRecords = 50

func (s *Server) recordRun(rec runRecord) {
	s.runsMu.Lock()
	defer s.runsMu.Unlock()
	s.runs = append([]runRecord{rec}, s.runs...)
	if len(s.runs) > maxRunRecords {
		s.runs = s.runs[:maxRunRecords]
	}
}

// New, bir gateway sunucusu kurar. adminToken boş değilse /api/* uçları
// "Authorization: Bearer <token>" başlığı ister; sessions verilmişse geçerli
// bir panel oturum token'ı da kabul edilir (moderatörün ayrıca yönetici
// anahtarı bilmesi gerekmez). resolver nil ise katılım kodu doğrulanmaz ve
// herkes varsayılan odaya düşer (Faz 0 yerel denemesi).
func New(log *slog.Logger, adminToken string, resolver rooms.Resolver, sessions rooms.SessionValidator) *Server {
	return &Server{
		log:        log,
		clock:      clock.New(),
		hub:        hub.New(log),
		adminToken: adminToken,
		resolver:   resolver,
		sessions:   sessions,
		sessCache:  map[string]time.Time{},
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
	mux.HandleFunc("GET /api/v0/runs", s.handleRuns)
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
		"rooms":          s.hub.RoomCounts(),
		"server_time_ms": s.clock.NowMs(),
	})
}

// Doğrulanmış panel oturumu bu kadar süre önbellekte kalır: çıkış yapan bir
// moderatörün konsol yetkisi en geç bu süre sonunda düşer (kabul edilen gecikme).
const sessionCacheTTL = 60 * time.Second

// checkAdmin, yönetici kilidi açıksa (adminToken ayarlı) Bearer başlığını
// doğrular: statik yönetici anahtarı YA DA geçerli bir panel oturumu kabul
// edilir. Hata yazdıysa false döner.
func (s *Server) checkAdmin(w http.ResponseWriter, r *http.Request) bool {
	if s.adminToken == "" {
		return true // kilit kapalı (Faz 0 yerel denemesi)
	}
	if token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok && token != "" {
		if subtle.ConstantTimeCompare([]byte(token), []byte(s.adminToken)) == 1 {
			return true
		}
		if s.validPanelSession(r.Context(), token) {
			return true
		}
	}
	writeJSON(w, http.StatusUnauthorized, map[string]any{
		"error": "geçersiz veya eksik yönetici token'ı (panelde oturum açmak da yeterlidir)"})
	return false
}

// validPanelSession, token'ı control-api'ye doğrulatır; sonucu kısa süre
// önbellekler. Geçici control-api arızası "geçersiz" sayılır (yönetici ucu
// açık kalmaz) ama günlüğe geçersiz oturumdan farklı yazılır.
func (s *Server) validPanelSession(ctx context.Context, token string) bool {
	if s.sessions == nil {
		return false
	}
	now := time.Now()
	s.sessMu.Lock()
	exp, ok := s.sessCache[token]
	s.sessMu.Unlock()
	if ok && now.Before(exp) {
		return true
	}
	vctx, cancel := context.WithTimeout(ctx, resolveTimeout)
	defer cancel()
	if err := s.sessions.ValidateSession(vctx, token); err != nil {
		if !errors.Is(err, rooms.ErrInvalidSession) {
			s.log.Warn("panel oturumu doğrulanamadı", "hata", err)
		}
		return false
	}
	s.sessMu.Lock()
	// Kaba temizlik: süresi geçmiş girdiler önbelleği şişirmesin.
	if len(s.sessCache) > 1024 {
		for k, e := range s.sessCache {
			if now.After(e) {
				delete(s.sessCache, k)
			}
		}
	}
	s.sessCache[token] = now.Add(sessionCacheTTL)
	s.sessMu.Unlock()
	return true
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Content-Type zorunluluğu ucuz bir CSRF önlemidir: tarayıcı,
		// preflight'sız çapraz-site isteklerde application/json gönderemez.
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			writeJSON(w, http.StatusUnsupportedMediaType, map[string]any{"error": "Content-Type: application/json gerekli"})
			return
		}
		if !s.checkAdmin(w, r) {
			return
		}
		next(w, r)
	}
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	if !s.checkAdmin(w, r) {
		return
	}
	s.runsMu.Lock()
	runs := make([]runRecord, len(s.runs))
	copy(runs, s.runs)
	s.runsMu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
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
		frameKind, raw, err := conn.ReadMessage()
		// t1 olabildiğince erken, çözümlemeden önce damgalanır.
		recvMs := s.clock.NowMs()
		if err != nil {
			return
		}
		resetDeadline()

		// Çerçevenin biçimi kodeki söyler: metin = JSON (v1),
		// ikili = protobuf (v2). Yanıtlar istemcinin hello kodeğini izler.
		binaryFrame := frameKind == websocket.BinaryMessage
		var msgType string
		var payload any
		if binaryFrame {
			msgType, payload, err = wire.DecodeBinary(raw)
		} else {
			msgType, payload, err = wire.DecodeMessage(raw)
		}
		if err != nil {
			s.log.Warn("bozuk çerçeve", "ikili", binaryFrame, "hata", err)
			continue
		}

		switch msgType {
		case wire.TypeHello:
			hello := payload.(wire.Hello)
			if hello.ProtocolVersion != wire.ProtocolVersion && hello.ProtocolVersion != wire.ProtocolVersionBinary {
				s.log.Warn("uyumsuz protokol sürümü", "istemci", hello.ProtocolVersion)
				return
			}
			client.SetBinary(binaryFrame)
			room := hub.DefaultRoom
			code := strings.ToUpper(strings.TrimSpace(hello.JoinCode))
			if code != "" && s.resolver != nil {
				ctx, cancel := context.WithTimeout(r.Context(), resolveTimeout)
				resolved, err := s.resolver.ResolveJoinCode(ctx, code)
				cancel()
				if err != nil {
					// Geçersiz kod da geçici control-api arızası da katılımı
					// reddeder; istemci jitter'lı geri çekilmeyle yeniden dener.
					s.log.Warn("katılım kodu çözülemedi", "kod", code, "hata", err)
					return
				}
				room = resolved
			}
			s.hub.JoinRoom(client, room)
			s.send(client, wire.TypeWelcome, wire.Welcome{
				ServerTimeMs:    s.clock.NowMs(),
				ProtocolVersion: hello.ProtocolVersion,
				RoomID:          room,
			})

		case wire.TypeClockSyncRequest:
			req := payload.(wire.ClockSyncRequest)
			// t2, yazma kilidi alındıktan sonra (SendLazy içinde) damgalanır:
			// kilidin beklettiği süre t2'ye yansır, ofset saptırılmaz.
			err := client.SendLazy(func() ([]byte, error) {
				resp := wire.ClockSyncResponse{
					Seq:          req.Seq,
					ClientMonoMs: req.ClientMonoMs,
					ServerRecvMs: recvMs,
					ServerSendMs: s.clock.NowMs(),
				}
				if client.IsBinary() {
					return wire.EncodeBinary(wire.TypeClockSyncResponse, resp)
				}
				return wire.Encode(wire.TypeClockSyncResponse, resp)
			})
			if err != nil {
				s.hub.Unregister(client)
				return
			}

		default:
			s.log.Warn("beklenmeyen mesaj türü", "tür", msgType)
		}
	}
}

func (s *Server) send(c *hub.Client, msgType string, msg any) {
	var data []byte
	var err error
	if c.IsBinary() {
		data, err = wire.EncodeBinary(msgType, msg)
	} else {
		data, err = wire.Encode(msgType, msg)
	}
	if err != nil {
		s.log.Error("mesaj kodlanamadı", "tür", msgType, "hata", err)
		return
	}
	if err := c.Send(data); err != nil {
		s.hub.Unregister(c)
	}
}

// encodeFrame, yayın mesajını iki kodlamada birden üretir.
func (s *Server) encodeFrame(msgType string, msg any) (hub.Frame, error) {
	jsonData, err := wire.Encode(msgType, msg)
	if err != nil {
		return hub.Frame{}, err
	}
	binData, err := wire.EncodeBinary(msgType, msg)
	if err != nil {
		return hub.Frame{}, err
	}
	return hub.Frame{JSON: jsonData, Binary: binData}, nil
}

// --- Kontrol API'si ---

type cueRequest struct {
	CueID      string `json:"cue_id"`
	RoomID     string `json:"room_id"` // boş: tüm odalara (Faz 0 davranışı)
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
	s.broadcastCueWithRepeats(cue, req.RoomID)

	targetCount := s.hub.Count()
	if req.RoomID != "" {
		targetCount = s.hub.RoomCounts()[req.RoomID]
	}
	s.log.Info("kue yayınlandı",
		"run_id", cue.RunID, "cue_id", cue.CueID, "oda", req.RoomID,
		"fire_at", cue.FireAtServerMs, "istemci", targetCount)
	s.recordRun(runRecord{
		RunID:            cue.RunID,
		Kind:             "cue",
		CueID:            cue.CueID,
		RoomID:           req.RoomID,
		FireAtServerMs:   cue.FireAtServerMs,
		IssuedAtServerMs: s.clock.NowMs(),
		Clients:          targetCount,
	})

	writeJSON(w, http.StatusOK, map[string]any{
		"run_id":            cue.RunID,
		"cue_id":            cue.CueID,
		"room_id":           req.RoomID,
		"fire_at_server_ms": cue.FireAtServerMs,
		"server_time_ms":    s.clock.NowMs(),
		"clients":           targetCount,
	})
}

// broadcastCueWithRepeats, aynı kueyi cueRepeats kez yayınlar; istemciler
// run_id ile tekilleştirir. İlk tekrar hemen, sonrakiler aralıklarla gider.
// room boşsa tüm istemcilere, doluysa yalnızca o odaya gider.
func (s *Server) broadcastCueWithRepeats(cue wire.CueStart, room string) {
	for i := uint32(1); i <= cueRepeats; i++ {
		repeat := cue
		repeat.RepeatSeq = i
		frame, err := s.encodeFrame(wire.TypeCueStart, repeat)
		if err != nil {
			s.log.Error("kue kodlanamadı", "hata", err)
			return
		}
		delay := time.Duration(i-1) * cueRepeatInterval
		time.AfterFunc(delay, func() {
			if room == "" {
				s.hub.Broadcast(frame)
			} else {
				s.hub.BroadcastRoom(room, frame)
			}
		})
	}
}

type interventionRequest struct {
	RunID  string `json:"run_id"`
	RoomID string `json:"room_id"` // boş: tüm odalara
	Kind   string `json:"kind"`
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
	frame, err := s.encodeFrame(wire.TypeIntervention, msg)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "mesaj kodlanamadı"})
		return
	}
	if req.RoomID == "" {
		s.hub.Broadcast(frame)
	} else {
		s.hub.BroadcastRoom(req.RoomID, frame)
	}
	s.log.Info("müdahale yayınlandı", "kind", req.Kind, "run_id", req.RunID, "oda", req.RoomID)
	s.recordRun(runRecord{
		RunID:            req.RunID,
		Kind:             req.Kind,
		RoomID:           req.RoomID,
		IssuedAtServerMs: msg.IssuedAtServerMs,
		Clients:          s.hub.Count(),
	})
	writeJSON(w, http.StatusOK, map[string]any{"kind": req.Kind, "room_id": req.RoomID, "clients": s.hub.Count()})
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

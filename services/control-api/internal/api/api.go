// Package api, kontrol API'sinin REST yüzeyidir.
//
// Kimlik: pilot kararı gereği e-posta + şifre (bcrypt) ve bearer token
// oturumu. Kiracılık: her istek, oturumun org'una daraltılır; başka
// kiracının kaynağı "yok" gibi görünür (404).
package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/msaliheroglu/tekses/services/control-api/internal/model"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store"
)

const (
	maxBodyBytes    = 1 << 20 // manifestler için 1 MiB üst sınır
	minPasswordLen  = 8
	joinCodeLen     = 6
	joinCodeRetries = 5
)

// Karışması kolay karakterler (0/O, 1/I/L) alfabede yok: kod stadyumda
// sesli okunup elle yazılabilmeli.
const joinCodeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"

type Server struct {
	log   *slog.Logger
	store store.Store
}

func New(log *slog.Logger, st store.Store) *Server {
	return &Server{log: log, store: st}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("POST /api/v1/auth/register", s.requireJSON(s.handleRegister))
	mux.HandleFunc("POST /api/v1/auth/login", s.requireJSON(s.handleLogin))

	mux.HandleFunc("GET /api/v1/events", s.authed(s.handleListEvents))
	mux.HandleFunc("POST /api/v1/events", s.authedJSON(s.handleCreateEvent))
	mux.HandleFunc("GET /api/v1/events/{id}", s.authed(s.handleGetEvent))
	mux.HandleFunc("GET /api/v1/events/{id}/rooms", s.authed(s.handleListRooms))
	mux.HandleFunc("POST /api/v1/events/{id}/rooms", s.authedJSON(s.handleCreateRoom))

	mux.HandleFunc("GET /api/v1/shows", s.authed(s.handleListShows))
	mux.HandleFunc("POST /api/v1/shows", s.authedJSON(s.handleCreateShow))

	return mux
}

// --- ara katmanlar ---

type authedHandler func(w http.ResponseWriter, r *http.Request, sess model.Session)

// requireJSON: gövdeli uçlarda Content-Type zorunluluğu (gateway'dekiyle
// aynı ucuz CSRF önlemi) + gövde boyutu sınırı.
func (s *Server) requireJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
			writeErr(w, http.StatusUnsupportedMediaType, "Content-Type: application/json gerekli")
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
		next(w, r)
	}
}

func (s *Server) authed(next authedHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || token == "" {
			writeErr(w, http.StatusUnauthorized, "Bearer token gerekli")
			return
		}
		sess, err := s.store.SessionByToken(token)
		if err != nil {
			writeErr(w, http.StatusUnauthorized, "geçersiz oturum")
			return
		}
		next(w, r, sess)
	}
}

func (s *Server) authedJSON(next authedHandler) http.HandlerFunc {
	return s.requireJSON(s.authed(next))
}

// --- kimlik ---

type registerRequest struct {
	Organization string `json:"organization"`
	Email        string `json:"email"`
	Password     string `json:"password"`
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "gövde çözülemedi")
		return
	}
	req.Organization = strings.TrimSpace(req.Organization)
	req.Email = normalizeEmail(req.Email)
	switch {
	case req.Organization == "":
		writeErr(w, http.StatusBadRequest, "organization boş olamaz")
		return
	case !strings.Contains(req.Email, "@"):
		writeErr(w, http.StatusBadRequest, "geçerli bir e-posta gerekli")
		return
	case len(req.Password) < minPasswordLen:
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("şifre en az %d karakter olmalı", minPasswordLen))
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "şifre işlenemedi")
		return
	}

	now := time.Now().UTC()
	org := model.Organization{ID: newID("org"), Name: req.Organization, CreatedAt: now}
	user := model.User{ID: newID("usr"), OrgID: org.ID, Email: req.Email, PasswordHash: hash, CreatedAt: now}

	// Önce kullanıcı: e-posta çakışırsa ortada sahipsiz org kalmasın.
	if err := s.store.CreateUser(user); err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeErr(w, http.StatusConflict, "bu e-posta zaten kayıtlı")
			return
		}
		writeErr(w, http.StatusInternalServerError, "kayıt başarısız")
		return
	}
	if err := s.store.CreateOrganization(org); err != nil {
		writeErr(w, http.StatusInternalServerError, "kayıt başarısız")
		return
	}

	token, err := s.newSession(user)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "oturum açılamadı")
		return
	}
	s.log.Info("yeni organizasyon", "org", org.ID, "ad", org.Name)
	writeJSON(w, http.StatusCreated, map[string]any{"token": token, "organization": org})
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "gövde çözülemedi")
		return
	}
	user, err := s.store.UserByEmail(normalizeEmail(req.Email))
	if err != nil {
		// Zamanlama farkı e-posta varlığını sızdırmasın diye yine de bir
		// karşılaştırma maliyeti ödenir.
		_ = bcrypt.CompareHashAndPassword(dummyHash, []byte(req.Password))
		writeErr(w, http.StatusUnauthorized, "e-posta veya şifre hatalı")
		return
	}
	if bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.Password)) != nil {
		writeErr(w, http.StatusUnauthorized, "e-posta veya şifre hatalı")
		return
	}
	token, err := s.newSession(user)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "oturum açılamadı")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"token": token})
}

var dummyHash = func() []byte {
	h, _ := bcrypt.GenerateFromPassword([]byte("zamanlama-esitleme"), bcrypt.DefaultCost)
	return h
}()

func (s *Server) newSession(user model.User) (string, error) {
	token, err := randomHex(16)
	if err != nil {
		return "", err
	}
	return token, s.store.CreateSession(model.Session{
		Token:     token,
		UserID:    user.ID,
		OrgID:     user.OrgID,
		CreatedAt: time.Now().UTC(),
	})
}

// --- etkinlikler ---

type createEventRequest struct {
	Name  string `json:"name"`
	Venue string `json:"venue"`
}

func (s *Server) handleCreateEvent(w http.ResponseWriter, r *http.Request, sess model.Session) {
	var req createEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "gövde çözülemedi")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name boş olamaz")
		return
	}
	event := model.Event{
		ID:        newID("ev"),
		OrgID:     sess.OrgID,
		Name:      req.Name,
		Venue:     strings.TrimSpace(req.Venue),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.store.CreateEvent(event); err != nil {
		writeErr(w, http.StatusInternalServerError, "etkinlik oluşturulamadı")
		return
	}
	writeJSON(w, http.StatusCreated, event)
}

func (s *Server) handleListEvents(w http.ResponseWriter, _ *http.Request, sess model.Session) {
	events, err := s.store.ListEvents(sess.OrgID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "etkinlikler listelenemedi")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"events": emptyIfNil(events)})
}

func (s *Server) handleGetEvent(w http.ResponseWriter, r *http.Request, sess model.Session) {
	event, err := s.store.EventByID(sess.OrgID, r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "etkinlik bulunamadı")
		return
	}
	writeJSON(w, http.StatusOK, event)
}

// --- odalar ---

type createRoomRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateRoom(w http.ResponseWriter, r *http.Request, sess model.Session) {
	event, err := s.store.EventByID(sess.OrgID, r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "etkinlik bulunamadı")
		return
	}
	var req createRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "gövde çözülemedi")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeErr(w, http.StatusBadRequest, "name boş olamaz")
		return
	}

	// joinCode küresel benzersizdir; çakışmada yeni kod denenir.
	var room model.Room
	for attempt := 0; ; attempt++ {
		code, err := newJoinCode()
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "katılım kodu üretilemedi")
			return
		}
		room = model.Room{
			ID:        newID("room"),
			OrgID:     sess.OrgID,
			EventID:   event.ID,
			Name:      req.Name,
			JoinCode:  code,
			CreatedAt: time.Now().UTC(),
		}
		err = s.store.CreateRoom(room)
		if err == nil {
			break
		}
		if !errors.Is(err, store.ErrConflict) || attempt >= joinCodeRetries {
			writeErr(w, http.StatusInternalServerError, "oda oluşturulamadı")
			return
		}
	}
	writeJSON(w, http.StatusCreated, room)
}

func (s *Server) handleListRooms(w http.ResponseWriter, r *http.Request, sess model.Session) {
	if _, err := s.store.EventByID(sess.OrgID, r.PathValue("id")); err != nil {
		writeErr(w, http.StatusNotFound, "etkinlik bulunamadı")
		return
	}
	rooms, err := s.store.ListRooms(sess.OrgID, r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "odalar listelenemedi")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rooms": emptyIfNil(rooms)})
}

// --- gösteriler ---

type createShowRequest struct {
	Title string `json:"title"`
}

func (s *Server) handleCreateShow(w http.ResponseWriter, r *http.Request, sess model.Session) {
	var req createShowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "gövde çözülemedi")
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		writeErr(w, http.StatusBadRequest, "title boş olamaz")
		return
	}
	show := model.Show{ID: newID("show"), OrgID: sess.OrgID, Title: req.Title, CreatedAt: time.Now().UTC()}
	if err := s.store.CreateShow(show); err != nil {
		writeErr(w, http.StatusInternalServerError, "gösteri oluşturulamadı")
		return
	}
	writeJSON(w, http.StatusCreated, show)
}

func (s *Server) handleListShows(w http.ResponseWriter, _ *http.Request, sess model.Session) {
	shows, err := s.store.ListShows(sess.OrgID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "gösteriler listelenemedi")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"shows": emptyIfNil(shows)})
}

// --- yardımcılar ---

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func newID(prefix string) string {
	hexPart, err := randomHex(6)
	if err != nil {
		// crypto/rand'ın çökmesi süreç düzeyinde bir felakettir; kimliksiz
		// kayıt üretmektense panik doğrudur.
		panic(fmt.Sprintf("api: rastgele kimlik üretilemedi: %v", err))
	}
	return prefix + "_" + hexPart
}

func newJoinCode() (string, error) {
	buf := make([]byte, joinCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	for i, b := range buf {
		buf[i] = joinCodeAlphabet[int(b)%len(joinCodeAlphabet)]
	}
	return string(buf), nil
}

func randomHex(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

// emptyIfNil: boş listeler JSON'da null değil [] görünsün.
func emptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}

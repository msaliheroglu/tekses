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
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/msaliheroglu/tekses/packages/blob"
	"github.com/msaliheroglu/tekses/packages/manifest"
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
	log      *slog.Logger
	store    store.Store
	packages blob.Store
}

// New, bir kontrol API sunucusu kurar. packages, yayınlanan manifestlerin
// içerik adresli paket deposudur (pilotta dosya sistemi + bu API'nin
// /packages ucu; üretimde R2 + CDN).
func New(log *slog.Logger, st store.Store, packages blob.Store) *Server {
	return &Server{log: log, store: st, packages: packages}
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
	mux.HandleFunc("GET /api/v1/shows/{id}/versions", s.authed(s.handleListShowVersions))
	mux.HandleFunc("POST /api/v1/shows/{id}/versions", s.authedJSON(s.handlePublishShowVersion))
	mux.HandleFunc("GET /api/v1/show-versions/{id}", s.authed(s.handleGetShowVersion))
	mux.HandleFunc("POST /api/v1/rooms/{id}/activate", s.authedJSON(s.handleActivateRoom))

	// Herkese açık katılım ucu: telefon, kodla oda + aktif gösteri sürümünü
	// çeker. Kimlik istemez (katılımcı hesapsızdır, karar §1).
	mux.HandleFunc("GET /api/v1/join/{code}", s.handleJoin)

	// Paket indirme (herkese açık, değişmez, agresif önbelleklenebilir).
	// Üretimde bu yol CDN/R2'ye devrolur; sözleşme aynı kalır:
	// /packages/<sha256>.json ve içerik özetle doğrulanır.
	mux.HandleFunc("GET /packages/{name}", s.handlePackage)

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

	if err := s.store.CreateOrgWithUser(org, user); err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeErr(w, http.StatusConflict, "bu e-posta zaten kayıtlı")
			return
		}
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

// --- gösteri sürümleri ve yayınlama ---

// handlePublishShowVersion, gövde olarak ham manifest alır; doğrular,
// kanonikleştirir, SHA-256 özetini alır ve DEĞİŞMEZ bir sürüm yaratır.
// Sürüm numarası gösteri başına 1'den artar.
func (s *Server) handlePublishShowVersion(w http.ResponseWriter, r *http.Request, sess model.Session) {
	show, err := s.store.ShowByID(sess.OrgID, r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "gösteri bulunamadı")
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "gövde okunamadı")
		return
	}
	m, err := manifest.Parse(raw)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	canonical, sum, err := m.Canonical()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "manifest kanonikleştirilemedi")
		return
	}
	// Paket önce depoya yazılır: sürüm kaydı varsa paketi de VAR olmalı
	// (telefonlar indirmeye başladığında 404 görmemeli). İçerik adresli
	// yazım idempotent olduğundan yarıda kesilme tekrar denemeyle çözülür.
	if err := s.packages.Put(r.Context(), packageKey(sum), canonical); err != nil {
		s.log.Error("paket yazılamadı", "hata", err)
		writeErr(w, http.StatusInternalServerError, "paket depolanamadı")
		return
	}
	sv, err := s.store.CreateShowVersion(model.ShowVersion{
		ID:           newID("sv"),
		OrgID:        sess.OrgID,
		ShowID:       show.ID,
		ManifestJSON: canonical,
		SHA256:       sum,
		CreatedAt:    time.Now().UTC(),
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "sürüm oluşturulamadı")
		return
	}
	s.log.Info("gösteri sürümü yayınlandı", "show", show.ID, "sürüm", sv.Version, "sha256", sum[:12])
	writeJSON(w, http.StatusCreated, sv)
}

func (s *Server) handleListShowVersions(w http.ResponseWriter, r *http.Request, sess model.Session) {
	if _, err := s.store.ShowByID(sess.OrgID, r.PathValue("id")); err != nil {
		writeErr(w, http.StatusNotFound, "gösteri bulunamadı")
		return
	}
	versions, err := s.store.ListShowVersions(sess.OrgID, r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "sürümler listelenemedi")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": emptyIfNil(versions)})
}

func (s *Server) handleGetShowVersion(w http.ResponseWriter, r *http.Request, sess model.Session) {
	sv, err := s.store.ShowVersionByID(sess.OrgID, r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "sürüm bulunamadı")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"version":  sv,
		"manifest": json.RawMessage(sv.ManifestJSON),
	})
}

type activateRequest struct {
	ShowVersionID string `json:"show_version_id"`
}

func (s *Server) handleActivateRoom(w http.ResponseWriter, r *http.Request, sess model.Session) {
	room, err := s.store.RoomByID(sess.OrgID, r.PathValue("id"))
	if err != nil {
		writeErr(w, http.StatusNotFound, "oda bulunamadı")
		return
	}
	var req activateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "gövde çözülemedi")
		return
	}
	if _, err := s.store.ShowVersionByID(sess.OrgID, req.ShowVersionID); err != nil {
		writeErr(w, http.StatusNotFound, "sürüm bulunamadı")
		return
	}
	if err := s.store.SetRoomActiveVersion(sess.OrgID, room.ID, req.ShowVersionID); err != nil {
		writeErr(w, http.StatusInternalServerError, "oda güncellenemedi")
		return
	}
	s.log.Info("odada gösteri etkinleştirildi", "oda", room.ID, "sürüm", req.ShowVersionID)
	writeJSON(w, http.StatusOK, map[string]any{"room_id": room.ID, "active_show_version_id": req.ShowVersionID})
}

// --- katılım (herkese açık) ---

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))
	room, err := s.store.RoomByJoinCode(code)
	if err != nil {
		writeErr(w, http.StatusNotFound, "katılım kodu geçersiz")
		return
	}
	event, err := s.store.EventByID(room.OrgID, room.EventID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "oda tutarsız")
		return
	}

	resp := map[string]any{
		"room_id":    room.ID,
		"room_name":  room.Name,
		"event_name": event.Name,
	}
	if room.ActiveShowVersionID != "" {
		sv, err := s.store.ShowVersionByID(room.OrgID, room.ActiveShowVersionID)
		if err == nil {
			resp["show_version"] = map[string]any{
				"id":      sv.ID,
				"version": sv.Version,
				"sha256":  sv.SHA256,
				// Telefonun tercih etmesi gereken yol: paketi bu adresten
				// indir, SHA-256 ile doğrula (60k telefon CDN'den çeker).
				"manifest_url": "/packages/" + packageKey(sv.SHA256),
				// Küçük manifestler için kolaylık; tel sözleşmesi URL'dir.
				"manifest": json.RawMessage(sv.ManifestJSON),
			}
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func packageKey(sha256Hex string) string { return sha256Hex + ".json" }

var packageNameRe = regexp.MustCompile(`^[0-9a-f]{64}\.json$`)

func (s *Server) handlePackage(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !packageNameRe.MatchString(name) {
		writeErr(w, http.StatusNotFound, "paket bulunamadı")
		return
	}
	data, err := s.packages.Get(r.Context(), name)
	if err != nil {
		writeErr(w, http.StatusNotFound, "paket bulunamadı")
		return
	}
	// İçerik adresli ve değişmez: sonsuza dek önbelleklenebilir.
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	_, _ = w.Write(data)
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

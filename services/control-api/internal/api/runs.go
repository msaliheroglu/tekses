package api

// Kalıcı Run izleri (F2.6 telemetri): kue yayınları ve müdahaleler.
//
// Yazma yolu iç uçtur: kaydı, olayı işleyen gateway düğümü üretir ve
// TEKSES_INTERNAL_TOKEN paylaşımlı sırrıyla buraya POST eder. Kayıp,
// bilinçli olarak tolere edilir (control-api çökükse kue engellenmez;
// gateway Warn loglar) — kue teli zaten kayba dayanıklı tasarlandı.
// Okuma yolu panelin org kapsamlı listesidir.

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/msaliheroglu/tekses/services/control-api/internal/model"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store"
)

const (
	defaultRunsLimit = 50
	maxRunsLimit     = 200
)

// internalRunRequest, gateway'in gönderdiği kayıttır (alan adları gateway'in
// runRecord'uyla ve tel JSON'uyla hizalı).
type internalRunRequest struct {
	ID               string `json:"id"`
	RoomID           string `json:"room_id"`
	Kind             string `json:"kind"`
	RunID            string `json:"run_id"`
	CueID            string `json:"cue_id"`
	FireAtServerMs   int64  `json:"fire_at_server_ms"`
	IssuedAtServerMs int64  `json:"issued_at_server_ms"`
	Clients          int    `json:"clients"`
	Node             string `json:"node"`
}

func (s *Server) handleInternalCreateRun(w http.ResponseWriter, r *http.Request) {
	// Sır ayarlanmamışsa uç YOK sayılır: kimliksiz yazma kapısı açılmaz.
	if s.internalToken == "" {
		writeErr(w, http.StatusNotFound, "iç uç yapılandırılmamış")
		return
	}
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token != s.internalToken {
		writeErr(w, http.StatusUnauthorized, "geçersiz iç uç token'ı")
		return
	}

	var req internalRunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "gövde çözülemedi")
		return
	}
	if req.ID == "" || req.Kind == "" {
		writeErr(w, http.StatusBadRequest, "id ve kind zorunlu")
		return
	}

	// Oda→org çözümü burada yapılır (kiracılık mantığı control-api'de kalır).
	// Boş oda ("tüm odalara"), Faz 0 odası ya da bilinmeyen oda → org'suz
	// kayıt: saklanır ama org kapsamlı listede görünmez.
	orgID := ""
	if req.RoomID != "" {
		if id, err := s.store.OrgIDByRoom(req.RoomID); err == nil {
			orgID = id
		} else if !errors.Is(err, store.ErrNotFound) {
			writeErr(w, http.StatusInternalServerError, "oda çözülemedi")
			return
		}
	}

	run := model.Run{
		ID:               req.ID,
		OrgID:            orgID,
		RoomID:           req.RoomID,
		Kind:             req.Kind,
		RunID:            req.RunID,
		CueID:            req.CueID,
		FireAtServerMs:   req.FireAtServerMs,
		IssuedAtServerMs: req.IssuedAtServerMs,
		Clients:          req.Clients,
		Node:             req.Node,
		CreatedAt:        time.Now().UTC(),
	}
	if err := s.store.CreateRun(run); err != nil {
		writeErr(w, http.StatusInternalServerError, "kayıt yazılamadı")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": run.ID})
}

func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request, sess model.Session) {
	limit := defaultRunsLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeErr(w, http.StatusBadRequest, "limit pozitif tam sayı olmalı")
			return
		}
		limit = min(n, maxRunsLimit)
	}
	runs, err := s.store.ListRuns(sess.OrgID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "kayıtlar listelenemedi")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs})
}

package server

// Küme geneli varlık sayacı (F2.6 telemetri): her düğüm 5 sn'de bir kendi
// sayımını NATS'e duyurur; her düğüm diğerlerinin SON raporunu tutar.
// /api/v0/presence yanıtı = yerel CANLI sayım + taze (≤15 sn) peer
// raporlarının toplamı — hangi kopyaya sorulursa sorulsun küme genelidir.
// Sayaç YAKLAŞIKTIR: yeniden bağlanan istemci kısa süre iki düğümde
// sayılabilir, raporlar 5 sn bayat olabilir; kue kararları buna bağlanmaz.

import (
	"net/http"
	"time"

	"github.com/msaliheroglu/tekses/services/gateway/internal/fanout"
)

const (
	presenceInterval = 5 * time.Second
	// Bir peer raporu bu süreyi aşarsa düğüm ölü sayılır ve toplamdan düşer.
	presenceStale = 15 * time.Second
)

// presenceEntry, bir peer'ın son raporu + ALICININ monoton saatiyle alınma
// anıdır (bayatlama denetimi gönderenin ts'iyle yapılmaz: düğümler arası
// duvar saati kayması sağlıklı düğümü bayat gösterebilir).
type presenceEntry struct {
	rep        fanout.PresenceReport
	receivedAt time.Time
}

// StartPresence, varlık yayınını başlatır: kendi raporunu hemen ve sonra
// 5 sn'de bir yayınlar, peer raporlarını toplar. Dönen stop, yayını durdurur
// (main'de bus.Close'tan ÖNCE çağrılacak şekilde defer edilir).
func (s *Server) StartPresence(p fanout.Presence) (stop func()) {
	if err := p.SubscribePresence(func(rep fanout.PresenceReport) {
		if rep.NodeID == s.nodeID {
			return // NATS kendi yayınımızı da teslim eder; çift sayma olmasın
		}
		s.presMu.Lock()
		s.peers[rep.NodeID] = presenceEntry{rep: rep, receivedAt: time.Now()}
		s.presMu.Unlock()
	}); err != nil {
		s.log.Error("presence aboneliği kurulamadı", "hata", err)
		return func() {}
	}

	publish := func() {
		rep := fanout.PresenceReport{
			NodeID: s.nodeID,
			Total:  s.hub.Count(),
			Rooms:  s.hub.RoomCounts(),
			Ts:     time.Now().UnixMilli(),
		}
		if err := p.PublishPresence(rep); err != nil {
			s.log.Warn("presence yayını başarısız", "hata", err)
		}
	}
	// İlk rapor hemen: yeni düğüm kümede 5 sn beklemeden görünür.
	publish()

	done := make(chan struct{})
	go func() {
		t := time.NewTicker(presenceInterval)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				publish()
				s.prunePeers()
			}
		}
	}()
	return func() { close(done) }
}

// prunePeers, bayat raporları düşürür ki ölü düğüm kimlikleri (her yeniden
// başlatma yeni kimlik üretir) haritada sonsuza dek birikmesin.
func (s *Server) prunePeers() {
	now := time.Now()
	s.presMu.Lock()
	for id, e := range s.peers {
		if now.Sub(e.receivedAt) > presenceStale {
			delete(s.peers, id)
		}
	}
	s.presMu.Unlock()
}

// handlePresence — GET /api/v0/presence: küme geneli yaklaşık sayım.
// NATS'siz tek düğümde de aynı şemayla (node_count=1, yalnız yerel sayım)
// döner; panel tek/çok düğümü aynı tiple okur. İşletmen token'ı ister
// (kiracılar arası veri — checkOperator gerekçesi).
func (s *Server) handlePresence(w http.ResponseWriter, r *http.Request) {
	if !s.checkOperator(w, r) {
		return
	}
	type nodeInfo struct {
		NodeID string `json:"node_id"`
		Total  int    `json:"total"`
		AgeMs  int64  `json:"age_ms"` // raporun yaşı (alıcı saati)
	}

	total := s.hub.Count()
	rooms := s.hub.RoomCounts()
	nodes := []nodeInfo{{NodeID: s.nodeID, Total: total, AgeMs: 0}}

	now := time.Now()
	s.presMu.Lock()
	for _, e := range s.peers {
		age := now.Sub(e.receivedAt)
		if age > presenceStale {
			continue
		}
		total += e.rep.Total
		for room, n := range e.rep.Rooms {
			rooms[room] += n
		}
		nodes = append(nodes, nodeInfo{NodeID: e.rep.NodeID, Total: e.rep.Total, AgeMs: age.Milliseconds()})
	}
	s.presMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"node_id":    s.nodeID,
		"node_count": len(nodes),
		"total":      total,
		"rooms":      rooms,
		"nodes":      nodes,
		// Panel etiketi için: sayım yaklaşıktır, ≤15 sn gecikmeli olabilir.
		"approx": true,
	})
}

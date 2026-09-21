package server

// Saat kalitesi ısı haritası (F2.6 telemetri) — sunucu tarafı ping-RTT.
//
// Gösterge RTT'dir, ofset DEĞİL: istemcinin ofset kestirim algoritması
// belirsizliği RTT/2 ile sınırladığından "kötü RTT = güvenilmez ofset"
// ilişkisi matematikseldir; RTT dürüst bir senkron kalite VEKİLİDİR.
// Tel değişikliği sıfırdır: gateway'in zaten attığı keepalive ping'ine
// gönderim damgası konur, tüm istemciler (Flutter, tarayıcı, loadgen)
// pong'da gövdeyi otomatik yankılar (RFC 6455).
//
// İlk örnek bağlantıdan ~pingInterval (50 sn) sonra gelir ve o aralıkta
// bayatlar — panel "örnek yok" kovasını ayrıca gösterir ki yeni dolan
// salon kırmızı görünmesin. Yanıt YALNIZ bu düğümün istemcilerini kapsar
// (runs.clients ile aynı uyarı); küme birleştirme kalıcı telemetri katmanına
// kalır.

import (
	"net/http"
	"sort"
)

// Isı kovaları sunucuda sabitlenir ki panel ve diğer tüketiciler aynı ısı
// tanımını kullansın. lt30 sınırı ürünün ≤30 ms hedefiyle hizalıdır.
const (
	bucketLt10  = 10
	bucketLt30  = 30
	bucketLt100 = 100
	// Bir örnek 3 ping aralığından eskiyse bayat sayılır (kopmuş/duraklamış
	// istemci ısıyı çarpıtmasın).
	staleAfterMs = 3 * 50_000
)

type roomClockStats struct {
	Clients  int   `json:"clients"`
	Sampled  int   `json:"sampled"`
	NoSample int   `json:"no_sample"`
	Stale    int   `json:"stale"`
	P50Ms    int64 `json:"p50_ms"`
	P95Ms    int64 `json:"p95_ms"`
	// Kovalar yalnız TAZE örnekler üzerinden sayılır.
	Lt10   int `json:"lt10"`
	Lt30   int `json:"lt30"`
	Lt100  int `json:"lt100"`
	Gte100 int `json:"gte100"`
}

// handleClockStats — GET /api/v0/clockstats: oda bazlı RTT dağılımı.
// İşletmen token'ı ister (kiracılar arası veri — checkOperator gerekçesi).
func (s *Server) handleClockStats(w http.ResponseWriter, r *http.Request) {
	if !s.checkOperator(w, r) {
		return
	}
	now := s.clock.NowMs()
	samples, noSample := s.hub.RoomRTTSamples()

	rooms := map[string]roomClockStats{}
	// Örneği olmayan istemcileri de odalarıyla listele (yeni dolan salon).
	for room, n := range noSample {
		st := rooms[room]
		st.NoSample = n
		st.Clients += n
		rooms[room] = st
	}
	for room, list := range samples {
		st := rooms[room]
		st.Clients += len(list)
		fresh := make([]int64, 0, len(list))
		for _, sm := range list {
			if now-sm.AtMs > staleAfterMs {
				st.Stale++
				continue
			}
			fresh = append(fresh, sm.RTTMs)
			switch {
			case sm.RTTMs < bucketLt10:
				st.Lt10++
			case sm.RTTMs < bucketLt30:
				st.Lt30++
			case sm.RTTMs < bucketLt100:
				st.Lt100++
			default:
				st.Gte100++
			}
		}
		st.Sampled = len(fresh)
		if len(fresh) > 0 {
			sort.Slice(fresh, func(i, j int) bool { return fresh[i] < fresh[j] })
			st.P50Ms = fresh[len(fresh)/2]
			st.P95Ms = fresh[(len(fresh)*95)/100]
		}
		rooms[room] = st
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"node_id":          s.nodeID,
		"ping_interval_ms": pingInterval.Milliseconds(),
		"rooms":            rooms,
	})
}

package server

import (
	"sync"

	"github.com/msaliheroglu/tekses/packages/proto/wire"
	"github.com/msaliheroglu/tekses/services/gateway/internal/hub"
)

// Geç katılım tekrarı (F3.1): odada koreografi başladıktan SONRA katılan
// telefon, süren koşuya ortasından yetişmelidir. Gateway bunun için oda
// başına SON kue'yi tutar ve yeni katılana welcome'dan hemen sonra tek
// seferlik yeniden gönderir; telefon fireAt'i geçmişte görünce koreografiyi
// kaldığı yerden oynatır (CueScheduler geç ateşler, motor frameAt(elapsed)
// ile ortadan sürer, biten koşu kendiliğinden no-op'tur).
//
// Gözlem noktası dağıtım sink'idir: çok düğümde HER düğüm, hangi düğüm
// yayınlarsa yayınlasın tüm çerçeveleri NATS'ten alır (yayımcı dahil) —
// yani bu hafıza düğümler arasında kendiliğinden tutarlıdır. Düğüm yeniden
// başlarsa hafıza boş başlar: o andan sonra katılanlar süren koşuyu kaçırır;
// kabul edilen bir sınırdır (kue teli de kayba dayanıklı, kalıcı değil).
//
// Bilinen sınırlar: HOLD sırasında katılan telefon koşuyu donmuş değil
// akar görür (moderatör kısa sürede sürdürür ya da durdurur); SKIP geçmişi
// yeni katılana taşınmaz. STOP/BLACKOUT koşuyu hafızadan da düşürür.

// replayTTLMs: bundan eski koşular yeni katılana gönderilmez. Gösteriler
// saatler sürebilir; telefon biten koşuyu zaten no-op'ladığı için cömert
// bir üst sınır yeterli (unutulmuş dünkü kue ekranda belirmesin diye var).
const replayTTLMs = 3 * 60 * 60 * 1000

type storedRun struct {
	cue        wire.CueStart
	storedAtMs int64
}

// lastRunStore, oda → son kue hafızasıdır. Anahtar yayın kapsamıdır:
// "" = tüm istemcilere yapılmış yayın (Faz 0 konsolu).
type lastRunStore struct {
	mu   sync.Mutex
	runs map[string]storedRun
	// stopped, yakın zamanda durdurulan koşuların mezar taşlarıdır
	// (run_id → durdurulma anı): STOP'tan SONRA gelen kue yinelemeleri
	// (yayın +250/+500 ms'de sürer) koşuyu hafızaya geri yazmasın.
	stopped map[string]int64
}

// stopTombstoneMs: mezar taşı ömrü. Yinelemeler yayından en geç ~500 ms
// sonra biter; 10 sn bol bol yeter ve harita büyümez.
const stopTombstoneMs = 10_000

func newLastRunStore() *lastRunStore {
	return &lastRunStore{runs: map[string]storedRun{}, stopped: map[string]int64{}}
}

// observe, dağıtım sink'inden geçen çerçeveyi gözler. Yalnız JSON kodlaması
// çözülür (her çerçeve iki kodlamayı birden taşır); kue hızı saniyede birkaç
// olduğundan maliyet ihmaldir.
func (s *lastRunStore) observe(room string, f hub.Frame, nowMs int64) {
	msgType, msg, err := wire.DecodeMessage(f.JSON)
	if err != nil {
		return
	}
	switch msgType {
	case wire.TypeCueStart:
		cue := msg.(wire.CueStart)
		s.mu.Lock()
		if at, dead := s.stopped[cue.RunID]; !dead || nowMs-at > stopTombstoneMs {
			s.runs[room] = storedRun{cue: cue, storedAtMs: nowMs}
		}
		s.mu.Unlock()
	case wire.TypeIntervention:
		iv := msg.(wire.Intervention)
		if iv.Kind != "STOP" && iv.Kind != "BLACKOUT" {
			return // HOLD/SKIP koşuyu bitirmez
		}
		s.mu.Lock()
		if iv.RunID != "" {
			s.stopped[iv.RunID] = nowMs
			for id, at := range s.stopped { // bayat taşları düşür
				if nowMs-at > stopTombstoneMs {
					delete(s.stopped, id)
				}
			}
		}
		for key, run := range s.runs {
			// run_id verilmişse yalnız o koşu; verilmemişse müdahalenin
			// kapsamı ("" = tümü, yoksa o oda ve genel yayın) düşürülür.
			if iv.RunID != "" {
				if run.cue.RunID == iv.RunID {
					delete(s.runs, key)
				}
			} else if room == "" || key == room || key == "" {
				delete(s.runs, key)
			}
		}
		s.mu.Unlock()
	}
}

// forRoom, odaya yeni katılan istemciye gönderilecek koşuyu döndürür:
// odanın kendi yayını ile genel ("") yayından YENİ olanı; süresi geçmişse
// hiçbiri. İkinci dönüş değeri bulunup bulunmadığıdır.
func (s *lastRunStore) forRoom(room string, nowMs int64) (wire.CueStart, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	best, ok := s.runs[room]
	if g, gok := s.runs[""]; gok && (!ok || g.storedAtMs > best.storedAtMs) {
		best, ok = g, true
	}
	if !ok || nowMs-best.storedAtMs > replayTTLMs {
		return wire.CueStart{}, false
	}
	return best.cue, true
}

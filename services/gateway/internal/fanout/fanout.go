// Package fanout, yayın çerçevelerinin gateway düğümlerine dağıtım yoludur.
//
// Tek düğümde (Faz 0/1) yol doğrudan yerel hub'dır. Çok düğümde (F2.6) yol
// NATS'tir: kueyi hangi düğüm alırsa alsın çerçeve "tekses.cast" konusuna
// yayımlanır ve HER düğüm (yayımcı dahil — NATS kendi aboneliğine de teslim
// eder) kendi yerel hub'ına verir. Böylece tüm düğümlerde tek tip yol işler.
//
// Neden çekirdek NATS, JetStream değil: kue teli zaten kayba dayanıklıdır
// (aynı run_id 3 kez yinelenir, istemci tekilleştirir); kalıcılık gereksiz
// gecikme ekler. Kalıcı Run izleri de JetStream'e değil, gateway→control-api
// HTTP iç ucuyla Postgres'e yazılır (internal/runsink) — kontrol düzlemi
// hızı düşük, control-api NATS'siz kalır. JetStream ihtiyaç doğarsa
// (yüksek hacimli telefon telemetrisi) yeniden değerlendirilir.
//
// Saat notu: fire_at_server_ms, kueyi alan düğümün saatinde hesaplanır ve
// tüm düğümlerde aynen yayınlanır. Sunucu saati duvar saatine sabitlenmiş
// monoton eksendir (internal/clock); düğümler NTP'liyse (bulut VM'lerde
// chrony varsayılan) eksenler ~1 ms içinde çakışır — 30 ms bütçede ihmal.
package fanout

import (
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"

	"github.com/msaliheroglu/tekses/services/gateway/internal/hub"
)

// Sink, çerçeveyi yerel istemcilere ulaştırır (room boş = herkese).
type Sink func(room string, f hub.Frame)

// Bus, çerçeveyi tüm düğümlerin sink'lerine ulaştırır.
type Bus interface {
	Cast(room string, f hub.Frame) error
	Close()
}

// --- yerel (tek düğüm) ---

type localBus struct{ sink Sink }

// NewLocal, çerçeveyi doğrudan yerel sink'e veren tek düğüm yoludur.
func NewLocal(sink Sink) Bus { return localBus{sink: sink} }

func (b localBus) Cast(room string, f hub.Frame) error {
	b.sink(room, f)
	return nil
}

func (b localBus) Close() {}

// --- NATS (çok düğüm) ---

// Konu tektir; oda gövdede taşınır. Kue hızı düşüktür (saniyede birkaç),
// konu başına ayrıştırma gerekmez.
const castSubject = "tekses.cast"

// castFrame, düğümler arası taşınan gövdedir. []byte alanları JSON'da
// base64'e döner; çerçeveler birkaç yüz bayt olduğundan maliyet ihmaldir.
type castFrame struct {
	Room string `json:"room,omitempty"`
	JSON []byte `json:"json,omitempty"`
	Bin  []byte `json:"bin,omitempty"`
}

type natsBus struct {
	conn *nats.Conn
	sub  *nats.Subscription
	// presSub, SubscribePresence kurulursa dolar (presence.go).
	presSub *nats.Subscription
}

// NewNATS, NATS'e bağlanır ve gelen çerçeveleri sink'e veren aboneliği
// kurar. Bağlantı koparsa istemci sınırsız yeniden dener (kue teli kaybı
// zaten yinelemelerle tolere eder).
func NewNATS(url string, sink Sink) (Bus, error) {
	conn, err := nats.Connect(url,
		nats.Name("tekses-gateway"),
		nats.MaxReconnects(-1),
	)
	if err != nil {
		return nil, fmt.Errorf("fanout: nats bağlantısı: %w", err)
	}
	// Kuyruk grubu YOK: bu bir iş dağıtımı değil fan-out'tur — her düğüm
	// her çerçeveyi almalı.
	sub, err := conn.Subscribe(castSubject, func(m *nats.Msg) {
		var cf castFrame
		if err := json.Unmarshal(m.Data, &cf); err != nil {
			return // bozuk çerçeve; kue yinelemeleri telafi eder
		}
		// Sink kendi goroutine'inde koşar: nats.go bir aboneliğin geri
		// çağrılarını TEK dağıtım goroutine'inde sıralı işler; hub yayını
		// tıkalı bir istemcinin yazma zaman aşımını (5 sn) bekleyebilir ve
		// senkron çağrı, düğüme gelen SONRAKİ tüm çerçeveleri (kue
		// yinelemeleri, başka odalar, HOLD/STOP) baş-blokaja sokar.
		// Çerçeveler arası sıra garantisi kalkar; tel bunu zaten tolere
		// eder (run_id tekilleştirme, mutlak fire_at, müdahaleler saniyeler
		// arayla).
		go sink(cf.Room, hub.Frame{JSON: cf.JSON, Binary: cf.Bin})
	})
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("fanout: nats aboneliği: %w", err)
	}
	return &natsBus{conn: conn, sub: sub}, nil
}

func (b *natsBus) Cast(room string, f hub.Frame) error {
	data, err := json.Marshal(castFrame{Room: room, JSON: f.JSON, Bin: f.Binary})
	if err != nil {
		return err
	}
	return b.conn.Publish(castSubject, data)
}

func (b *natsBus) Close() {
	_ = b.sub.Unsubscribe()
	if b.presSub != nil {
		_ = b.presSub.Unsubscribe()
	}
	b.conn.Close()
}

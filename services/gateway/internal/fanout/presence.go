package fanout

// Küme geneli varlık (presence) yayını: her düğüm periyodik olarak kendi
// istemci sayılarını duyurur; her düğüm diğerlerinin son raporlarını tutar.
// Yalnızca NATS yolu uygular — tek düğümde (localBus) ne goroutine ne yayın
// vardır, /api/v0/presence yerel sayımla döner (sıfır ek maliyet).

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

const presenceSubject = "tekses.presence"

// PresenceReport, bir düğümün anlık istemci sayımıdır. Ts gönderenin duvar
// saatidir (ms) ve YALNIZCA gözlem içindir: bayatlama denetimi alıcının
// yerel monoton saatiyle yapılır (düğümler arası saat kayması sağlıklı bir
// düğümü bayat göstermesin).
type PresenceReport struct {
	NodeID string         `json:"node_id"`
	Total  int            `json:"total"`
	Rooms  map[string]int `json:"rooms,omitempty"`
	Ts     int64          `json:"ts"`
}

// Presence, düğümler arası varlık yayınıdır. Bus gerçeklemesi bunu
// UYGULAYABİLİR (yalnız NATS uygular); çağıran tip iddiasıyla yoklar.
type Presence interface {
	PublishPresence(rep PresenceReport) error
	SubscribePresence(handler func(rep PresenceReport)) error
}

func (b *natsBus) PublishPresence(rep PresenceReport) error {
	data, err := json.Marshal(rep)
	if err != nil {
		return err
	}
	return b.conn.Publish(presenceSubject, data)
}

func (b *natsBus) SubscribePresence(handler func(rep PresenceReport)) error {
	// Kuyruk grubu yok: her düğüm her raporu almalı (fan-out).
	sub, err := b.conn.Subscribe(presenceSubject, func(m *nats.Msg) {
		var rep PresenceReport
		if err := json.Unmarshal(m.Data, &rep); err != nil || rep.NodeID == "" {
			return
		}
		handler(rep)
	})
	if err != nil {
		return err
	}
	b.presSub = sub
	return nil
}

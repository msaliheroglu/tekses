package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"

	"github.com/msaliheroglu/tekses/packages/proto/wire"
	"github.com/msaliheroglu/tekses/services/gateway/internal/fanout"
	"github.com/msaliheroglu/tekses/services/gateway/internal/rooms"
)

// startEmbeddedNATS, testler için rastgele portta gömülü bir NATS sunucusu
// başlatır — dış süreç/ağ gerekmez.
func startEmbeddedNATS(t *testing.T) string {
	t.Helper()
	ns, err := natsserver.NewServer(&natsserver.Options{Port: -1})
	if err != nil {
		t.Fatal(err)
	}
	go ns.Start()
	if !ns.ReadyForConnections(5 * time.Second) {
		t.Fatal("gömülü nats hazır olmadı")
	}
	t.Cleanup(ns.Shutdown)
	return ns.ClientURL()
}

// newNATSNode, ortak NATS'e bağlı bir gateway düğümü kurar.
func newNATSNode(t *testing.T, natsURL string, resolver rooms.Resolver) (*httptest.Server, string) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelError}))
	srv := New(log, "", resolver, nil)
	bus, err := fanout.NewNATS(natsURL, srv.BroadcastSink())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(bus.Close)
	srv.SetBus(bus)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return ts, "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
}

// TestMultiNodeCueFanout: iki düğüm, ortak NATS. Aynı odanın istemcileri
// farklı düğümlerdeyken tek düğüme POST edilen kue İKİSİNE de aynı
// fire_at_server_ms ile ulaşmalı; başka odadaki istemciye ulaşmamalı.
func TestMultiNodeCueFanout(t *testing.T) {
	natsURL := startEmbeddedNATS(t)
	resolver := fakeResolver{codes: map[string]string{"ABC234": "room_a"}}
	node1, ws1 := newNATSNode(t, natsURL, resolver)
	_, ws2 := newNATSNode(t, natsURL, resolver)

	// A → düğüm 1, room_a; B → düğüm 2, room_a; C → düğüm 2, varsayılan oda.
	connA := dial(t, ws1)
	sendMsg(t, connA, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion, JoinCode: "ABC234"})
	_ = readEnvelope(t, connA)
	connB := dial(t, ws2)
	sendMsg(t, connB, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion, JoinCode: "ABC234"})
	_ = readEnvelope(t, connB)
	connC := dial(t, ws2)
	sendMsg(t, connC, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion})
	_ = readEnvelope(t, connC)

	// Kue, yalnızca 1. düğüme POST edilir.
	body, _ := json.Marshal(map[string]any{"delayMs": 600, "room_id": "room_a", "cue_id": "coklu"})
	resp, err := http.Post(node1.URL+"/api/v0/cue", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("kue durumu = %d", resp.StatusCode)
	}

	envA := readEnvelope(t, connA)
	if envA.Type != wire.TypeCueStart {
		t.Fatalf("A'ya gelen tür = %s", envA.Type)
	}
	var cueA wire.CueStart
	if err := json.Unmarshal(envA.Data, &cueA); err != nil {
		t.Fatal(err)
	}

	envB := readEnvelope(t, connB)
	if envB.Type != wire.TypeCueStart {
		t.Fatalf("B'ye (diğer düğüm) gelen tür = %s", envB.Type)
	}
	var cueB wire.CueStart
	if err := json.Unmarshal(envB.Data, &cueB); err != nil {
		t.Fatal(err)
	}

	// Senkronun özü: iki düğümün istemcileri AYNI mutlak ateşleme anını görür.
	if cueA.RunID != cueB.RunID || cueA.FireAtServerMs != cueB.FireAtServerMs {
		t.Fatalf("düğümler arası kue uyuşmuyor: A=%+v B=%+v", cueA, cueB)
	}
	if cueA.CueID != "coklu" {
		t.Fatalf("cue_id = %q", cueA.CueID)
	}

	// C başka odada: kue ona düşmemeli.
	_ = connC.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, _, err := connC.ReadMessage(); err == nil {
		t.Fatal("C başka odanın kuesini aldı")
	}
}

// TestMultiNodeShowActivated: show_activated sinyali de düğümler arası
// dağıtılmalı (2. düğüme POST → 1. düğümdeki istemci almalı).
func TestMultiNodeShowActivated(t *testing.T) {
	natsURL := startEmbeddedNATS(t)
	resolver := fakeResolver{codes: map[string]string{"ABC234": "room_a"}}
	_, ws1 := newNATSNode(t, natsURL, resolver)
	node2, _ := newNATSNode(t, natsURL, resolver)

	connA := dial(t, ws1)
	sendMsg(t, connA, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion, JoinCode: "ABC234"})
	_ = readEnvelope(t, connA)

	body, _ := json.Marshal(map[string]any{"room_id": "room_a", "show_version_id": "sv_9"})
	resp, err := http.Post(node2.URL+"/api/v0/show-activated", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	env := readEnvelope(t, connA)
	if env.Type != wire.TypeShowActivated {
		t.Fatalf("gelen tür = %s, beklenen show_activated", env.Type)
	}
	var msg wire.ShowActivated
	if err := json.Unmarshal(env.Data, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.ShowVersionID != "sv_9" {
		t.Fatalf("beklenmeyen gövde: %+v", msg)
	}
}

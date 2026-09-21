package server

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/msaliheroglu/tekses/packages/proto/wire"
	"github.com/msaliheroglu/tekses/services/gateway/internal/hub"
	"github.com/msaliheroglu/tekses/services/gateway/internal/rooms"
)

// fakeResolver, testlerde katılım kodlarını sabit tablodan çözer.
type fakeResolver struct{ codes map[string]string }

func (f fakeResolver) ResolveJoinCode(_ context.Context, code string) (string, error) {
	room, ok := f.codes[code]
	if !ok {
		return "", rooms.ErrUnknownCode
	}
	return room, nil
}

func newTestServer(t *testing.T, adminToken string) (*httptest.Server, string) {
	t.Helper()
	return newTestServerWithResolver(t, adminToken, nil)
}

func newTestServerWithResolver(t *testing.T, adminToken string, resolver rooms.Resolver) (*httptest.Server, string) {
	t.Helper()
	return newTestServerFull(t, adminToken, resolver, nil)
}

func newTestServerFull(t *testing.T, adminToken string, resolver rooms.Resolver, sessions rooms.SessionValidator) (*httptest.Server, string) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelError}))
	ts := httptest.NewServer(New(log, adminToken, resolver, sessions).Handler())
	t.Cleanup(ts.Close)
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	return ts, wsURL
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) { w.t.Log(string(p)); return len(p), nil }

func dial(t *testing.T, wsURL string) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("ws bağlantısı kurulamadı: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func sendMsg(t *testing.T, conn *websocket.Conn, msgType string, msg any) {
	t.Helper()
	data, err := wire.Encode(msgType, msg)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatal(err)
	}
}

func readEnvelope(t *testing.T, conn *websocket.Conn) wire.Envelope {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, raw, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ws okuma hatası: %v", err)
	}
	env, err := wire.Decode(raw)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func TestStaticPages(t *testing.T) {
	ts, _ := newTestServer(t, "")
	for path, marker := range map[string]string{
		"/":     "moderatör konsolu",
		"/join": "Gösteriye katıl",
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		var body bytes.Buffer
		_, _ = body.ReadFrom(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s durumu = %d, beklenen 200", path, resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
			t.Errorf("%s content-type = %q", path, ct)
		}
		if !strings.Contains(body.String(), marker) {
			t.Errorf("%s sayfasında %q yok", path, marker)
		}
	}
}

func TestHelloWelcome(t *testing.T) {
	_, wsURL := newTestServer(t, "")
	conn := dial(t, wsURL)

	sendMsg(t, conn, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion, ClientKind: "test"})
	env := readEnvelope(t, conn)
	if env.Type != wire.TypeWelcome {
		t.Fatalf("tür = %s, beklenen welcome", env.Type)
	}
	var welcome wire.Welcome
	if err := json.Unmarshal(env.Data, &welcome); err != nil {
		t.Fatal(err)
	}
	if welcome.RoomID != hub.DefaultRoom || welcome.ServerTimeMs == 0 {
		t.Fatalf("beklenmeyen welcome: %+v", welcome)
	}
}

// İkili (protobuf) tel: v2 istemci ikili hello yollar, ikili welcome/saat
// senkronu/kue alır; aynı yayında v1 istemci JSON metin çerçevesi almayı
// sürdürür.
func TestBinaryWireClient(t *testing.T) {
	ts, wsURL := newTestServer(t, "")

	// v2 istemci (ikili çerçeveler)
	binConn := dial(t, wsURL)
	helloBin, err := wire.EncodeBinary(wire.TypeHello, wire.Hello{
		ProtocolVersion: wire.ProtocolVersionBinary, ClientKind: "test-bin"})
	if err != nil {
		t.Fatal(err)
	}
	if err := binConn.WriteMessage(websocket.BinaryMessage, helloBin); err != nil {
		t.Fatal(err)
	}
	readBinary := func() (string, any) {
		t.Helper()
		_ = binConn.SetReadDeadline(time.Now().Add(5 * time.Second))
		mt, raw, err := binConn.ReadMessage()
		if err != nil {
			t.Fatalf("ikili okuma hatası: %v", err)
		}
		if mt != websocket.BinaryMessage {
			t.Fatalf("çerçeve türü = %d, beklenen ikili", mt)
		}
		msgType, msg, err := wire.DecodeBinary(raw)
		if err != nil {
			t.Fatal(err)
		}
		return msgType, msg
	}
	msgType, msg := readBinary()
	welcome, ok := msg.(wire.Welcome)
	if msgType != wire.TypeWelcome || !ok || welcome.RoomID != hub.DefaultRoom {
		t.Fatalf("ikili welcome beklenirken: %s %+v", msgType, msg)
	}

	// İkili saat senkronu değişimi.
	ping, _ := wire.EncodeBinary(wire.TypeClockSyncRequest, wire.ClockSyncRequest{Seq: 3, ClientMonoMs: 777})
	if err := binConn.WriteMessage(websocket.BinaryMessage, ping); err != nil {
		t.Fatal(err)
	}
	msgType, msg = readBinary()
	pong, ok := msg.(wire.ClockSyncResponse)
	if msgType != wire.TypeClockSyncResponse || !ok || pong.Seq != 3 || pong.ClientMonoMs != 777 {
		t.Fatalf("ikili saat yanıtı beklenirken: %s %+v", msgType, msg)
	}

	// v1 (JSON) istemci aynı anda bağlı.
	jsonConn := dial(t, wsURL)
	sendMsg(t, jsonConn, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion})
	_ = readEnvelope(t, jsonConn) // welcome

	// Yayın: iki istemci de kendi kodeğinde aynı kueyi almalı.
	body, _ := json.Marshal(map[string]any{"delayMs": 600, "cue_id": "cift-kodek"})
	resp, err := http.Post(ts.URL+"/api/v0/cue", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	msgType, msg = readBinary()
	binCue, ok := msg.(wire.CueStart)
	if msgType != wire.TypeCueStart || !ok || binCue.CueID != "cift-kodek" {
		t.Fatalf("ikili kue beklenirken: %s %+v", msgType, msg)
	}
	env := readEnvelope(t, jsonConn)
	if env.Type != wire.TypeCueStart {
		t.Fatalf("JSON istemci kue almadı: %s", env.Type)
	}
	var jsonCue wire.CueStart
	if err := json.Unmarshal(env.Data, &jsonCue); err != nil {
		t.Fatal(err)
	}
	if jsonCue.RunID != binCue.RunID || jsonCue.FireAtServerMs != binCue.FireAtServerMs {
		t.Fatalf("iki kodek farklı kue taşıdı: %+v / %+v", jsonCue, binCue)
	}
}

func TestRoomScopedJoinAndCue(t *testing.T) {
	resolver := fakeResolver{codes: map[string]string{"ABC234": "room_a"}}
	ts, wsURL := newTestServerWithResolver(t, "", resolver)

	// İstemci A: kodla room_a'ya girer (kod küçük harfle de yazılabilmeli).
	connA := dial(t, wsURL)
	sendMsg(t, connA, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion, JoinCode: "abc234"})
	envA := readEnvelope(t, connA)
	var welcomeA wire.Welcome
	if err := json.Unmarshal(envA.Data, &welcomeA); err != nil {
		t.Fatal(err)
	}
	if welcomeA.RoomID != "room_a" {
		t.Fatalf("A'nın odası = %q, beklenen room_a", welcomeA.RoomID)
	}

	// İstemci B: kodsuz, varsayılan odada.
	connB := dial(t, wsURL)
	sendMsg(t, connB, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion})
	_ = readEnvelope(t, connB)

	// room_a'ya daraltılmış kue yalnızca A'ya gider.
	body, _ := json.Marshal(map[string]any{"delayMs": 600, "room_id": "room_a"})
	resp, err := http.Post(ts.URL+"/api/v0/cue", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	var cueResp struct {
		Clients int `json:"clients"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cueResp); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if cueResp.Clients != 1 {
		t.Fatalf("hedef istemci = %d, beklenen 1", cueResp.Clients)
	}
	if env := readEnvelope(t, connA); env.Type != wire.TypeCueStart {
		t.Fatalf("A kue almadı: %s", env.Type)
	}
	_ = connB.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, _, err := connB.ReadMessage(); err == nil {
		t.Fatal("B başka odanın kuesini aldı")
	}

	// Geçersiz kodla hello → sunucu bağlantıyı kapatır.
	connC := dial(t, wsURL)
	sendMsg(t, connC, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion, JoinCode: "YANLIS"})
	_ = connC.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, _, err := connC.ReadMessage(); err == nil {
		t.Fatal("geçersiz kod kabul edildi")
	}
}

func TestShowActivatedBroadcast(t *testing.T) {
	resolver := fakeResolver{codes: map[string]string{"ABC234": "room_a"}}
	ts, wsURL := newTestServerWithResolver(t, "", resolver)

	// A odada (v1 JSON), B varsayılan odada — sinyal yalnızca A'ya gitmeli.
	connA := dial(t, wsURL)
	sendMsg(t, connA, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion, JoinCode: "ABC234"})
	_ = readEnvelope(t, connA)
	connB := dial(t, wsURL)
	sendMsg(t, connB, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion})
	_ = readEnvelope(t, connB)

	body, _ := json.Marshal(map[string]any{"room_id": "room_a", "show_version_id": "sv_1"})
	resp, err := http.Post(ts.URL+"/api/v0/show-activated", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("durum = %d, beklenen 200", resp.StatusCode)
	}

	env := readEnvelope(t, connA)
	if env.Type != wire.TypeShowActivated {
		t.Fatalf("A'ya gelen tür = %s, beklenen show_activated", env.Type)
	}
	var msg wire.ShowActivated
	if err := json.Unmarshal(env.Data, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.RoomID != "room_a" || msg.ShowVersionID != "sv_1" {
		t.Fatalf("beklenmeyen gövde: %+v", msg)
	}
	_ = connB.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, _, err := connB.ReadMessage(); err == nil {
		t.Fatal("B başka odanın sinyalini aldı")
	}

	// room_id olmadan istek reddedilir.
	respBad, err := http.Post(ts.URL+"/api/v0/show-activated", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	respBad.Body.Close()
	if respBad.StatusCode != http.StatusBadRequest {
		t.Fatalf("room_id'siz durum = %d, beklenen 400", respBad.StatusCode)
	}
}

func TestClockSyncExchange(t *testing.T) {
	_, wsURL := newTestServer(t, "")
	conn := dial(t, wsURL)

	sendMsg(t, conn, wire.TypeClockSyncRequest, wire.ClockSyncRequest{Seq: 7, ClientMonoMs: 123456})
	env := readEnvelope(t, conn)
	if env.Type != wire.TypeClockSyncResponse {
		t.Fatalf("tür = %s, beklenen clock_sync_response", env.Type)
	}
	var resp wire.ClockSyncResponse
	if err := json.Unmarshal(env.Data, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Seq != 7 || resp.ClientMonoMs != 123456 {
		t.Fatalf("seq/t0 yansıması yanlış: %+v", resp)
	}
	if resp.ServerSendMs < resp.ServerRecvMs {
		t.Fatalf("t2 < t1: %+v", resp)
	}
}

func TestCueBroadcastWithRepeats(t *testing.T) {
	ts, wsURL := newTestServer(t, "")
	conn := dial(t, wsURL)
	sendMsg(t, conn, wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion})
	_ = readEnvelope(t, conn) // welcome

	body, _ := json.Marshal(map[string]any{"delayMs": 600, "torch": true, "flashHz": 2})
	resp, err := http.Post(ts.URL+"/api/v0/cue", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cue POST durumu = %d", resp.StatusCode)
	}
	var cueResp struct {
		RunID          string `json:"run_id"`
		FireAtServerMs int64  `json:"fire_at_server_ms"`
		ServerTimeMs   int64  `json:"server_time_ms"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cueResp); err != nil {
		t.Fatal(err)
	}
	if cueResp.FireAtServerMs <= cueResp.ServerTimeMs {
		t.Fatalf("fire_at gelecekte değil: %+v", cueResp)
	}

	// 3 tekrar, aynı run_id, artan repeat_seq.
	for want := uint32(1); want <= cueRepeats; want++ {
		env := readEnvelope(t, conn)
		if env.Type != wire.TypeCueStart {
			t.Fatalf("tür = %s, beklenen cue_start", env.Type)
		}
		var cue wire.CueStart
		if err := json.Unmarshal(env.Data, &cue); err != nil {
			t.Fatal(err)
		}
		if cue.RunID != cueResp.RunID || cue.RepeatSeq != want {
			t.Fatalf("tekrar %d beklenirken: %+v", want, cue)
		}
		if cue.Payload.FlashHz != 2 || !cue.Payload.Torch {
			t.Fatalf("yük korunmadı: %+v", cue.Payload)
		}
	}
}

func TestCueValidation(t *testing.T) {
	ts, _ := newTestServer(t, "")
	for name, body := range map[string]string{
		"yüksek flashHz": `{"flashHz": 8}`,
		"kısa gecikme":   `{"delayMs": 100}`,
		"bozuk renk":     `{"color": "kirmizi"}`,
	} {
		resp, err := http.Post(ts.URL+"/api/v0/cue", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: durum = %d, beklenen 400", name, resp.StatusCode)
		}
	}
}

func TestInterventionBroadcast(t *testing.T) {
	ts, wsURL := newTestServer(t, "")
	conn := dial(t, wsURL)

	body := strings.NewReader(`{"kind":"BLACKOUT","run_id":"r1"}`)
	resp, err := http.Post(ts.URL+"/api/v0/intervention", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("intervention POST durumu = %d", resp.StatusCode)
	}

	env := readEnvelope(t, conn)
	if env.Type != wire.TypeIntervention {
		t.Fatalf("tür = %s, beklenen intervention", env.Type)
	}
	var iv wire.Intervention
	if err := json.Unmarshal(env.Data, &iv); err != nil {
		t.Fatal(err)
	}
	if iv.Kind != "BLACKOUT" || iv.RunID != "r1" {
		t.Fatalf("beklenmeyen müdahale: %+v", iv)
	}

	// Geçersiz tür reddedilir.
	resp2, err := http.Post(ts.URL+"/api/v0/intervention", "application/json", strings.NewReader(`{"kind":"PANIC"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("geçersiz kind durumu = %d, beklenen 400", resp2.StatusCode)
	}
}

func TestAdminTokenRequired(t *testing.T) {
	ts, _ := newTestServer(t, "gizli")

	resp, err := http.Post(ts.URL+"/api/v0/cue", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("token'sız istek durumu = %d, beklenen 401", resp.StatusCode)
	}

	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v0/cue", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer gizli")
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("token'lı istek durumu = %d, beklenen 200", resp2.StatusCode)
	}
}

// fakeSessions: sabit token listesini geçerli oturum sayar ve çağrıları sayar.
type fakeSessions struct {
	valid map[string]bool
	calls int
}

func (f *fakeSessions) ValidateSession(_ context.Context, token string) error {
	f.calls++
	if f.valid[token] {
		return nil
	}
	return rooms.ErrInvalidSession
}

func TestAdminAcceptsPanelSession(t *testing.T) {
	sessions := &fakeSessions{valid: map[string]bool{"panel-oturumu": true}}
	ts, _ := newTestServerFull(t, "gizli", nil, sessions)

	post := func(token string) int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/v0/cue", strings.NewReader(`{}`))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	if got := post("panel-oturumu"); got != http.StatusOK {
		t.Fatalf("panel oturumu ile durum = %d, beklenen 200", got)
	}
	if got := post("sahte-oturum"); got != http.StatusUnauthorized {
		t.Fatalf("geçersiz oturum ile durum = %d, beklenen 401", got)
	}
	// Önbellek: aynı geçerli token ikinci kez control-api'ye sorulmaz.
	before := sessions.calls
	if got := post("panel-oturumu"); got != http.StatusOK {
		t.Fatalf("önbellekli oturum ile durum = %d, beklenen 200", got)
	}
	if sessions.calls != before {
		t.Fatalf("önbelleğe rağmen doğrulayıcı yeniden çağrıldı (%d → %d)", before, sessions.calls)
	}
}

// Telemetri uçları işletmen token'ı İSTER: panel oturumu (checkAdmin'in
// kabul ettiği) yetmez — kiracılar arası veri sızmasın.
func TestTelemetryRequiresOperatorToken(t *testing.T) {
	sessions := &fakeSessions{valid: map[string]bool{"panel-oturumu": true}}
	ts, _ := newTestServerFull(t, "gizli", nil, sessions)

	get := func(path, token string) int {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, ts.URL+path, nil)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		return resp.StatusCode
	}

	for _, path := range []string{"/api/v0/presence", "/api/v0/clockstats"} {
		if got := get(path, "gizli"); got != http.StatusOK {
			t.Fatalf("%s işletmen token'ı ile durum = %d, beklenen 200", path, got)
		}
		if got := get(path, "panel-oturumu"); got != http.StatusUnauthorized {
			t.Fatalf("%s panel oturumu ile durum = %d, beklenen 401 (kiracı sızıntısı)", path, got)
		}
		if got := get(path, ""); got != http.StatusUnauthorized {
			t.Fatalf("%s token'sız durum = %d, beklenen 401", path, got)
		}
	}
}

func TestRunsRecorded(t *testing.T) {
	ts, _ := newTestServer(t, "")

	body, _ := json.Marshal(map[string]any{"delayMs": 600, "cue_id": "program"})
	resp, err := http.Post(ts.URL+"/api/v0/cue", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	resp2, err := http.Post(ts.URL+"/api/v0/intervention", "application/json",
		strings.NewReader(`{"kind":"STOP"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()

	runsResp, err := http.Get(ts.URL + "/api/v0/runs")
	if err != nil {
		t.Fatal(err)
	}
	defer runsResp.Body.Close()
	var got struct {
		Runs []struct {
			Kind  string `json:"kind"`
			CueID string `json:"cue_id"`
		} `json:"runs"`
	}
	if err := json.NewDecoder(runsResp.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	// En yenisi başta: STOP, sonra cue.
	if len(got.Runs) != 2 || got.Runs[0].Kind != "STOP" || got.Runs[1].Kind != "cue" || got.Runs[1].CueID != "program" {
		t.Fatalf("beklenmeyen runs: %+v", got.Runs)
	}
}

func TestRunsRequiresToken(t *testing.T) {
	ts, _ := newTestServer(t, "gizli")
	resp, err := http.Get(ts.URL + "/api/v0/runs")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("token'sız runs durumu = %d, beklenen 401", resp.StatusCode)
	}
}

func TestContentTypeRequired(t *testing.T) {
	// CSRF önlemi: application/json olmayan gövdeler (örn. çapraz-site
	// form POST'unun text/plain'i) API uçlarında reddedilir.
	ts, _ := newTestServer(t, "")
	resp, err := http.Post(ts.URL+"/api/v0/cue", "text/plain", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("text/plain istek durumu = %d, beklenen 415", resp.StatusCode)
	}
}

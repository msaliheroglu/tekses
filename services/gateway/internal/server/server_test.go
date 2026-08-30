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

	"github.com/gorilla/websocket"

	"github.com/msaliheroglu/tekses/packages/proto/wire"
)

func newTestServer(t *testing.T, adminToken string) (*httptest.Server, string) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelError}))
	ts := httptest.NewServer(New(log, adminToken).Handler())
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
	if welcome.RoomID != roomID || welcome.ServerTimeMs == 0 {
		t.Fatalf("beklenmeyen welcome: %+v", welcome)
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

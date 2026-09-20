package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/msaliheroglu/tekses/packages/blob"
	"github.com/msaliheroglu/tekses/services/control-api/internal/store/memstore"
)

type client struct {
	t     *testing.T
	base  string
	token string
}

func newTestAPI(t *testing.T) *client {
	t.Helper()
	return newTestAPIWithTranscriber(t, "")
}

func newTestAPIWithTranscriber(t *testing.T, transcriber string) *client {
	t.Helper()
	log := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelError}))
	packages, err := blob.NewFS(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(New(log, memstore.New(), packages, transcriber, "ic-sir").Handler())
	t.Cleanup(ts.Close)
	return &client{t: t, base: ts.URL}
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) { w.t.Log(string(p)); return len(p), nil }

// do, JSON istek atar; yanıt gövdesini out'a (nil değilse) çözer ve durum
// kodunu döndürür.
func (c *client) do(method, path string, body any, out any) int {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			c.t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, c.base+path, &buf)
	if err != nil {
		c.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			c.t.Fatalf("%s %s yanıtı çözülemedi: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

func (c *client) register(org, email string) {
	c.t.Helper()
	var resp struct {
		Token string `json:"token"`
	}
	status := c.do(http.MethodPost, "/api/v1/auth/register",
		map[string]string{"organization": org, "email": email, "password": "cok-gizli-1"}, &resp)
	if status != http.StatusCreated || resp.Token == "" {
		c.t.Fatalf("register durumu = %d, token = %q", status, resp.Token)
	}
	c.token = resp.Token
}

func TestRegisterLoginFlow(t *testing.T) {
	c := newTestAPI(t)
	c.register("Deneme Org", "mod@ornek.com")

	// Aynı e-posta ikinci kez kaydolamaz.
	if status := c.do(http.MethodPost, "/api/v1/auth/register",
		map[string]string{"organization": "X", "email": "MOD@ornek.com", "password": "cok-gizli-1"}, nil); status != http.StatusConflict {
		t.Fatalf("çift kayıt durumu = %d, beklenen 409", status)
	}

	// Yanlış şifre reddedilir.
	if status := c.do(http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": "mod@ornek.com", "password": "yanlis-sifre"}, nil); status != http.StatusUnauthorized {
		t.Fatalf("yanlış şifre durumu = %d, beklenen 401", status)
	}

	// Doğru şifreyle (büyük harfli e-postayla da) girilir.
	var login struct {
		Token string `json:"token"`
	}
	if status := c.do(http.MethodPost, "/api/v1/auth/login",
		map[string]string{"email": "Mod@Ornek.com", "password": "cok-gizli-1"}, &login); status != http.StatusOK || login.Token == "" {
		t.Fatalf("login durumu = %d", status)
	}
}

func TestEventRoomShowCRUD(t *testing.T) {
	c := newTestAPI(t)
	c.register("Stadyum AŞ", "mod@stadyum.com")

	var event struct {
		ID string `json:"id"`
	}
	if status := c.do(http.MethodPost, "/api/v1/events",
		map[string]string{"name": "Final Maçı", "venue": "Atatürk Olimpiyat"}, &event); status != http.StatusCreated {
		t.Fatalf("etkinlik oluşturma durumu = %d", status)
	}

	var room struct {
		ID       string `json:"id"`
		JoinCode string `json:"join_code"`
	}
	if status := c.do(http.MethodPost, "/api/v1/events/"+event.ID+"/rooms",
		map[string]string{"name": "Kuzey Tribünü"}, &room); status != http.StatusCreated {
		t.Fatalf("oda oluşturma durumu = %d", status)
	}
	if len(room.JoinCode) != joinCodeLen {
		t.Fatalf("join_code uzunluğu = %d, beklenen %d", len(room.JoinCode), joinCodeLen)
	}
	for _, ch := range room.JoinCode {
		if !strings.ContainsRune(joinCodeAlphabet, ch) {
			t.Fatalf("join_code alfabede olmayan karakter içeriyor: %q", room.JoinCode)
		}
	}

	var rooms struct {
		Rooms []json.RawMessage `json:"rooms"`
	}
	if status := c.do(http.MethodGet, "/api/v1/events/"+event.ID+"/rooms", nil, &rooms); status != http.StatusOK || len(rooms.Rooms) != 1 {
		t.Fatalf("oda listesi durumu = %d, adet = %d", status, len(rooms.Rooms))
	}

	if status := c.do(http.MethodPost, "/api/v1/shows", map[string]string{"title": "Marş Seti"}, nil); status != http.StatusCreated {
		t.Fatalf("gösteri oluşturma durumu = %d", status)
	}
}

func TestTenantIsolation(t *testing.T) {
	c := newTestAPI(t)
	c.register("Org A", "a@ornek.com")
	var event struct {
		ID string `json:"id"`
	}
	if status := c.do(http.MethodPost, "/api/v1/events", map[string]string{"name": "A Etkinliği"}, &event); status != http.StatusCreated {
		t.Fatal("A etkinliği oluşturulamadı")
	}

	// B kiracısı A'nın etkinliğini göremez, listesinde de bulamaz.
	c.register("Org B", "b@ornek.com")
	if status := c.do(http.MethodGet, "/api/v1/events/"+event.ID, nil, nil); status != http.StatusNotFound {
		t.Fatalf("çapraz kiracı erişim durumu = %d, beklenen 404", status)
	}
	var list struct {
		Events []json.RawMessage `json:"events"`
	}
	if status := c.do(http.MethodGet, "/api/v1/events", nil, &list); status != http.StatusOK || len(list.Events) != 0 {
		t.Fatalf("B'nin etkinlik listesi boş değil: durum %d, adet %d", status, len(list.Events))
	}
}

const testManifest = `{
  "title": "Marş Seti",
  "sequences": [{
    "id": "seq-1", "title": "Açılış", "duration_ms": 60000,
    "lyric_lines": [{"at_ms": 0, "duration_ms": 4000, "text": "Hep beraber!"}],
    "cue_lanes": [{"id": "ekran", "kind": "screen", "cues": [
      {"at_ms": 0, "duration_ms": 4000, "color": "#FF2A2A", "flash_hz": 2}
    ]}]
  }]
}`

func TestPublishActivateJoinFlow(t *testing.T) {
	c := newTestAPI(t)
	c.register("Stadyum AŞ", "mod@stadyum.com")

	var event struct {
		ID string `json:"id"`
	}
	c.do(http.MethodPost, "/api/v1/events", map[string]string{"name": "Final"}, &event)
	var room struct {
		ID       string `json:"id"`
		JoinCode string `json:"join_code"`
	}
	c.do(http.MethodPost, "/api/v1/events/"+event.ID+"/rooms", map[string]string{"name": "Tribün"}, &room)
	var show struct {
		ID string `json:"id"`
	}
	c.do(http.MethodPost, "/api/v1/shows", map[string]string{"title": "Marş Seti"}, &show)

	// Yayınlama: sürüm 1, sonra sürüm 2; aynı içerik aynı özeti verir.
	var v1, v2 struct {
		ID      string `json:"id"`
		Version int    `json:"version"`
		SHA256  string `json:"sha256"`
	}
	if status := c.do(http.MethodPost, "/api/v1/shows/"+show.ID+"/versions", json.RawMessage(testManifest), &v1); status != http.StatusCreated {
		t.Fatalf("yayınlama durumu = %d", status)
	}
	if status := c.do(http.MethodPost, "/api/v1/shows/"+show.ID+"/versions", json.RawMessage(testManifest), &v2); status != http.StatusCreated {
		t.Fatalf("ikinci yayınlama durumu = %d", status)
	}
	if v1.Version != 1 || v2.Version != 2 {
		t.Fatalf("sürüm numaraları = %d, %d; beklenen 1, 2", v1.Version, v2.Version)
	}
	if v1.SHA256 != v2.SHA256 || len(v1.SHA256) != 64 {
		t.Fatalf("özetler tutarsız: %s / %s", v1.SHA256, v2.SHA256)
	}

	// Geçersiz manifest reddedilir.
	bad := `{"title":"X","sequences":[{"id":"a","title":"t","duration_ms":1000,
	  "cue_lanes":[{"id":"l","kind":"screen","cues":[{"at_ms":0,"color":"#FFFFFF","flash_hz":9}]}]}]}`
	if status := c.do(http.MethodPost, "/api/v1/shows/"+show.ID+"/versions", json.RawMessage(bad), nil); status != http.StatusBadRequest {
		t.Fatalf("geçersiz manifest durumu = %d, beklenen 400", status)
	}

	// Etkinleştir ve kodla katıl (kimliksiz).
	if status := c.do(http.MethodPost, "/api/v1/rooms/"+room.ID+"/activate",
		map[string]string{"show_version_id": v2.ID}, nil); status != http.StatusOK {
		t.Fatalf("etkinleştirme durumu = %d", status)
	}
	anon := &client{t: t, base: c.base} // token yok
	var join struct {
		RoomID      string `json:"room_id"`
		EventName   string `json:"event_name"`
		ShowVersion struct {
			ID          string          `json:"id"`
			SHA256      string          `json:"sha256"`
			ManifestURL string          `json:"manifest_url"`
			Manifest    json.RawMessage `json:"manifest"`
		} `json:"show_version"`
	}
	if status := anon.do(http.MethodGet, "/api/v1/join/"+room.JoinCode, nil, &join); status != http.StatusOK {
		t.Fatalf("katılım durumu = %d", status)
	}
	if join.RoomID != room.ID || join.ShowVersion.ID != v2.ID || len(join.ShowVersion.Manifest) == 0 {
		t.Fatalf("katılım yanıtı eksik: %+v", join)
	}

	// Paket indirme sözleşmesi: manifest_url'den inen baytların SHA-256'sı
	// join yanıtındaki özetle birebir tutmalı (telefonun doğrulama yolu).
	pkgResp, err := http.Get(c.base + join.ShowVersion.ManifestURL)
	if err != nil {
		t.Fatal(err)
	}
	pkgBytes, _ := io.ReadAll(pkgResp.Body)
	pkgResp.Body.Close()
	if pkgResp.StatusCode != http.StatusOK {
		t.Fatalf("paket indirme durumu = %d", pkgResp.StatusCode)
	}
	if cc := pkgResp.Header.Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Fatalf("paket immutable önbellek başlığı taşımıyor: %q", cc)
	}
	sum := sha256.Sum256(pkgBytes)
	if hex.EncodeToString(sum[:]) != join.ShowVersion.SHA256 {
		t.Fatal("indirilen paketin özeti join yanıtındaki sha256 ile tutmuyor")
	}

	// Olmayan/bozuk paket adları 404.
	for _, bad := range []string{"/packages/kotu.json", "/packages/" + strings.Repeat("0", 64) + ".json"} {
		resp, err := http.Get(c.base + bad)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("%s durumu = %d, beklenen 404", bad, resp.StatusCode)
		}
	}

	// Bilinmeyen kod 404.
	if status := anon.do(http.MethodGet, "/api/v1/join/YOKKOD", nil, nil); status != http.StatusNotFound {
		t.Fatalf("bilinmeyen kod durumu = %d, beklenen 404", status)
	}

	// Başka kiracının sürümü odada etkinleştirilemez.
	other := &client{t: t, base: c.base}
	other.register("Org B", "b@ornek.com")
	var otherShow struct {
		ID string `json:"id"`
	}
	other.do(http.MethodPost, "/api/v1/shows", map[string]string{"title": "B Gösterisi"}, &otherShow)
	var otherV struct {
		ID string `json:"id"`
	}
	other.do(http.MethodPost, "/api/v1/shows/"+otherShow.ID+"/versions", json.RawMessage(testManifest), &otherV)
	if status := c.do(http.MethodPost, "/api/v1/rooms/"+room.ID+"/activate",
		map[string]string{"show_version_id": otherV.ID}, nil); status != http.StatusNotFound {
		t.Fatalf("çapraz kiracı etkinleştirme durumu = %d, beklenen 404", status)
	}
}

func TestAudioAssetFlow(t *testing.T) {
	c := newTestAPI(t)
	c.register("Ses AŞ", "ses@ornek.com")

	// Yükleme: ham ses gövdesi → içerik adresli asset_id.
	fakeMp3 := []byte("ID3-sahte-mp3-govdesi-test")
	req, _ := http.NewRequest(http.MethodPost, c.base+"/api/v1/assets", bytes.NewReader(fakeMp3))
	req.Header.Set("Content-Type", "audio/mpeg")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var up struct {
		AssetID string `json:"asset_id"`
		URL     string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&up); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated || !strings.HasSuffix(up.AssetID, ".mp3") {
		t.Fatalf("yükleme durumu = %d, asset_id = %q", resp.StatusCode, up.AssetID)
	}
	wantSum := sha256.Sum256(fakeMp3)
	if up.AssetID != hex.EncodeToString(wantSum[:])+".mp3" {
		t.Fatalf("asset_id içerik adresli değil: %s", up.AssetID)
	}

	// Herkese açık indirme, bayt bayt aynı ve immutable.
	dl, err := http.Get(c.base + up.URL)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(dl.Body)
	dl.Body.Close()
	if dl.StatusCode != http.StatusOK || string(body) != string(fakeMp3) {
		t.Fatalf("indirme durumu = %d, gövde eşleşmiyor", dl.StatusCode)
	}
	if ct := dl.Header.Get("Content-Type"); ct != "audio/mpeg" {
		t.Fatalf("içerik türü = %q", ct)
	}

	// Ses türü olmayan yükleme reddedilir.
	req2, _ := http.NewRequest(http.MethodPost, c.base+"/api/v1/assets", strings.NewReader("x"))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+c.token)
	resp2, _ := http.DefaultClient.Do(req2)
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("json yükleme durumu = %d, beklenen 415", resp2.StatusCode)
	}

	// Yayın doğrulaması: var olan varlıkla geçer, olmayanla 400.
	var show struct {
		ID string `json:"id"`
	}
	c.do(http.MethodPost, "/api/v1/shows", map[string]string{"title": "Sesli"}, &show)
	manifestWith := func(assetID string) string {
		return `{"title":"X","sequences":[{"id":"a","title":"t","duration_ms":10000,
		  "cue_lanes":[{"id":"ses","kind":"audio","cues":[{"at_ms":0,"duration_ms":5000,"asset_id":"` + assetID + `"}]}]}]}`
	}
	if status := c.do(http.MethodPost, "/api/v1/shows/"+show.ID+"/versions",
		json.RawMessage(manifestWith(up.AssetID)), nil); status != http.StatusCreated {
		t.Fatalf("var olan varlıkla yayın durumu = %d", status)
	}
	missing := strings.Repeat("0", 64) + ".mp3"
	if status := c.do(http.MethodPost, "/api/v1/shows/"+show.ID+"/versions",
		json.RawMessage(manifestWith(missing)), nil); status != http.StatusBadRequest {
		t.Fatalf("olmayan varlıkla yayın durumu = %d, beklenen 400", status)
	}
	if status := c.do(http.MethodPost, "/api/v1/shows/"+show.ID+"/versions",
		json.RawMessage(manifestWith("serbest-metin")), nil); status != http.StatusBadRequest {
		t.Fatalf("biçimsiz asset_id ile yayın durumu = %d, beklenen 400", status)
	}
}

func TestTranscriptionFlow(t *testing.T) {
	// Sahte çözümleyici: sözleşmeye uygun sabit JSON basar (gerçek Whisper
	// entegrasyonu deploy/transcribe-whisper.sh ile VM'de kurulur).
	stub := t.TempDir() + "/stub-transcriber.sh"
	if err := os.WriteFile(stub, []byte(`#!/bin/sh
echo '{"segments":[{"start_ms":4000,"end_ms":8000,"text":"Nakarat"},{"start_ms":1200,"end_ms":4000,"text":" İlk satır "},{"start_ms":9000,"end_ms":9500,"text":"  "},{"start_ms":10000,"end_ms":30000,"text":"[MÜZİK ÇALIYOR]"},{"start_ms":31000,"end_ms":32000,"text":"(alkış)"},{"start_ms":33000,"end_ms":34000,"text":"♪ ♪"}]}'
`), 0o755); err != nil {
		t.Fatal(err)
	}

	c := newTestAPIWithTranscriber(t, stub)
	c.register("Karaoke AŞ", "kr@ornek.com")

	// Varlık yükle.
	req, _ := http.NewRequest(http.MethodPost, c.base+"/api/v1/assets", bytes.NewReader([]byte("sahte-ses")))
	req.Header.Set("Content-Type", "audio/mpeg")
	req.Header.Set("Authorization", "Bearer "+c.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var up struct {
		AssetID string `json:"asset_id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&up)
	resp.Body.Close()

	// Çözümleme başlat ve bitene dek yokla.
	var start struct {
		TranscriptionID string `json:"transcription_id"`
	}
	if status := c.do(http.MethodPost, "/api/v1/assets/"+up.AssetID+"/transcribe", map[string]any{}, &start); status != http.StatusAccepted {
		t.Fatalf("başlatma durumu = %d", status)
	}
	var result struct {
		Status     string `json:"status"`
		LyricLines []struct {
			AtMs       int    `json:"at_ms"`
			DurationMs int    `json:"duration_ms"`
			Text       string `json:"text"`
		} `json:"lyric_lines"`
		Error string `json:"error"`
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		c.do(http.MethodGet, "/api/v1/transcriptions/"+start.TranscriptionID, nil, &result)
		if result.Status != "running" || time.Now().After(deadline) {
			break
		}
		time.Sleep(30 * time.Millisecond)
	}
	if result.Status != "done" {
		t.Fatalf("iş durumu = %s (%s)", result.Status, result.Error)
	}
	// Sıralanmış, kırpılmış, boş satırlar atılmış.
	if len(result.LyricLines) != 2 ||
		result.LyricLines[0].Text != "İlk satır" || result.LyricLines[0].AtMs != 1200 ||
		result.LyricLines[1].Text != "Nakarat" || result.LyricLines[1].DurationMs != 4000 {
		t.Fatalf("beklenmeyen satırlar: %+v", result.LyricLines)
	}

	// Başka kiracı işi göremez.
	other := &client{t: t, base: c.base}
	other.register("B", "b2@ornek.com")
	if status := other.do(http.MethodGet, "/api/v1/transcriptions/"+start.TranscriptionID, nil, nil); status != http.StatusNotFound {
		t.Fatalf("çapraz kiracı iş erişimi = %d, beklenen 404", status)
	}

	// Yapılandırılmamış sunucuda 501.
	c2 := newTestAPI(t)
	c2.register("X", "x@ornek.com")
	if status := c2.do(http.MethodPost, "/api/v1/assets/"+strings.Repeat("0", 64)+".mp3/transcribe", map[string]any{}, nil); status != http.StatusNotImplemented {
		t.Fatalf("yapılandırılmamış durum = %d, beklenen 501", status)
	}
}

func TestAuthRequired(t *testing.T) {
	c := newTestAPI(t)
	for _, path := range []string{"/api/v1/events", "/api/v1/shows"} {
		if status := c.do(http.MethodGet, path, nil, nil); status != http.StatusUnauthorized {
			t.Errorf("token'sız GET %s durumu = %d, beklenen 401", path, status)
		}
	}

	// Content-Type'sız POST 415 döner (fmt importunu da kullanır).
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/api/v1/auth/login", c.base), strings.NewReader("{}"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Fatalf("Content-Type'sız POST durumu = %d, beklenen 415", resp.StatusCode)
	}
}

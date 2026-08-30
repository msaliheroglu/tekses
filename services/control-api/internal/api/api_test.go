package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/msaliheroglu/tekses/services/control-api/internal/store/memstore"
)

type client struct {
	t     *testing.T
	base  string
	token string
}

func newTestAPI(t *testing.T) *client {
	t.Helper()
	log := slog.New(slog.NewTextHandler(testWriter{t}, &slog.HandlerOptions{Level: slog.LevelError}))
	ts := httptest.NewServer(New(log, memstore.New()).Handler())
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
			ID       string          `json:"id"`
			SHA256   string          `json:"sha256"`
			Manifest json.RawMessage `json:"manifest"`
		} `json:"show_version"`
	}
	if status := anon.do(http.MethodGet, "/api/v1/join/"+room.JoinCode, nil, &join); status != http.StatusOK {
		t.Fatalf("katılım durumu = %d", status)
	}
	if join.RoomID != room.ID || join.ShowVersion.ID != v2.ID || len(join.ShowVersion.Manifest) == 0 {
		t.Fatalf("katılım yanıtı eksik: %+v", join)
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

package manifest

import (
	"encoding/json"
	"flag"
	"math"
	"os"
	"strings"
	"testing"
)

// Mekân altın vektörleri: Go ve Dart aynı koltuğu AYNI konuma çözmelidir.
// Dosya testdata/venue_vectors.json; Dart testi aynı dosyayı okuyup kendi
// çözücüsünü doğrular (apps/participant/test/venue_test.dart) — tel altın
// çerçeveleriyle aynı çapraz doğrulama deseni. Şema/örnek değişince:
// go test ./packages/manifest -run TestVenueGolden -update
var updateVenue = flag.Bool("update", false, "mekân altın vektör dosyasını yeniden üret")

const venueVectorsPath = "testdata/venue_vectors.json"

// Üç blok bilinçli çeşitliliktedir: eksen hizalı + eğimli tribün (KUZEY),
// 90° dönük + tireli kimlik (DOGU-ALT), tek sıralı sıfır row_vec'li (SAHNE).
func goldenVenue() Venue {
	return Venue{
		Name: "Test Stadyumu",
		Blocks: []Block{
			{
				ID: "KUZEY", Rows: 20, SeatsPerRow: 40,
				Origin:  Vec3{0, 0, 0},
				SeatVec: Vec3{0.5, 0, 0},
				RowVec:  Vec3{0, 0.8, 0.4},
			},
			{
				ID: "DOGU-ALT", Rows: 10, SeatsPerRow: 30,
				Origin:  Vec3{25, 5, 0},
				SeatVec: Vec3{0, 0.5, 0},
				RowVec:  Vec3{0.8, 0, 0.4},
			},
			{
				ID: "SAHNE", Rows: 1, SeatsPerRow: 5,
				Origin:  Vec3{10, -3, 1},
				SeatVec: Vec3{0.6, 0, 0},
			},
		},
	}
}

var goldenSeats = []string{
	"KUZEY-1-1", "KUZEY-3-5", "KUZEY-20-40",
	"DOGU-ALT-1-1", "DOGU-ALT-10-30",
	"SAHNE-1-1", "SAHNE-1-5",
}

type venueVectors struct {
	Comment string        `json:"comment"`
	Venue   Venue         `json:"venue"`
	Cases   []venueVector `json:"cases"`
}

type venueVector struct {
	Seat string  `json:"seat"`
	X    float64 `json:"x"`
	Y    float64 `json:"y"`
	Z    float64 `json:"z"`
	U    float64 `json:"u"`
	V    float64 `json:"v"`
	W    float64 `json:"w"`
}

func TestVenueGolden(t *testing.T) {
	if *updateVenue {
		v := goldenVenue()
		out := venueVectors{
			Comment: "üretici: go test ./packages/manifest -run TestVenueGolden -update",
			Venue:   v,
		}
		for _, s := range goldenSeats {
			ref, err := ParseSeatRef(s)
			if err != nil {
				t.Fatal(err)
			}
			pos, err := v.Resolve(ref)
			if err != nil {
				t.Fatal(err)
			}
			out.Cases = append(out.Cases, venueVector{
				Seat: s, X: pos.X, Y: pos.Y, Z: pos.Z, U: pos.U, V: pos.V, W: pos.W,
			})
		}
		data, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(venueVectorsPath, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(venueVectorsPath)
	if err != nil {
		t.Fatalf("altın vektör dosyası okunamadı (önce -update ile üret): %v", err)
	}
	var in venueVectors
	if err := json.Unmarshal(data, &in); err != nil {
		t.Fatal(err)
	}
	if len(in.Cases) == 0 {
		t.Fatal("altın vektör dosyası boş")
	}
	for _, c := range in.Cases {
		ref, err := ParseSeatRef(c.Seat)
		if err != nil {
			t.Fatal(err)
		}
		pos, err := in.Venue.Resolve(ref)
		if err != nil {
			t.Fatalf("%s: %v", c.Seat, err)
		}
		got := [6]float64{pos.X, pos.Y, pos.Z, pos.U, pos.V, pos.W}
		want := [6]float64{c.X, c.Y, c.Z, c.U, c.V, c.W}
		for i := range got {
			if math.Abs(got[i]-want[i]) > 1e-12 {
				t.Errorf("%s: bileşen %d = %v, altın %v", c.Seat, i, got[i], want[i])
			}
		}
	}
}

// El hesabı doğrulaması — altın dosyadan bağımsız (dosya Go'nun kendi
// çıktısından üretildiği için tek başına doğruluk kanıtı değildir).
func TestSeatResolveHand(t *testing.T) {
	v := goldenVenue()
	// Sınır kutusu: min(0,-3,0), max(32.2,19.5,7.6) — blok köşelerinden.
	cases := []struct {
		seat string
		want SeatPos
	}{
		{"KUZEY-1-1", SeatPos{X: 0, Y: 0, Z: 0, U: 0, V: 3 / 22.5, W: 0}},
		// origin + 2·row_vec + 4·seat_vec = (2, 1.6, 0.8)
		{"KUZEY-3-5", SeatPos{X: 2, Y: 1.6, Z: 0.8, U: 2 / 32.2, V: 4.6 / 22.5, W: 0.8 / 7.6}},
		// (25,5,0) + 9·(0.8,0,0.4) + 29·(0,0.5,0) = (32.2, 19.5, 3.6) — mekânın uç köşesi
		{"DOGU-ALT-10-30", SeatPos{X: 32.2, Y: 19.5, Z: 3.6, U: 1, V: 1, W: 3.6 / 7.6}},
		{"SAHNE-1-5", SeatPos{X: 12.4, Y: -3, Z: 1, U: 12.4 / 32.2, V: 0, W: 1 / 7.6}},
	}
	for _, c := range cases {
		ref, err := ParseSeatRef(c.seat)
		if err != nil {
			t.Fatal(err)
		}
		got, err := v.Resolve(ref)
		if err != nil {
			t.Fatalf("%s: %v", c.seat, err)
		}
		approx := func(a, b float64) bool { return math.Abs(a-b) < 1e-9 }
		if !approx(got.X, c.want.X) || !approx(got.Y, c.want.Y) || !approx(got.Z, c.want.Z) ||
			!approx(got.U, c.want.U) || !approx(got.V, c.want.V) || !approx(got.W, c.want.W) {
			t.Errorf("%s = %+v, beklenen %+v", c.seat, got, c.want)
		}
	}
}

func TestSeatResolveErrors(t *testing.T) {
	v := goldenVenue()
	for _, seat := range []string{"BATI-1-1", "KUZEY-21-1", "KUZEY-1-41", "KUZEY-0-1"} {
		ref, err := ParseSeatRef(seat)
		if err != nil {
			continue // ParseSeatRef zaten reddetti (ör. sıra 0)
		}
		if _, err := v.Resolve(ref); err == nil {
			t.Errorf("%s: hata bekleniyordu", seat)
		}
	}
}

func TestParseSeatRef(t *testing.T) {
	good := []struct {
		in   string
		want SeatRef
	}{
		{"A-12-5", SeatRef{"A", 12, 5}},
		{"DOGU-ALT-3-7", SeatRef{"DOGU-ALT", 3, 7}}, // tireli blok: son iki parça sıra/koltuk
		{"5-1-1", SeatRef{"5", 1, 1}},
	}
	for _, c := range good {
		got, err := ParseSeatRef(c.in)
		if err != nil {
			t.Fatalf("%s: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("%s = %+v, beklenen %+v", c.in, got, c.want)
		}
		if got.String() != c.in {
			t.Errorf("%s gidiş-dönüşü bozuk: %s", c.in, got.String())
		}
	}
	for _, bad := range []string{"", "A-1", "A-0-1", "A-1-0", "A-x-1", "A-1-y", "-1-2"} {
		if _, err := ParseSeatRef(bad); err == nil {
			t.Errorf("%q: hata bekleniyordu", bad)
		}
	}
}

func venueManifestJSON(venuePart string) []byte {
	return []byte(`{
		"title": "Deneme",
		"sequences": [{"id": "s1", "title": "S1", "duration_ms": 1000}],
		"venue": ` + venuePart + `}`)
}

func TestManifestVenueValidation(t *testing.T) {
	// Geçerli: mekânlı manifest çözülür ve mekân erişilebilir.
	raw, err := json.Marshal(goldenVenue())
	if err != nil {
		t.Fatal(err)
	}
	m, err := Parse(venueManifestJSON(string(raw)))
	if err != nil {
		t.Fatalf("geçerli mekân reddedildi: %v", err)
	}
	if m.Venue == nil || len(m.Venue.Blocks) != 3 {
		t.Fatal("mekân çözülmedi")
	}

	// Landmark (saha/sahne) isteğe bağlıdır ve geçerli biçimde kabul edilir.
	m2, err := Parse(venueManifestJSON(`{"blocks": [
		{"id": "A", "rows": 1, "seats_per_row": 1}],
		"landmark": {"kind": "pitch", "x": 0, "y": 0, "w": 105, "d": 68}}`))
	if err != nil {
		t.Fatalf("landmark'lı mekân reddedildi: %v", err)
	}
	if m2.Venue.Landmark == nil || m2.Venue.Landmark.Kind != LandmarkPitch {
		t.Fatal("landmark çözülmedi")
	}

	bad := []struct {
		name, venue string
	}{
		{"boş blok listesi", `{"blocks": []}`},
		{"boş id", `{"blocks": [{"id": "", "rows": 1, "seats_per_row": 1}]}`},
		{"boşluklu id", `{"blocks": [{"id": "A 1", "rows": 1, "seats_per_row": 1}]}`},
		{"tekrarlı id", `{"blocks": [
			{"id": "A", "rows": 1, "seats_per_row": 1},
			{"id": "A", "rows": 1, "seats_per_row": 1}]}`},
		{"sıfır satır", `{"blocks": [{"id": "A", "rows": 0, "seats_per_row": 1}]}`},
		{"çok sıralı sıfır row_vec", `{"blocks": [
			{"id": "A", "rows": 2, "seats_per_row": 1, "seat_vec": {"x": 0.5}}]}`},
		{"çok koltuklu sıfır seat_vec", `{"blocks": [
			{"id": "A", "rows": 1, "seats_per_row": 2, "row_vec": {"y": 0.8}}]}`},
		{"bilinmeyen alan (yazım hatası)", `{"blocks": [
			{"id": "A", "rowz": 1, "seats_per_row": 1}]}`},
		{"koltuk üst sınırı", `{"blocks": [
			{"id": "A", "rows": 1000, "seats_per_row": 1000,
			 "row_vec": {"y": 0.8}, "seat_vec": {"x": 0.5}}]}`},
		{"geçersiz landmark türü", `{"blocks": [
			{"id": "A", "rows": 1, "seats_per_row": 1}],
			"landmark": {"kind": "pool", "x": 0, "y": 0, "w": 10, "d": 5}}`},
		{"landmark boyutsuz", `{"blocks": [
			{"id": "A", "rows": 1, "seats_per_row": 1}],
			"landmark": {"kind": "pitch", "x": 0, "y": 0, "w": 0, "d": 5}}`},
	}
	for _, c := range bad {
		if _, err := Parse(venueManifestJSON(c.venue)); err == nil {
			t.Errorf("%s: hata bekleniyordu", c.name)
		}
	}
}

// Mekânsız manifestin kanonik baytları venue alanından etkilenmez — eski
// gösterilerin SHA-256 özetleri değişmemeli (paket doğrulaması kırılırdı).
func TestCanonicalWithoutVenueUnchanged(t *testing.T) {
	m, err := Parse([]byte(`{
		"title": "Deneme",
		"sequences": [{"id": "s1", "title": "S1", "duration_ms": 1000}]}`))
	if err != nil {
		t.Fatal(err)
	}
	data, _, err := m.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "venue") {
		t.Fatalf("mekânsız manifestin kanonik JSON'unda venue çıktı: %s", data)
	}
}

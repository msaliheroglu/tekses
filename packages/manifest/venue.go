package manifest

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Mekân modeli (Faz 4, koltuk bazlı koreografi).
//
// Mekân = grid'li bloklar: her blok sıra×koltuk ızgarasıdır ve uzamda bir
// yerde durur. Koltuk konumu doğrusaldır:
//
//	konum = origin + (sıra-1)·row_vec + (koltuk-1)·seat_vec
//
// row_vec bir sıradan SONRAKİNE (z bileşeni tribün eğimini taşır), seat_vec
// bir koltuktan YANINDAKİNE giden vektördür (metre). Kavisli tribün, birkaç
// düz blokla yaklaşıklanır. Efektler mekânın sınır kutusuna göre 0..1
// normalize koordinat kullanır (SeatPos.U/V/W) — böylece aynı "dalga soldan
// sağa" tanımı her mekânda çalışır.
//
// Koltuk kimliği İSTEMCİDE kalır (karar dosyası: koltuk başına tel trafiği
// yok); sunucu yalnızca manifesti doğrular.

const (
	maxVenueBlocks = 2000
	maxVenueSeats  = 500_000 // toplam koltuk (80k stadyum + bol pay)

	// Sıfıra yapışık adım vektörü üst üste koltuk üretir (float tozu dahil).
	minStepMeters = 0.001
)

// Blok kimliği boşluk içeremez (koltuk dizgisi ve URL'de taşınır).
var blockIDRe = regexp.MustCompile(`^\S{1,64}$`)

type Venue struct {
	Name   string  `json:"name,omitempty"`
	Blocks []Block `json:"blocks"`
}

type Block struct {
	ID          string `json:"id"`
	Rows        int    `json:"rows"`
	SeatsPerRow int    `json:"seats_per_row"`
	Origin      Vec3   `json:"origin"`   // 1. sıra 1. koltuk (metre)
	RowVec      Vec3   `json:"row_vec"`  // sıra r → r+1
	SeatVec     Vec3   `json:"seat_vec"` // koltuk s → s+1
}

type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func (v Vec3) add(o Vec3) Vec3      { return Vec3{v.X + o.X, v.Y + o.Y, v.Z + o.Z} }
func (v Vec3) scale(k float64) Vec3 { return Vec3{v.X * k, v.Y * k, v.Z * k} }
func (v Vec3) length() float64      { return math.Sqrt(v.X*v.X + v.Y*v.Y + v.Z*v.Z) }

// SeatRef, bir koltuğun mekân içindeki adresidir (1 tabanlı).
type SeatRef struct {
	Block string
	Row   int
	Seat  int
}

// String, koltuk dizgisi biçimidir: "BLOK-SIRA-KOLTUK" (QR/URL sözleşmesi,
// F4.1 bunu taşıyacak). Blok kimliği tire içerebilir; çözümleme sağdan
// yapılır ve son iki parça daima sıra/koltuktur.
func (r SeatRef) String() string {
	return fmt.Sprintf("%s-%d-%d", r.Block, r.Row, r.Seat)
}

// ParseSeatRef, "BLOK-SIRA-KOLTUK" dizgisini çözer. Eşleşme birebir yapılır
// (büyük/küçük harf katlanmaz — Türkçe İ/i tuzaklarından uzak dur).
func ParseSeatRef(s string) (SeatRef, error) {
	parts := strings.Split(s, "-")
	if len(parts) < 3 {
		return SeatRef{}, fmt.Errorf("koltuk %q BLOK-SIRA-KOLTUK biçiminde olmalı", s)
	}
	row, err := strconv.Atoi(parts[len(parts)-2])
	if err != nil || row < 1 {
		return SeatRef{}, fmt.Errorf("koltuk %q: sıra pozitif tam sayı olmalı", s)
	}
	seat, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || seat < 1 {
		return SeatRef{}, fmt.Errorf("koltuk %q: koltuk numarası pozitif tam sayı olmalı", s)
	}
	block := strings.Join(parts[:len(parts)-2], "-")
	if block == "" {
		return SeatRef{}, fmt.Errorf("koltuk %q: blok kimliği boş", s)
	}
	return SeatRef{Block: block, Row: row, Seat: seat}, nil
}

// SeatPos, çözülmüş koltuk konumudur: metre cinsinden mekân uzayı (X/Y/Z)
// ve mekân sınır kutusuna göre 0..1 normalize eksenler (U/V/W). Efektler
// normalize ekseni kullanır; kutunun sıfır genişlikli ekseni 0.5 sayılır.
type SeatPos struct {
	X, Y, Z float64
	U, V, W float64
}

func (m Manifest) validateVenue() error {
	v := m.Venue
	if v == nil {
		return nil
	}
	if len(v.Blocks) == 0 {
		return fmt.Errorf("venue: en az bir blok gerekli")
	}
	if len(v.Blocks) > maxVenueBlocks {
		return fmt.Errorf("venue: blok sayısı %d üst sınırı aşıyor", maxVenueBlocks)
	}
	ids := map[string]bool{}
	totalSeats := 0
	for i, b := range v.Blocks {
		where := fmt.Sprintf("venue.blocks[%d]", i)
		if !blockIDRe.MatchString(b.ID) {
			return fmt.Errorf("%s: id boşluk içermeyen 1..64 karakter olmalı", where)
		}
		if ids[b.ID] {
			return fmt.Errorf("%s: id %q tekrar ediyor", where, b.ID)
		}
		ids[b.ID] = true
		if b.Rows < 1 || b.SeatsPerRow < 1 {
			return fmt.Errorf("%s: rows ve seats_per_row en az 1 olmalı", where)
		}
		totalSeats += b.Rows * b.SeatsPerRow
		if b.Rows > 1 && b.RowVec.length() < minStepMeters {
			return fmt.Errorf("%s: rows > 1 için row_vec sıfır olamaz", where)
		}
		if b.SeatsPerRow > 1 && b.SeatVec.length() < minStepMeters {
			return fmt.Errorf("%s: seats_per_row > 1 için seat_vec sıfır olamaz", where)
		}
	}
	if totalSeats > maxVenueSeats {
		return fmt.Errorf("venue: toplam koltuk %d üst sınırı (%d) aşıyor", totalSeats, maxVenueSeats)
	}
	return nil
}

// seatWorld, bloktaki (1 tabanlı) sıra/koltuğun metre konumunu verir.
func seatWorld(b Block, row, seat int) Vec3 {
	return b.Origin.
		add(b.RowVec.scale(float64(row - 1))).
		add(b.SeatVec.scale(float64(seat - 1)))
}

// Bounds, mekânın sınır kutusunu verir (min, max; metre). Konum sıra ve
// koltukta doğrusal olduğundan uç değerler blok köşelerindedir — koltuk
// koltuk gezmeye gerek yok.
func (v Venue) Bounds() (min, max Vec3) {
	first := true
	for _, b := range v.Blocks {
		for _, row := range [2]int{1, b.Rows} {
			for _, seat := range [2]int{1, b.SeatsPerRow} {
				p := seatWorld(b, row, seat)
				if first {
					min, max = p, p
					first = false
					continue
				}
				min.X, max.X = math.Min(min.X, p.X), math.Max(max.X, p.X)
				min.Y, max.Y = math.Min(min.Y, p.Y), math.Max(max.Y, p.Y)
				min.Z, max.Z = math.Min(min.Z, p.Z), math.Max(max.Z, p.Z)
			}
		}
	}
	return min, max
}

// Resolve, koltuğu konuma çözer. Bilinmeyen blok ya da ızgara dışı
// sıra/koltuk hatadır (telefon bunu kullanıcıya "koltuk bulunamadı" der).
func (v Venue) Resolve(ref SeatRef) (SeatPos, error) {
	var blk *Block
	for i := range v.Blocks {
		if v.Blocks[i].ID == ref.Block {
			blk = &v.Blocks[i]
			break
		}
	}
	if blk == nil {
		return SeatPos{}, fmt.Errorf("blok %q mekânda yok", ref.Block)
	}
	if ref.Row < 1 || ref.Row > blk.Rows {
		return SeatPos{}, fmt.Errorf("blok %q: sıra %d aralık dışı (1..%d)", ref.Block, ref.Row, blk.Rows)
	}
	if ref.Seat < 1 || ref.Seat > blk.SeatsPerRow {
		return SeatPos{}, fmt.Errorf("blok %q: koltuk %d aralık dışı (1..%d)", ref.Block, ref.Seat, blk.SeatsPerRow)
	}
	p := seatWorld(*blk, ref.Row, ref.Seat)
	min, max := v.Bounds()
	return SeatPos{
		X: p.X, Y: p.Y, Z: p.Z,
		U: normAxis(p.X, min.X, max.X),
		V: normAxis(p.Y, min.Y, max.Y),
		W: normAxis(p.Z, min.Z, max.Z),
	}, nil
}

func normAxis(p, lo, hi float64) float64 {
	if hi-lo < minStepMeters {
		return 0.5
	}
	return (p - lo) / (hi - lo)
}

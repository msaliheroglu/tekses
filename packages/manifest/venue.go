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
	// Landmark, sahanın/sahnenin yeridir — panel editörü ve 3B önizleme
	// İÇİN görsel bağlamdır (izleyen mekânda nerede olduğunu anlasın);
	// telefon koreografisi bunu KULLANMAZ (normalize eksenler yalnız
	// koltuklardan türetilir, işaret eklemek u/v/w'yi değiştirmez).
	Landmark *Landmark `json:"landmark,omitempty"`
}

// Landmark türleri.
const (
	LandmarkPitch = "pitch" // saha (stadyum)
	LandmarkStage = "stage" // sahne (salon/konser)
)

type Landmark struct {
	Kind string  `json:"kind"` // pitch | stage
	X    float64 `json:"x"`    // merkez (metre)
	Y    float64 `json:"y"`
	W    float64 `json:"w"` // genişlik (x ekseni)
	D    float64 `json:"d"` // derinlik (y ekseni)
}

// Blok türleri.
const (
	BlockGrid = "grid" // düz ızgara (varsayılan; kind boş = grid)
	BlockArc  = "arc"  // yay: köşe/oval tribün — sıra başına koltuk yarıçapla artar
)

type Block struct {
	ID   string `json:"id"`
	Kind string `json:"kind,omitempty"` // ""|grid | arc
	Rows int    `json:"rows"`

	// grid alanları:
	SeatsPerRow int  `json:"seats_per_row,omitempty"`
	Origin      Vec3 `json:"origin,omitzero"`   // 1. sıra 1. koltuk (metre)
	RowVec      Vec3 `json:"row_vec,omitzero"`  // sıra r → r+1
	SeatVec     Vec3 `json:"seat_vec,omitzero"` // koltuk s → s+1

	// arc alanları: koltuklar, center etrafında angle_start→angle_end yayına
	// dizilir; 1. sıra radius yarıçapındadır, her sıra row_step kadar dışarı
	// ve rake kadar yukarı gider. Sıradaki koltuk sayısı yay uzunluğundan
	// türetilir (SeatsInRow) — sahaya yakın sırada az, yukarıda çok koltuk.
	Center        *Vec3   `json:"center,omitempty"`
	Radius        float64 `json:"radius,omitempty"`    // 1. sıranın yarıçapı (m)
	RowStep       float64 `json:"row_step,omitempty"`  // sıra başına yarıçap artışı (m)
	Rake          float64 `json:"rake,omitempty"`      // sıra başına yükselme (m)
	AngleStartDeg float64 `json:"angle_start_deg,omitempty"` // koltuk 1 bu uçta
	AngleEndDeg   float64 `json:"angle_end_deg,omitempty"`
	SeatStep      float64 `json:"seat_step,omitempty"` // yay boyunca koltuk aralığı (m)
}

// SeatsInRow, o sıradaki koltuk sayısıdır. Grid'de sabittir; yayda, sıranın
// yay uzunluğuna sığan koltuk sayısıdır (koltuk 1 angle_start ucundadır).
func (b Block) SeatsInRow(row int) int {
	if b.Kind != BlockArc {
		return b.SeatsPerRow
	}
	r := b.Radius + float64(row-1)*b.RowStep
	span := (b.AngleEndDeg - b.AngleStartDeg) * math.Pi / 180
	n := int(math.Floor(r*span/b.SeatStep)) + 1
	if n < 1 {
		n = 1
	}
	return n
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
		if b.Rows < 1 {
			return fmt.Errorf("%s: rows en az 1 olmalı", where)
		}
		switch b.Kind {
		case "", BlockGrid:
			if b.SeatsPerRow < 1 {
				return fmt.Errorf("%s: seats_per_row en az 1 olmalı", where)
			}
			totalSeats += b.Rows * b.SeatsPerRow
			if b.Rows > 1 && b.RowVec.length() < minStepMeters {
				return fmt.Errorf("%s: rows > 1 için row_vec sıfır olamaz", where)
			}
			if b.SeatsPerRow > 1 && b.SeatVec.length() < minStepMeters {
				return fmt.Errorf("%s: seats_per_row > 1 için seat_vec sıfır olamaz", where)
			}
		case BlockArc:
			if b.Center == nil {
				return fmt.Errorf("%s: arc blok için center zorunlu", where)
			}
			if b.Radius < 1 {
				return fmt.Errorf("%s: arc blok için radius en az 1 m olmalı", where)
			}
			if b.SeatStep < 0.05 {
				return fmt.Errorf("%s: arc blok için seat_step en az 0.05 m olmalı", where)
			}
			if b.Rows > 1 && b.RowStep < minStepMeters {
				return fmt.Errorf("%s: rows > 1 için row_step sıfır olamaz", where)
			}
			span := b.AngleEndDeg - b.AngleStartDeg
			if span <= 0 || span > 360 {
				return fmt.Errorf("%s: açı aralığı (angle_end_deg−angle_start_deg) 0..360 içinde pozitif olmalı", where)
			}
			for row := 1; row <= b.Rows; row++ {
				totalSeats += b.SeatsInRow(row)
			}
		default:
			return fmt.Errorf("%s: kind %q geçersiz (grid|arc)", where, b.Kind)
		}
	}
	if totalSeats > maxVenueSeats {
		return fmt.Errorf("venue: toplam koltuk %d üst sınırı (%d) aşıyor", totalSeats, maxVenueSeats)
	}
	if lm := v.Landmark; lm != nil {
		if lm.Kind != LandmarkPitch && lm.Kind != LandmarkStage {
			return fmt.Errorf("venue.landmark: kind %q geçersiz (pitch|stage)", lm.Kind)
		}
		if lm.W <= 0 || lm.D <= 0 || lm.W > 2000 || lm.D > 2000 {
			return fmt.Errorf("venue.landmark: w/d 0..2000 m aralığında pozitif olmalı")
		}
	}
	return nil
}

// seatWorld, bloktaki (1 tabanlı) sıra/koltuğun metre konumunu verir.
// ARİTMETİK DÖRT GERÇEKLEMEDE BİREBİRDİR (Dart venue.dart, join.html JS,
// panel venueEditor.ts) — değiştirirken dördünü birden değiştir.
func seatWorld(b Block, row, seat int) Vec3 {
	if b.Kind == BlockArc {
		r := b.Radius + float64(row-1)*b.RowStep
		theta := b.AngleStartDeg*math.Pi/180 + float64(seat-1)*b.SeatStep/r
		return Vec3{
			X: b.Center.X + r*math.Cos(theta),
			Y: b.Center.Y + r*math.Sin(theta),
			Z: b.Center.Z + float64(row-1)*b.Rake,
		}
	}
	return b.Origin.
		add(b.RowVec.scale(float64(row - 1))).
		add(b.SeatVec.scale(float64(seat - 1)))
}

// boundsSamples, bloğun sınır kutusunu geren örnek noktalardır. Grid'de dört
// köşe yeter (konum doğrusal). Yayda uçlar köşede olmayabilir: iki uç sıra ×
// (başlangıç, bitiş ve aradaki her 90° katı) açıları örneklenir — kosinüs/
// sinüsün uç değerleri eksen katlarındadır.
func boundsSamples(b Block) []Vec3 {
	if b.Kind != BlockArc {
		return []Vec3{
			seatWorld(b, 1, 1),
			seatWorld(b, 1, b.SeatsPerRow),
			seatWorld(b, b.Rows, 1),
			seatWorld(b, b.Rows, b.SeatsPerRow),
		}
	}
	angles := []float64{b.AngleStartDeg, b.AngleEndDeg}
	for k := math.Ceil(b.AngleStartDeg / 90); k*90 < b.AngleEndDeg; k++ {
		if k*90 > b.AngleStartDeg {
			angles = append(angles, k*90)
		}
	}
	var out []Vec3
	for _, row := range [2]int{1, b.Rows} {
		r := b.Radius + float64(row-1)*b.RowStep
		z := b.Center.Z + float64(row-1)*b.Rake
		for _, deg := range angles {
			th := deg * math.Pi / 180
			out = append(out, Vec3{
				X: b.Center.X + r*math.Cos(th),
				Y: b.Center.Y + r*math.Sin(th),
				Z: z,
			})
		}
	}
	return out
}

// Bounds, mekânın sınır kutusunu verir (min, max; metre).
func (v Venue) Bounds() (min, max Vec3) {
	first := true
	for _, b := range v.Blocks {
		for _, p := range boundsSamples(b) {
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
	if rowSeats := blk.SeatsInRow(ref.Row); ref.Seat < 1 || ref.Seat > rowSeats {
		return SeatPos{}, fmt.Errorf("blok %q: koltuk %d aralık dışı (%d. sırada 1..%d)",
			ref.Block, ref.Seat, ref.Row, rowSeats)
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

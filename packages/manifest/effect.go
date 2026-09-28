package manifest

import (
	"fmt"
	"math"
	"strconv"
)

// Uzamsal efektler (F4.2, koltuk bazlı koreografi).
//
// Efekt, ekran/fener kuesine takılan SAF bir fonksiyondur:
// (normalize koltuk konumu u/v/w, kue içi süre ms) → renk. Telefon, tarayıcı
// ve panel önizlemesi rengini kendisi türetir — koltuk başına tel trafiği
// yoktur. Buradaki EvalEffect REFERANStır: Dart (spatial_effect.dart) ve
// join.html JS'i birebir aynı aritmetiği taşır ve
// testdata/effect_vectors.json altın vektörlerine karşı doğrulanır.
//
// Işık güvenliği: efektli kue flash_hz taşıyamaz; dalga süpürmesi ve bitmap
// kaydırması, tek bir koltuğun gördüğü değişim sıklığı MaxFlashHz'i (3 Hz)
// aşmayacak şekilde doğrulanır. Işığa duyarlı mod efekti değerlendirmez,
// kuenin sabit rengini basar (istemci tarafı).
//
// Koltuksuz telefon mekânın ortası sayılır (u=v=w=0.5) — herkes katılır,
// kimse sessizce dışarıda kalmaz.

// Efekt türleri.
const (
	EffectWave     = "wave"     // yönlü tarama bandı (meksika dalgası)
	EffectGradient = "gradient" // eksen boyunca sabit renk geçişi
	EffectBitmap   = "bitmap"   // koltuk = piksel (bayrak; kayan yazı/slogan)
	EffectCycle    = "cycle"    // renk döngüsü (uzamsal değil; marş arka planı)
)

const (
	// Koltuk başına değişim sıklığı sınırı: 1000 ms / MaxFlashHz, yukarı.
	minEffectPeriodMs = 334

	maxBitmapRows = 64
	maxBitmapCols = 256
	maxPaletteLen = 16
)

type Effect struct {
	Kind string `json:"kind"`
	// Yatay eksen: "u" (varsayılan), "v", "w" ya da "ring" — ring, mekân
	// merkezinin etrafındaki açıdır (0..1; +u yönünden başlar, saat yönünün
	// tersine artar): meksika dalgası ve stadyumu turlayan yazı bununla
	// tribünden tribüne SIRAYLA ilerler. Bitmap'in dikeyi daima w'dur.
	Axis    string `json:"axis,omitempty"`
	Reverse bool   `json:"reverse,omitempty"` // ring'de saat yönü demektir
	// wave: bir tam süpürmenin süresi. bitmap: kaydırma dönemi (0 = sabit).
	// cycle: renk başına süre (0 = söz satırını izle; motor düzeyinde).
	PeriodMs int `json:"period_ms,omitempty"`
	// wave: bandın normalize genişliği (0..1].
	Width float64 `json:"width,omitempty"`
	// wave: bant dışının rengi ('' = kapalı/siyah). gradient: geçişin ucu
	// (zorunlu; başlangıç kuenin color'ı).
	Color2 string  `json:"color2,omitempty"`
	Bitmap *Bitmap `json:"bitmap,omitempty"`
	// Blocks: yalnız bu bloklardaki koltuklar oynar (boş = herkes). Tek
	// tribünlük koreografi ve blok blok kurgular bununla yapılır; mekân
	// planındaki blok kimlikleriyle doğrulanır. Koltuksuz istemci blok
	// filtresi olan efektte daima kapalıdır.
	Blocks []string `json:"blocks,omitempty"`
	// Area: bitmap'in yerleştirme penceresi (normalize yatay×dikey dikdörtgen;
	// yoksa tüm mekân). Desen pencerenin içine haritalanır, dışı kapalıdır —
	// moderatör bayrağı/sloganı tribünde istediği yere koyar.
	Area *Area `json:"area,omitempty"`
	// Colors: cycle'ın renk listesi (2..16 adet #RRGGBB).
	Colors []string `json:"colors,omitempty"`
}

// Area: normalize yerleştirme penceresi. Yatay eksen efektin axis'idir
// (u/v/w/ring), dikey daima w.
type Area struct {
	U0 float64 `json:"u0"`
	W0 float64 `json:"w0"`
	U1 float64 `json:"u1"`
	W1 float64 `json:"w1"`
}

// Bitmap: satır dizgileri, karakter = palet indeksi (0-9a-f), '.' = kapalı.
// Kayan yazı/slogan, panel editörünün (F4.3) metinden ürettiği geniş bir
// bitmap'tir — manifest ayrı bir yazı türü taşımaz.
type Bitmap struct {
	Palette []string `json:"palette"`
	Rows    []string `json:"rows"`
}

func isHexDigit(c byte) (int, bool) {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0'), true
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10, true
	}
	return 0, false
}

func (e *Effect) validate(laneKind string, venueBlocks map[string]bool) error {
	switch e.Axis {
	case "", "u", "v", "w", "ring":
	default:
		return fmt.Errorf("effect.axis %q geçersiz (u|v|w|ring)", e.Axis)
	}
	if len(e.Blocks) > 0 {
		if venueBlocks == nil {
			return fmt.Errorf("effect.blocks için manifest'te mekân planı (venue) gerekli")
		}
		if len(e.Blocks) > 64 {
			return fmt.Errorf("effect.blocks en çok 64 blok alır")
		}
		seen := map[string]bool{}
		for _, id := range e.Blocks {
			if !venueBlocks[id] {
				return fmt.Errorf("effect.blocks: %q mekân planında yok", id)
			}
			if seen[id] {
				return fmt.Errorf("effect.blocks: %q tekrar ediyor", id)
			}
			seen[id] = true
		}
	}
	if e.Area != nil && e.Kind != EffectBitmap {
		return fmt.Errorf("effect.area yalnız bitmap efektinde kullanılır")
	}
	if e.Kind != EffectCycle && len(e.Colors) > 0 {
		return fmt.Errorf("colors yalnız cycle efektinde kullanılır")
	}
	switch e.Kind {
	case EffectWave:
		if e.Width <= 0 || e.Width > 1 {
			return fmt.Errorf("wave: width 0..1 aralığında (0 hariç) olmalı")
		}
		if e.PeriodMs < minEffectPeriodMs {
			return fmt.Errorf("wave: period_ms en az %d olmalı (ışığa duyarlılık sınırı)", minEffectPeriodMs)
		}
		if e.Color2 != "" && !colorRe.MatchString(e.Color2) {
			return fmt.Errorf("wave: color2 #RRGGBB biçiminde olmalı")
		}
		if e.Bitmap != nil {
			return fmt.Errorf("wave: bitmap taşıyamaz")
		}
	case EffectGradient:
		if laneKind == LaneTorch {
			// Fener ikilidir; ara renk basamağı anlamsız.
			return fmt.Errorf("gradient torch şeridinde kullanılamaz")
		}
		if !colorRe.MatchString(e.Color2) {
			return fmt.Errorf("gradient: color2 #RRGGBB biçiminde zorunlu")
		}
		if e.PeriodMs != 0 || e.Width != 0 || e.Bitmap != nil {
			return fmt.Errorf("gradient: period_ms/width/bitmap taşıyamaz")
		}
	case EffectBitmap:
		b := e.Bitmap
		if b == nil {
			return fmt.Errorf("bitmap: bitmap alanı zorunlu")
		}
		if e.Width != 0 || e.Color2 != "" {
			return fmt.Errorf("bitmap: width/color2 taşıyamaz")
		}
		if len(b.Palette) == 0 || len(b.Palette) > maxPaletteLen {
			return fmt.Errorf("bitmap: palette 1..%d renk olmalı", maxPaletteLen)
		}
		for i, c := range b.Palette {
			if !colorRe.MatchString(c) {
				return fmt.Errorf("bitmap: palette[%d] #RRGGBB biçiminde olmalı", i)
			}
		}
		if len(b.Rows) == 0 || len(b.Rows) > maxBitmapRows {
			return fmt.Errorf("bitmap: rows 1..%d satır olmalı", maxBitmapRows)
		}
		cols := len(b.Rows[0])
		if cols == 0 || cols > maxBitmapCols {
			return fmt.Errorf("bitmap: satır uzunluğu 1..%d olmalı", maxBitmapCols)
		}
		for i, row := range b.Rows {
			if len(row) != cols {
				return fmt.Errorf("bitmap: rows[%d] uzunluğu ilk satırla aynı olmalı", i)
			}
			for j := 0; j < len(row); j++ {
				if row[j] == '.' {
					continue
				}
				idx, ok := isHexDigit(row[j])
				if !ok || idx >= len(b.Palette) {
					return fmt.Errorf("bitmap: rows[%d][%d] %q palet indeksi değil", i, j, row[j])
				}
			}
		}
		// Kaydırmada bir koltuğun önünden saniyede en çok MaxFlashHz sütun
		// geçebilir: dönem başına cols sütun → period ≥ cols·334 ms.
		if e.PeriodMs != 0 && e.PeriodMs < cols*minEffectPeriodMs {
			return fmt.Errorf("bitmap: kaydırma için period_ms en az %d olmalı (%d sütun × %d ms; ışığa duyarlılık sınırı)",
				cols*minEffectPeriodMs, cols, minEffectPeriodMs)
		}
		if a := e.Area; a != nil {
			if !(a.U0 >= 0 && a.U0 < a.U1 && a.U1 <= 1) ||
				!(a.W0 >= 0 && a.W0 < a.W1 && a.W1 <= 1) {
				return fmt.Errorf("bitmap: area penceresi 0..1 içinde ve u0<u1, w0<w1 olmalı")
			}
		}
	case EffectCycle:
		if laneKind == LaneTorch {
			// Fener ikilidir; renk döngüsü anlamsız (yanıp sönme için flash_hz var).
			return fmt.Errorf("cycle torch şeridinde kullanılamaz")
		}
		if len(e.Colors) < 2 || len(e.Colors) > maxPaletteLen {
			return fmt.Errorf("cycle: colors 2..%d renk olmalı", maxPaletteLen)
		}
		for i, c := range e.Colors {
			if !colorRe.MatchString(c) {
				return fmt.Errorf("cycle: colors[%d] #RRGGBB biçiminde olmalı", i)
			}
		}
		// period_ms = 0 → renk, söz satırını izler (motor düzeyi kural:
		// başlamış söz satırı sayısı mod renk sayısı).
		if e.PeriodMs != 0 && e.PeriodMs < minEffectPeriodMs {
			return fmt.Errorf("cycle: period_ms 0 (söz izler) ya da en az %d olmalı (ışığa duyarlılık sınırı)", minEffectPeriodMs)
		}
		if e.Axis != "" || e.Width != 0 || e.Color2 != "" || e.Bitmap != nil {
			return fmt.Errorf("cycle: axis/width/color2/bitmap taşıyamaz")
		}
	default:
		return fmt.Errorf("effect.kind %q geçersiz (wave|gradient|bitmap|cycle)", e.Kind)
	}
	return nil
}

func axisPos(e Effect, u, v, w float64) float64 {
	p := u
	switch e.Axis {
	case "v":
		p = v
	case "w":
		p = w
	case "ring":
		// Mekân merkezinin (normalize 0.5, 0.5) etrafındaki açı payı: 0..1,
		// +u yönünden başlar, saat yönünün TERSİNE artar. Dalga ve kaydırma
		// zaten sargılı olduğundan efekt stadyumu kesintisiz turlar.
		p = math.Atan2(v-0.5, u-0.5) / (2 * math.Pi)
		if p < 0 {
			p++
		}
	}
	if e.Reverse {
		p = 1 - p // ring'de: saat yönü
	}
	return p
}

func parseColor(c string) (r, g, b float64) {
	pr, _ := strconv.ParseUint(c[1:3], 16, 8)
	pg, _ := strconv.ParseUint(c[3:5], 16, 8)
	pb, _ := strconv.ParseUint(c[5:7], 16, 8)
	return float64(pr), float64(pg), float64(pb)
}

func lerpColor(c1, c2 string, t float64) string {
	r1, g1, b1 := parseColor(c1)
	r2, g2, b2 := parseColor(c2)
	return fmt.Sprintf("#%02X%02X%02X",
		int(math.Round(r1+(r2-r1)*t)),
		int(math.Round(g1+(g2-g1)*t)),
		int(math.Round(b1+(b2-b1)*t)))
}

// EvalEffect, efektin (u,v,w) konumundaki, block bloğundaki telefona kue
// başlangıcından sinceMs sonra basacağı rengi verir; "" = kapalı (siyah/
// fener sönük). cueColor, kuenin color alanıdır (wave bandı / gradyan
// başlangıcı; fener şeridinde değerlendirme "#FFFFFF" ile çağrılır, yalnız
// açık/kapalı önemli). block, istemcinin koltuk bloğudur ('' = koltuksuz).
// cycle'da period_ms=0 (söz izleme) MOTOR düzeyinde işlenir; burada ilk
// renk döner. ARİTMETİK DÖRT GERÇEKLEMEDE BİREBİRDİR — değiştirirken
// dördünü birden değiştir ve altın vektörleri yeniden üret.
func EvalEffect(e Effect, cueColor string, u, v, w float64, block string, sinceMs int) string {
	if len(e.Blocks) > 0 {
		ok := false
		for _, b := range e.Blocks {
			if b == block {
				ok = true
				break
			}
		}
		if !ok {
			return "" // blok filtresi: kapsam dışı (koltuksuz istemci dahil)
		}
	}
	switch e.Kind {
	case EffectWave:
		p := axisPos(e, u, v, w)
		front := float64(sinceMs%e.PeriodMs) / float64(e.PeriodMs)
		d := math.Abs(p - front)
		if d > 0.5 {
			d = 1 - d // süpürme sargılıdır: 0.95 ile 0.05 komşudur
		}
		if d <= e.Width/2 {
			return cueColor
		}
		return e.Color2
	case EffectGradient:
		return lerpColor(cueColor, e.Color2, axisPos(e, u, v, w))
	case EffectCycle:
		if e.PeriodMs <= 0 {
			return e.Colors[0]
		}
		return e.Colors[(sinceMs/e.PeriodMs)%len(e.Colors)]
	case EffectBitmap:
		b := e.Bitmap
		cols := len(b.Rows[0])
		p := axisPos(e, u, v, w)
		q := w // dikey daima w
		if a := e.Area; a != nil {
			// Yerleştirme penceresi: dışı kapalı, içi 0..1'e haritalanır.
			if p < a.U0 || p > a.U1 || q < a.W0 || q > a.W1 {
				return ""
			}
			p = (p - a.U0) / (a.U1 - a.U0)
			q = (q - a.W0) / (a.W1 - a.W0)
		}
		if e.PeriodMs > 0 {
			// Kaydırma: desen p ekseninde sola akar, dönem başına bir tam tur.
			p += float64(sinceMs%e.PeriodMs) / float64(e.PeriodMs)
			if p >= 1 {
				p -= 1
			}
		}
		col := int(math.Floor(p * float64(cols)))
		if col >= cols {
			col = cols - 1 // p=1 ucu son sütuna yapışır
		}
		if col < 0 {
			col = 0
		}
		nrows := len(b.Rows)
		rowIdx := int(math.Floor((1 - q) * float64(nrows))) // ilk satır = tepe
		if rowIdx >= nrows {
			rowIdx = nrows - 1
		}
		if rowIdx < 0 {
			rowIdx = 0
		}
		ch := b.Rows[rowIdx][col]
		if ch == '.' {
			return ""
		}
		idx, _ := isHexDigit(ch)
		return b.Palette[idx]
	}
	return ""
}

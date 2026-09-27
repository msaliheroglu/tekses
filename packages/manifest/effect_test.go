package manifest

import (
	"encoding/json"
	"math"
	"os"
	"testing"
)

// Efekt altın vektörleri: Dart (spatial_effect.dart) ve join.html JS'i aynı
// dosyaya karşı doğrulanır — üç gerçekleme aynı (konum, an) için AYNI rengi
// basmalı. Üretici: go test ./packages/manifest -run TestEffectGolden -update
const effectVectorsPath = "testdata/effect_vectors.json"

type effectVector struct {
	Name     string  `json:"name"`
	CueColor string  `json:"cue_color"`
	Effect   Effect  `json:"effect"`
	U        float64 `json:"u"`
	V        float64 `json:"v"`
	W        float64 `json:"w"`
	SinceMs  int     `json:"since_ms"`
	Want     string  `json:"want"`
}

var flagBitmap = &Bitmap{
	// 4 sütun × 2 satır: üst yarı kırmızı, alt yarı beyaz; sağ alt köşe delik.
	Palette: []string{"#E30A17", "#FFFFFF"},
	Rows:    []string{"0000", "111."},
}

func effectGoldenInputs() []effectVector {
	return []effectVector{
		// Dalga: 2 sn'de u ekseninde bir tam süpürme, %20 bant.
		{Name: "dalga bant içinde", CueColor: "#FF0000",
			Effect: Effect{Kind: EffectWave, PeriodMs: 2000, Width: 0.2},
			U:      0.5, V: 0.1, W: 0.1, SinceMs: 1000},
		{Name: "dalga bant dışında (arka plan boş=kapalı)", CueColor: "#FF0000",
			Effect: Effect{Kind: EffectWave, PeriodMs: 2000, Width: 0.2},
			U:      0.9, V: 0.1, W: 0.1, SinceMs: 100},
		{Name: "dalga sargı: 0.98 ile 0.05 komşu", CueColor: "#FF0000",
			Effect: Effect{Kind: EffectWave, PeriodMs: 2000, Width: 0.2},
			U:      0.98, V: 0, W: 0, SinceMs: 100},
		{Name: "dalga ters yön + arka plan rengi + v ekseni", CueColor: "#00FF00",
			Effect: Effect{Kind: EffectWave, Axis: "v", Reverse: true,
				PeriodMs: 4000, Width: 0.5, Color2: "#101010"},
			U: 0, V: 0.75, W: 0, SinceMs: 1000},
		// Gradyan: konumdan renk, zamandan bağımsız.
		{Name: "gradyan çeyrek", CueColor: "#000000",
			Effect: Effect{Kind: EffectGradient, Color2: "#FFFFFF"},
			U:      0.25, V: 0, W: 0, SinceMs: 999999},
		{Name: "gradyan w ekseni ters", CueColor: "#FF0000",
			Effect: Effect{Kind: EffectGradient, Axis: "w", Reverse: true, Color2: "#0000FF"},
			U:      0, V: 0, W: 0.25, SinceMs: 0},
		// Bitmap: bayrak deseni, üst satır w=1 tarafı.
		{Name: "bitmap tepe sol", CueColor: "#FFFFFF",
			Effect: Effect{Kind: EffectBitmap, Bitmap: flagBitmap},
			U:      0.1, V: 0, W: 0.9, SinceMs: 0},
		{Name: "bitmap alt orta", CueColor: "#FFFFFF",
			Effect: Effect{Kind: EffectBitmap, Bitmap: flagBitmap},
			U:      0.5, V: 0, W: 0.2, SinceMs: 0},
		{Name: "bitmap delik (nokta=kapalı)", CueColor: "#FFFFFF",
			Effect: Effect{Kind: EffectBitmap, Bitmap: flagBitmap},
			U:      0.99, V: 0, W: 0.2, SinceMs: 0},
		{Name: "bitmap kaydırma yarım dönem", CueColor: "#FFFFFF",
			Effect: Effect{Kind: EffectBitmap, PeriodMs: 4 * minEffectPeriodMs, Bitmap: flagBitmap},
			U:      0.1, V: 0, W: 0.2, SinceMs: 2 * minEffectPeriodMs},
	}
}

func TestEffectGolden(t *testing.T) {
	if *updateVenue {
		cases := effectGoldenInputs()
		for i := range cases {
			cases[i].Want = EvalEffect(cases[i].Effect, cases[i].CueColor,
				cases[i].U, cases[i].V, cases[i].W, cases[i].SinceMs)
		}
		data, err := json.MarshalIndent(map[string]any{
			"comment": "üretici: go test ./packages/manifest -run TestEffectGolden -update",
			"cases":   cases,
		}, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(effectVectorsPath, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(effectVectorsPath)
	if err != nil {
		t.Fatalf("altın vektör dosyası okunamadı (önce -update ile üret): %v", err)
	}
	var in struct {
		Cases []effectVector `json:"cases"`
	}
	if err := json.Unmarshal(data, &in); err != nil {
		t.Fatal(err)
	}
	if len(in.Cases) == 0 {
		t.Fatal("altın vektör dosyası boş")
	}
	for _, c := range in.Cases {
		got := EvalEffect(c.Effect, c.CueColor, c.U, c.V, c.W, c.SinceMs)
		if got != c.Want {
			t.Errorf("%s: %q, altın %q", c.Name, got, c.Want)
		}
	}
}

// El hesabı — altın dosyadan bağımsız doğruluk denetimi.
func TestEvalEffectHand(t *testing.T) {
	wave := Effect{Kind: EffectWave, PeriodMs: 2000, Width: 0.2, Color2: "#101010"}
	// e=1000 → cephe 0.5; u=0.5 bandın tam ortası.
	if got := EvalEffect(wave, "#FF0000", 0.5, 0, 0, 1000); got != "#FF0000" {
		t.Errorf("dalga ortası: %q", got)
	}
	// u=0.3, cephe 0.5 → uzaklık 0.2 > 0.1 → arka plan.
	if got := EvalEffect(wave, "#FF0000", 0.3, 0, 0, 1000); got != "#101010" {
		t.Errorf("dalga dışı: %q", got)
	}
	// Sargı: u=0.98, cephe 0.05 → |0.93| → 1−0.93 = 0.07 ≤ 0.1 → bant içi.
	if got := EvalEffect(wave, "#FF0000", 0.98, 0, 0, 100); got != "#FF0000" {
		t.Errorf("dalga sargısı: %q", got)
	}

	grad := Effect{Kind: EffectGradient, Color2: "#FFFFFF"}
	// t=0.25: round(255·0.25)=64 → #404040.
	if got := EvalEffect(grad, "#000000", 0.25, 0, 0, 0); got != "#404040" {
		t.Errorf("gradyan: %q", got)
	}

	bmp := Effect{Kind: EffectBitmap, Bitmap: flagBitmap}
	// w=0.9 → satır 0 (tepe) → kırmızı; w=0.2 → satır 1 → beyaz; son sütun '.'
	if got := EvalEffect(bmp, "#FFFFFF", 0.1, 0, 0.9, 0); got != "#E30A17" {
		t.Errorf("bitmap tepe: %q", got)
	}
	if got := EvalEffect(bmp, "#FFFFFF", 0.5, 0, 0.2, 0); got != "#FFFFFF" {
		t.Errorf("bitmap alt: %q", got)
	}
	if got := EvalEffect(bmp, "#FFFFFF", 0.99, 0, 0.2, 0); got != "" {
		t.Errorf("bitmap delik: %q", got)
	}
	// u=1 ucu son sütuna yapışır (taşma yok).
	if got := EvalEffect(bmp, "#FFFFFF", 1, 0, 0.9, 0); got != "#E30A17" {
		t.Errorf("bitmap u=1: %q", got)
	}
}

func effectCueJSON(lane, cuePart string) []byte {
	return []byte(`{
		"title": "Deneme",
		"sequences": [{"id": "s1", "title": "S1", "duration_ms": 10000,
			"cue_lanes": [{"id": "l1", "kind": "` + lane + `",
				"cues": [` + cuePart + `]}]}]}`)
}

func TestEffectValidation(t *testing.T) {
	good := []struct{ lane, cue string }{
		{"screen", `{"at_ms": 0, "duration_ms": 5000, "color": "#FF0000",
			"effect": {"kind": "wave", "period_ms": 2000, "width": 0.2}}`},
		{"screen", `{"at_ms": 0, "duration_ms": 5000, "color": "#FF0000",
			"effect": {"kind": "gradient", "color2": "#0000FF", "axis": "w", "reverse": true}}`},
		{"torch", `{"at_ms": 0, "duration_ms": 5000,
			"effect": {"kind": "wave", "period_ms": 2000, "width": 0.1}}`},
		{"screen", `{"at_ms": 0, "duration_ms": 5000, "color": "#FF0000",
			"effect": {"kind": "bitmap", "period_ms": 1336,
				"bitmap": {"palette": ["#FF0000"], "rows": ["0.0.", "..00"]}}}`},
	}
	for i, c := range good {
		if _, err := Parse(effectCueJSON(c.lane, c.cue)); err != nil {
			t.Errorf("geçerli[%d] reddedildi: %v", i, err)
		}
	}

	bad := []struct{ name, lane, cue string }{
		{"efekt + flash_hz birlikte", "screen", `{"at_ms": 0, "duration_ms": 5000,
			"color": "#FF0000", "flash_hz": 2,
			"effect": {"kind": "wave", "period_ms": 2000, "width": 0.2}}`},
		{"bilinmeyen tür", "screen", `{"at_ms": 0, "duration_ms": 5000, "color": "#FF0000",
			"effect": {"kind": "spiral"}}`},
		{"dalga dönemi çok kısa (ışık güvenliği)", "screen", `{"at_ms": 0, "duration_ms": 5000,
			"color": "#FF0000", "effect": {"kind": "wave", "period_ms": 200, "width": 0.2}}`},
		{"dalga genişliği aralık dışı", "screen", `{"at_ms": 0, "duration_ms": 5000,
			"color": "#FF0000", "effect": {"kind": "wave", "period_ms": 2000, "width": 1.5}}`},
		{"gradyan color2 eksik", "screen", `{"at_ms": 0, "duration_ms": 5000,
			"color": "#FF0000", "effect": {"kind": "gradient"}}`},
		{"gradyan fenerde", "torch", `{"at_ms": 0, "duration_ms": 5000,
			"effect": {"kind": "gradient", "color2": "#0000FF"}}`},
		{"bitmap kaydırması çok hızlı", "screen", `{"at_ms": 0, "duration_ms": 5000,
			"color": "#FF0000", "effect": {"kind": "bitmap", "period_ms": 1000,
				"bitmap": {"palette": ["#FF0000"], "rows": ["0000"]}}}`},
		{"bitmap satır uzunlukları farklı", "screen", `{"at_ms": 0, "duration_ms": 5000,
			"color": "#FF0000", "effect": {"kind": "bitmap",
				"bitmap": {"palette": ["#FF0000"], "rows": ["00", "000"]}}}`},
		{"bitmap palet dışı indeks", "screen", `{"at_ms": 0, "duration_ms": 5000,
			"color": "#FF0000", "effect": {"kind": "bitmap",
				"bitmap": {"palette": ["#FF0000"], "rows": ["01"]}}}`},
		{"audio'da efekt", "audio", `{"at_ms": 0, "duration_ms": 5000, "asset_id": "a1",
			"effect": {"kind": "wave", "period_ms": 2000, "width": 0.2}}`},
		{"geçersiz eksen", "screen", `{"at_ms": 0, "duration_ms": 5000, "color": "#FF0000",
			"effect": {"kind": "wave", "period_ms": 2000, "width": 0.2, "axis": "x"}}`},
	}
	for _, c := range bad {
		if _, err := Parse(effectCueJSON(c.lane, c.cue)); err == nil {
			t.Errorf("%s: hata bekleniyordu", c.name)
		}
	}
}

// Işık güvenliği değişmezi: geçerli her dalga/kaydırma, tek koltuğun gördüğü
// açık→kapalı geçiş sıklığını MaxFlashHz'in altında tutar. Sınır sabitinin
// MaxFlashHz'le tutarlı kaldığını denetler (biri değişirse bu test kırılır).
func TestEffectPeriodMatchesFlashLimit(t *testing.T) {
	if minEffectPeriodMs < int(math.Ceil(1000.0/float64(MaxFlashHz))) {
		t.Fatalf("minEffectPeriodMs %d, 1000/MaxFlashHz %d'den küçük olamaz",
			minEffectPeriodMs, int(math.Ceil(1000.0/float64(MaxFlashHz))))
	}
}

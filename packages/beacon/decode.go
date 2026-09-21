package beacon

// Çözücü: kayıttan beacon patlamasını bulur (chirp korelasyonu) ve FSK
// yükünü Goertzel enerjileriyle çözer. Saf Go'dur — telefon tarafına (Dart)
// birebir taşınacak referans gerçeklemedir; birim testleri sözleşmeyi pinler.

import (
	"fmt"
	"math"
)

// Detection, kayıtta bulunan tek bir beacon çözümüdür.
type Detection struct {
	Payload Payload
	// StartSample, önekin (chirp) kayıttaki başlangıç örneğidir. Ateşleme
	// anı = kayıtta bu örneğin duyulduğu yerel an + Payload.CountdownMs.
	// (Geri sayım, ÖNEKİN BAŞINA göre tanımlıdır; kodlayıcı ve saha kılavuzu
	// aynı sözleşmeyi kullanır.)
	StartSample int
	// Score, chirp korelasyonunun normalize gücüdür (0..1; teşhis için).
	Score float64
}

// Decode, kayıttaki İLK beacon'ı çözer. Kayıt, patlamayı tamamen içermeli;
// örnekleme hızı kodlayıcıyla aynı olmak zorunda DEĞİLDİR (44,1k kayıt,
// 48k üretim çözülür — frekanslar hıza göre türetilir).
func Decode(samples []float64, sampleRate int) (Detection, error) {
	dets, err := DecodeAll(samples, sampleRate, 1)
	if err != nil {
		return Detection{}, err
	}
	return dets[0], nil
}

// DecodeAll, kayıttaki en çok max beacon'ı (zaman sırasıyla) çözer.
func DecodeAll(samples []float64, sampleRate, max int) ([]Detection, error) {
	if sampleRate < 40000 {
		return nil, fmt.Errorf("beacon: örnekleme hızı en az 40 kHz olmalı (%d verildi)", sampleRate)
	}
	sr := float64(sampleRate)
	// Bant dışı sesi (konuşma, müzik, geniş bant gürültü) at: beacon bandı
	// 18,5 kHz üstünde; süzgeçsiz korelasyon skoru kalabalık mekân kaydında
	// sulanır. 17,5 kHz kesimli 2. derece yüksek-geçiren yeterli ve Dart'a
	// birebir taşınabilir.
	samples = highpass(samples, sr, 17500)
	chirpN := int(ChirpSec * sr)
	gapN := int(GapSec * sr)
	symN := int(SymbolSec * sr)
	burstN := chirpN + gapN + PayloadBits*symN

	// Chirp şablonu (kodlayıcıyla aynı, genliksiz).
	tmpl := make([]float64, chirpN)
	phase := 0.0
	for i := 0; i < chirpN; i++ {
		t := float64(i) / float64(chirpN)
		freq := ChirpLowHz + (ChirpHighHz-ChirpLowHz)*t
		phase += 2 * math.Pi * freq / sr
		tmpl[i] = envelope(i, chirpN) * math.Sin(phase)
	}
	var tmplEnergy float64
	for _, v := range tmpl {
		tmplEnergy += v * v
	}

	var out []Detection
	searchFrom := 0
	for len(out) < max {
		start, score := findChirp(samples, tmpl, tmplEnergy, searchFrom)
		if start < 0 {
			break
		}
		// Patlamanın kuyruğunun sığıp sığmadığına burada bakılmaz: algılama
		// birkaç örnek geç kilitlenebilir ve tam-uzunluk kayıtta son
		// yinelemeyi elerdi; kesik yükü demodulate'in sınır denetimi eler.
		p, err := demodulate(samples, sr, start+chirpN+gapN, symN)
		if err == nil {
			out = append(out, Detection{Payload: p, StartSample: start, Score: score})
			searchFrom = start + burstN
		} else {
			// Yanlış tepe (yankı vb.): chirp'in yarısı kadar ilerleyip
			// aramayı sürdür.
			searchFrom = start + chirpN/2
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("beacon: kayıtta çözülebilir beacon bulunamadı")
	}
	return out, nil
}

// findChirp, kaba (4 örnek adımlı) normalize çapraz korelasyonla en güçlü
// chirp konumunu bulur, sonra ±8 örnek içinde inceltir. Dönen skor 0..1
// arasıdır; eşik altı (-1, 0) döner.
func findChirp(samples, tmpl []float64, tmplEnergy float64, from int) (int, float64) {
	const step = 4
	const threshold = 0.35 // yankılı/ gürültülü PA kaydında bile sağlam tepe

	bestAt, bestScore := -1, 0.0
	limit := len(samples) - len(tmpl)
	for at := from; at <= limit; at += step {
		if s := corrScore(samples, tmpl, tmplEnergy, at); s > bestScore {
			bestScore, bestAt = s, at
		}
	}
	if bestAt < 0 || bestScore < threshold {
		return -1, 0
	}
	for at := maxInt(from, bestAt-8); at <= minInt(limit, bestAt+8); at++ {
		if s := corrScore(samples, tmpl, tmplEnergy, at); s > bestScore {
			bestScore, bestAt = s, at
		}
	}
	return bestAt, bestScore
}

// corrScore, şablonla normalize korelasyon karesi döndürür (0..1).
func corrScore(samples, tmpl []float64, tmplEnergy float64, at int) float64 {
	var dot, energy float64
	for i, tv := range tmpl {
		sv := samples[at+i]
		dot += sv * tv
		energy += sv * sv
	}
	if energy == 0 {
		return 0
	}
	return dot * dot / (energy * tmplEnergy)
}

// demodulate, yük sembollerini Goertzel enerjileriyle bitlere çevirir ve
// yükü çözer. Sembolün ortadaki %70'i okunur (kenar yumuşatmaları ve küçük
// senkron kaymaları dışarıda kalsın).
func demodulate(samples []float64, sr float64, at, symN int) (Payload, error) {
	margin := symN * 15 / 100
	bits := make([]byte, PayloadBits)
	for s := 0; s < PayloadBits; s++ {
		lo := at + s*symN + margin
		hi := at + (s+1)*symN - margin
		if hi > len(samples) {
			return Payload{}, fmt.Errorf("beacon: kayıt yük ortasında bitiyor")
		}
		win := samples[lo:hi]
		if goertzel(win, sr, Bit1Hz) > goertzel(win, sr, Bit0Hz) {
			bits[s] = 1
		}
	}
	return parseBits(bits)
}

// goertzel, penceredeki hedef frekans enerjisini döndürür (FFT'siz tek bant
// sorgusu — telefonda da ucuzdur).
func goertzel(win []float64, sr, freq float64) float64 {
	w := 2 * math.Pi * freq / sr
	coeff := 2 * math.Cos(w)
	var s0, s1, s2 float64
	for _, v := range win {
		s0 = v + coeff*s1 - s2
		s2, s1 = s1, s0
	}
	return s1*s1 + s2*s2 - coeff*s1*s2
}

// highpass, RBJ biquad yüksek-geçiren süzgeçtir (Q = 0,707, Butterworth).
// Amaç keskinlik değil bant dışı enerjiyi bastırmaktır; katsayılar her
// çağrıda hesaplanır (çözüm sıklığı düşük).
func highpass(in []float64, sr, fc float64) []float64 {
	w0 := 2 * math.Pi * fc / sr
	cosw, sinw := math.Cos(w0), math.Sin(w0)
	alpha := sinw / (2 * 0.7071)
	b0 := (1 + cosw) / 2
	b1 := -(1 + cosw)
	b2 := (1 + cosw) / 2
	a0 := 1 + alpha
	a1 := -2 * cosw
	a2 := 1 - alpha
	b0, b1, b2, a1, a2 = b0/a0, b1/a0, b2/a0, a1/a0, a2/a0

	out := make([]float64, len(in))
	var x1, x2, y1, y2 float64
	for i, x := range in {
		y := b0*x + b1*x1 + b2*x2 - a1*y1 - a2*y2
		out[i] = y
		x2, x1 = x1, x
		y2, y1 = y1, y
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

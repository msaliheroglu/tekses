package beacon

import (
	"math"
	"math/rand/v2"
	"testing"
)

func testPayload() Payload {
	return Payload{Version: Version, CueIndex: 3, Seq: 7, CountdownMs: 2500}
}

// sabit tohumlu gürültü: test tekrarlanabilir kalsın.
func addNoise(samples []float64, amp float64, seed uint64) []float64 {
	rng := rand.New(rand.NewPCG(seed, seed^0xdead))
	out := make([]float64, len(samples))
	for i, v := range samples {
		out[i] = v + amp*(2*rng.Float64()-1)
	}
	return out
}

func embed(burst []float64, before, after int) []float64 {
	out := make([]float64, 0, before+len(burst)+after)
	out = append(out, make([]float64, before)...)
	out = append(out, burst...)
	return append(out, make([]float64, after)...)
}

func TestRoundTrip(t *testing.T) {
	for _, sr := range []int{48000, 44100} {
		burst, err := Encode(testPayload(), sr)
		if err != nil {
			t.Fatal(err)
		}
		rec := embed(burst, sr/3, sr/5) // patlama kaydın ortasında
		det, err := Decode(rec, sr)
		if err != nil {
			t.Fatalf("sr=%d: %v", sr, err)
		}
		if det.Payload != testPayload() {
			t.Fatalf("sr=%d: yük = %+v", sr, det.Payload)
		}
		// Önek konumu senkron sözleşmesidir: ±3 ms içinde bulunmalı.
		if diff := det.StartSample - sr/3; math.Abs(float64(diff)) > float64(sr)*0.003 {
			t.Fatalf("sr=%d: önek konumu %d örnek sapmış", sr, diff)
		}
	}
}

func TestNoisyAndAttenuated(t *testing.T) {
	const sr = 48000
	burst, err := Encode(testPayload(), sr)
	if err != nil {
		t.Fatal(err)
	}
	// PA + mesafe taklidi: sinyal 1/8'e düşmüş, geniş bant gürültü sinyalin
	// 2 katı genlikte (negatif SNR).
	weak := make([]float64, len(burst))
	for i, v := range burst {
		weak[i] = v / 8
	}
	rec := addNoise(embed(weak, sr/2, sr/2), 0.2, 42)
	det, err := Decode(rec, sr)
	if err != nil {
		t.Fatalf("gürültülü kayıt çözülemedi: %v", err)
	}
	if det.Payload != testPayload() {
		t.Fatalf("gürültülü yük = %+v", det.Payload)
	}
}

func TestMultipleBursts(t *testing.T) {
	const sr = 48000
	p1 := testPayload()
	p2 := p1
	p2.Seq = 8
	p2.CountdownMs = 1500
	b1, _ := Encode(p1, sr)
	b2, _ := Encode(p2, sr)
	rec := embed(b1, sr/4, sr/4)
	rec = append(rec, embed(b2, 0, sr/4)...)

	dets, err := DecodeAll(rec, sr, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(dets) != 2 || dets[0].Payload != p1 || dets[1].Payload != p2 {
		t.Fatalf("beklenmeyen çözümler: %+v", dets)
	}
}

// Kayıt tam patlamanın bitiminde sonlanıyorsa (kuyrukta sessizlik yok) son
// yineleme yine çözülmeli: algılama birkaç örnek geç kilitlenebilir ve katı
// bir "patlama sığmıyor" ön denetimi onu elerdi (regresyon).
func TestBurstAtRecordingEnd(t *testing.T) {
	const sr = 48000
	var total []float64
	for i := 0; i < 3; i++ {
		b, err := Encode(Payload{Version: Version, CueIndex: 2, Seq: i + 1, CountdownMs: 3000 - i*1000}, sr)
		if err != nil {
			t.Fatal(err)
		}
		total = append(total, make([]float64, i*sr-len(total))...)
		total = append(total, b...)
	}
	dets, err := DecodeAll(total, sr, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(dets) != 3 {
		t.Fatalf("3 beklenirken %d çözüldü: %+v", len(dets), dets)
	}
	if dets[2].Payload.Seq != 3 || dets[2].Payload.CountdownMs != 1000 {
		t.Fatalf("son yineleme yanlış: %+v", dets[2].Payload)
	}
}

func TestCorruptPayloadRejected(t *testing.T) {
	const sr = 48000
	burst, _ := Encode(testPayload(), sr)
	// Yükün ortasındaki iki sembolü sustur: CRC tutmamalı, çözüm olmamalı.
	symN := int(SymbolSec * float64(sr))
	chirpN := int(ChirpSec * float64(sr))
	gapN := int(GapSec * float64(sr))
	for i := 0; i < 2*symN; i++ {
		burst[chirpN+gapN+10*symN+i] = 0
	}
	if _, err := Decode(embed(burst, sr/4, sr/4), sr); err == nil {
		t.Fatal("bozuk yük çözülmemeliydi (CRC)")
	}
}

func TestSilenceRejected(t *testing.T) {
	if _, err := Decode(make([]float64, 48000), 48000); err == nil {
		t.Fatal("sessizlikte beacon bulunmamalıydı")
	}
}

// Oda yankısı: aynı taşıyıcıda peş peşe sembollerin kuyruğu, ardından gelen
// zıt sembolün başını maskeler (saha imzası: yalnız "111→0" geçişlerinde
// 0→1 hataları). Geç pencere profili bunu kurtarmalı. Yankı, çok vuruşlu
// gecikmeli kopyalarla taklit edilir.
func TestReverberantRecording(t *testing.T) {
	const sr = 48000
	// CRC'si "…1110 1110" gibi 1-koşusu→0 geçişleri içeren saha yükü.
	p := Payload{Version: Version, CueIndex: 0, Seq: 1, CountdownMs: 3000}
	burst, _ := Encode(p, sr)
	rec := embed(burst, sr/2, sr/2)
	for _, tap := range []struct {
		delayMs float64
		gain    float64
	}{{3, 0.5}, {7, 0.35}, {12, 0.25}, {18, 0.15}} {
		d := int(tap.delayMs * sr / 1000)
		for i := len(rec) - 1; i >= d; i-- {
			rec[i] += tap.gain * rec[i-d]
		}
	}
	det, err := Decode(rec, sr)
	if err != nil {
		t.Fatalf("yankılı kayıt çözülemedi: %v", err)
	}
	if det.Payload != p {
		t.Fatalf("yankılı yük = %+v", det.Payload)
	}
}

// Güçlü yansıma chirp kilidini TAM BİR SEMBOL (20 ms) kaydırabilir: bitler
// "emin ama yanlış" çıkar, yalnız CRC yakalar. ±20 ms yeniden denemesi bunu
// kurtarmalı (saha vakası: skor 0.81, karar payı 0.97, CRC sürekli ✗).
func TestWholeSymbolLockError(t *testing.T) {
	const sr = 48000
	burst, _ := Encode(testPayload(), sr)
	symN := int(SymbolSec * sr)
	// Kilit hatası taklidi: demodülasyona kasıtlı +1 sembol kaymış konum ver.
	chirpN := int(ChirpSec * float64(sr))
	gapN := int(GapSec * float64(sr))
	rec := embed(burst, sr/4, sr/2)
	if _, err := demodulateWithRetry(rec, sr, sr/4+chirpN+gapN+symN, symN); err != nil {
		t.Fatalf("+1 sembol kilit hatası kurtarılamadı: %v", err)
	}
	if _, err := demodulateWithRetry(rec, sr, sr/4+chirpN+gapN-symN, symN); err != nil {
		t.Fatalf("-1 sembol kilit hatası kurtarılamadı: %v", err)
	}
}

func TestAnalyze(t *testing.T) {
	const sr = 48000
	burst, _ := Encode(testPayload(), sr)
	rep, err := Analyze(embed(burst, sr/2, sr/2), sr)
	if err != nil {
		t.Fatal(err)
	}
	if rep.InBandRatio < 0.2 || !rep.CarrierSeen || rep.BestChirpScore < 0.35 {
		t.Fatalf("temiz beacon raporu zayıf: %+v", rep)
	}

	// Bant dışı içerik (1 kHz ton): bant oranı ~0, taşıyıcı yok.
	low := make([]float64, 2*sr)
	for i := range low {
		low[i] = 0.5 * math.Sin(2*math.Pi*1000*float64(i)/sr)
	}
	rep2, err := Analyze(low, sr)
	if err != nil {
		t.Fatal(err)
	}
	if rep2.InBandRatio > 0.001 || rep2.CarrierSeen || rep2.BestChirpScore >= 0.35 {
		t.Fatalf("bant dışı kayıt raporu yanlış: %+v", rep2)
	}
}

func TestPayloadValidation(t *testing.T) {
	bad := []Payload{
		{Version: 2, CueIndex: 0, Seq: 0, CountdownMs: 0},
		{Version: Version, CueIndex: 256, Seq: 0, CountdownMs: 0},
		{Version: Version, CueIndex: 0, Seq: 16, CountdownMs: 0},
		{Version: Version, CueIndex: 0, Seq: 0, CountdownMs: 1024 * CountdownRes},
	}
	for _, p := range bad {
		if _, err := Encode(p, 48000); err == nil {
			t.Fatalf("geçersiz yük kabul edildi: %+v", p)
		}
	}
	// 100 ms çözünürlük: 2570 ms → 2500 ms olarak taşınır (kayıp bilinçli).
	p := Payload{Version: Version, CueIndex: 1, Seq: 1, CountdownMs: 2570}
	burst, err := Encode(p, 48000)
	if err != nil {
		t.Fatal(err)
	}
	det, err := Decode(embed(burst, 4800, 4800), 48000)
	if err != nil {
		t.Fatal(err)
	}
	if det.Payload.CountdownMs != 2500 {
		t.Fatalf("geri sayım çözünürlüğü: %d", det.Payload.CountdownMs)
	}
}

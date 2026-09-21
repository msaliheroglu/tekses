// Package beacon, ultrasonik yedek tetik sinyalinin sözleşmesidir (F2.7).
//
// Amaç: WebSocket'i kopmuş telefonlara kueyi mekân hoparlöründen (PA)
// duyulmaz bantta ulaştırmak. Karar dokümanı §3: 18,5–20 kHz; chirp önek +
// küçük FSK yük (kue, sıra, geri sayım, CRC); ateşlemeden ~3 sn önce
// başlayıp yinelenir. Ses hızı ve yankı nedeniyle ±300 ms hata taşır —
// ERİŞİLEBİLİRLİK YEDEĞİDİR, hassasiyet kaynağı değildir (Cue Arbiter,
// WS kaynağı varken beacon'ı yok sayar).
//
// Sinyal biçimi (v1, örnekleme hızından bağımsız tanım):
//
//	önek   : 120 ms doğrusal chirp, 18500 → 19900 Hz (algılama + zaman senkronu)
//	boşluk :  10 ms sessizlik
//	yük    : 34 bit FSK, sembol 20 ms (50 baud); bit0 = 18600 Hz, bit1 = 19400 Hz
//
// Yük bit düzeni (MSB önce):
//
//	 4 bit  version   (1)
//	 8 bit  cue_index (0 = otomatik program; 1..N = manifestteki sekans sırası)
//	 4 bit  seq       (yineleme sayacı; telefon run tekilleştirmesinde kullanır)
//	10 bit  countdown (100 ms birimiyle ateşlemeye kalan süre; 0..102,3 sn)
//	 8 bit  CRC-8     (poly 0x07; önceki 26 bitin bayt paketlemesi üzerinden)
//
// Kue kimliği tel üzerindeki string değil KÜÇÜK SAYIDIR: beacon bant genişliği
// çok dar. Eşleme sözleşmesi: cue_index 0 = "program", i>0 = manifest
// sequences[i-1]. Geri sayım, çözüldüğü ANIN yerel saatine eklenir; önekin
// başlangıcına göre sembol konumu düzeltmesi Decode içinde yapılır.
package beacon

import (
	"fmt"
	"math"
)

const (
	Version = 1

	// Bant ve zamanlama (saniye/Hz cinsinden; örnek sayıları hıza göre türer).
	ChirpLowHz   = 18500.0
	ChirpHighHz  = 19900.0
	Bit0Hz       = 18600.0
	Bit1Hz       = 19400.0
	ChirpSec     = 0.120
	GapSec       = 0.010
	SymbolSec    = 0.020
	PayloadBits  = 34
	CountdownRes = 100 // ms / birim

	// Üretilen sinyalin genliği (tam ölçek 1.0'a göre): kırpılmadan yüksek.
	amplitude = 0.8
)

// Payload, beacon'ın taşıdığı bilgidir.
type Payload struct {
	Version     int
	CueIndex    int // 0 = program, 1..N = manifest sırası
	Seq         int // 0..15
	CountdownMs int // ateşlemeye kalan süre (100 ms çözünürlük, 0..102300)
}

// Validate, alan aralıklarını denetler.
func (p Payload) Validate() error {
	if p.Version != Version {
		return fmt.Errorf("beacon: desteklenmeyen version %d", p.Version)
	}
	if p.CueIndex < 0 || p.CueIndex > 255 {
		return fmt.Errorf("beacon: cue_index 0..255 aralığında olmalı")
	}
	if p.Seq < 0 || p.Seq > 15 {
		return fmt.Errorf("beacon: seq 0..15 aralığında olmalı")
	}
	if p.CountdownMs < 0 || p.CountdownMs > 1023*CountdownRes {
		return fmt.Errorf("beacon: countdown 0..%d ms aralığında olmalı", 1023*CountdownRes)
	}
	return nil
}

// bits, yükü MSB-önce 34 bite serer (CRC dahil).
func (p Payload) bits() []byte {
	units := p.CountdownMs / CountdownRes
	var b []byte
	push := func(v, n int) {
		for i := n - 1; i >= 0; i-- {
			b = append(b, byte((v>>i)&1))
		}
	}
	push(p.Version, 4)
	push(p.CueIndex, 8)
	push(p.Seq, 4)
	push(units, 10)
	push(int(crc8(packBits(b))), 8)
	return b
}

// parseBits, 34 biti çözer ve CRC'yi doğrular.
func parseBits(b []byte) (Payload, error) {
	if len(b) != PayloadBits {
		return Payload{}, fmt.Errorf("beacon: %d bit bekleniyordu", PayloadBits)
	}
	take := func(off, n int) int {
		v := 0
		for i := 0; i < n; i++ {
			v = v<<1 | int(b[off+i])
		}
		return v
	}
	got := byte(take(26, 8))
	if want := crc8(packBits(b[:26])); got != want {
		return Payload{}, fmt.Errorf("beacon: CRC uyuşmuyor")
	}
	p := Payload{
		Version:     take(0, 4),
		CueIndex:    take(4, 8),
		Seq:         take(12, 4),
		CountdownMs: take(16, 10) * CountdownRes,
	}
	if err := p.Validate(); err != nil {
		return Payload{}, err
	}
	return p, nil
}

// packBits, bit dizisini MSB-önce baytlara paketler (son bayt sıfır dolgulu).
func packBits(bits []byte) []byte {
	out := make([]byte, (len(bits)+7)/8)
	for i, bit := range bits {
		if bit != 0 {
			out[i/8] |= 1 << (7 - i%8)
		}
	}
	return out
}

// crc8, CRC-8/ATM'dir (poly 0x07, init 0x00) — tek baytlık, donanımsız
// ortamda (Dart dahil) üç satırda taşınabilir.
func crc8(data []byte) byte {
	var crc byte
	for _, d := range data {
		crc ^= d
		for i := 0; i < 8; i++ {
			if crc&0x80 != 0 {
				crc = crc<<1 ^ 0x07
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

// Encode, yükü verilen örnekleme hızında tek bir beacon patlamasına çevirir
// (önek + boşluk + FSK yükü). Örnekler [-1, 1] aralığındadır.
func Encode(p Payload, sampleRate int) ([]float64, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if sampleRate < 40000 {
		// Nyquist: 19,9 kHz taşımak için ≥ 40 kHz gerekir (44,1k ve 48k uyar).
		return nil, fmt.Errorf("beacon: örnekleme hızı en az 40 kHz olmalı (%d verildi)", sampleRate)
	}
	sr := float64(sampleRate)
	chirpN := int(ChirpSec * sr)
	gapN := int(GapSec * sr)
	symN := int(SymbolSec * sr)

	out := make([]float64, 0, chirpN+gapN+PayloadBits*symN)

	// Doğrusal chirp: faz sürekli tutulur ki hoparlörde çıtlama olmasın.
	phase := 0.0
	for i := 0; i < chirpN; i++ {
		t := float64(i) / float64(chirpN)
		freq := ChirpLowHz + (ChirpHighHz-ChirpLowHz)*t
		phase += 2 * math.Pi * freq / sr
		out = append(out, amplitude*envelope(i, chirpN)*math.Sin(phase))
	}
	out = append(out, make([]float64, gapN)...)

	for _, bit := range p.bits() {
		freq := Bit0Hz
		if bit != 0 {
			freq = Bit1Hz
		}
		for i := 0; i < symN; i++ {
			phase += 2 * math.Pi * freq / sr
			out = append(out, amplitude*envelope(i, symN)*math.Sin(phase))
		}
	}
	return out, nil
}

// envelope, parça başı/sonunda 2 ms'lik yumuşatma uygular (çıtlama ve bant
// dışı sıçrama olmasın); ortada 1'dir.
func envelope(i, n int) float64 {
	ramp := n / 10
	if r := 2 * n / 100; r > 0 && r < ramp {
		ramp = r
	}
	if ramp == 0 {
		return 1
	}
	switch {
	case i < ramp:
		return float64(i) / float64(ramp)
	case i >= n-ramp:
		return float64(n-1-i) / float64(ramp)
	default:
		return 1
	}
}

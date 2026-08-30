// Package clocksync, NTP benzeri örneklerden saat ofseti kestirimi yapar.
//
// Algoritma karar dokümanındaki gibidir ve Dart tarafındaki
// apps/participant/lib/core/clock_sync.dart ile birebir aynı tutulmalıdır:
// 8–12 örnek toplanır, RTT'ye göre en iyi yarı seçilir, ofsetlerin medyanı
// alınır. Ofset, "sunucu saati - istemci monoton saati" (ms) olarak tanımlıdır:
//
//	sunucuSaati ≈ istemciMonoton + ofset
package clocksync

import (
	"errors"
	"sort"
)

// Sample, tek bir saat senkronu değişiminin dört zaman damgasıdır (ms).
type Sample struct {
	T0 int64 // istek gönderildi (istemci monoton)
	T1 int64 // istek alındı (sunucu)
	T2 int64 // yanıt gönderildi (sunucu)
	T3 int64 // yanıt alındı (istemci monoton)
}

// RTT, örneğin ağda geçirdiği süredir; sunucuda geçen işlem süresi düşülür.
func (s Sample) RTT() int64 {
	return (s.T3 - s.T0) - (s.T2 - s.T1)
}

// Offset, bu tek örneğin ofset tahminidir.
func (s Sample) Offset() int64 {
	return ((s.T1 - s.T0) + (s.T2 - s.T3)) / 2
}

// Estimate, kestirim sonucudur.
type Estimate struct {
	// OffsetMs: sunucuSaati ≈ istemciMonoton + OffsetMs.
	OffsetMs int64
	// BestRTTMs: seçilen örnekler arasındaki en düşük RTT; kalite göstergesi.
	BestRTTMs int64
	// UsedSamples: medyana giren örnek sayısı.
	UsedSamples int
}

// ErrNoSamples, hiç örnek verilmediğinde döner.
var ErrNoSamples = errors.New("clocksync: örnek yok")

// Estimator, örnek biriktirir ve kestirim üretir. Eşzamanlı kullanım için
// güvenli değildir; çağıran tek goroutine'den kullanmalıdır.
type Estimator struct {
	samples []Sample
}

// Add, bir değişim örneği ekler. Negatif RTT'li (saat sıçraması, bozuk
// zaman damgası) örnekler atılır.
func (e *Estimator) Add(s Sample) {
	if s.RTT() < 0 {
		return
	}
	e.samples = append(e.samples, s)
}

// Len, biriken geçerli örnek sayısıdır.
func (e *Estimator) Len() int { return len(e.samples) }

// Reset, yeni bir senkron turu için örnekleri temizler.
func (e *Estimator) Reset() { e.samples = e.samples[:0] }

// Estimate, düşük RTT'li yarının ofset medyanını döndürür.
func (e *Estimator) Estimate() (Estimate, error) {
	n := len(e.samples)
	if n == 0 {
		return Estimate{}, ErrNoSamples
	}

	byRTT := make([]Sample, n)
	copy(byRTT, e.samples)
	sort.Slice(byRTT, func(i, j int) bool { return byRTT[i].RTT() < byRTT[j].RTT() })

	// En iyi yarı (en az 1): yüksek RTT'li örnekler asimetrik gecikme
	// taşıdığı için ofseti saptırır.
	keep := (n + 1) / 2
	chosen := byRTT[:keep]

	offsets := make([]int64, keep)
	for i, s := range chosen {
		offsets[i] = s.Offset()
	}
	sort.Slice(offsets, func(i, j int) bool { return offsets[i] < offsets[j] })

	var median int64
	if keep%2 == 1 {
		median = offsets[keep/2]
	} else {
		median = (offsets[keep/2-1] + offsets[keep/2]) / 2
	}

	return Estimate{
		OffsetMs:    median,
		BestRTTMs:   chosen[0].RTT(),
		UsedSamples: keep,
	}, nil
}

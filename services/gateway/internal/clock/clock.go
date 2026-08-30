// Package clock, gateway'in tek referans saatini sağlar.
//
// Zaman çizelgesi duvar saatine benzer bir eksende (Unix ms) yayınlanır ama
// süreç içinde daima monoton saatten türetilir: NTP düzeltmesi ya da elle
// saat ayarı gibi duvar saati sıçramaları yayın sırasında ekseni oynatamaz.
package clock

import "time"

// ServerClock, süreç başlangıcındaki duvar saatine sabitlenmiş monoton saattir.
type ServerClock struct {
	baseWallMs int64
	base       time.Time
}

// New, şimdiki duvar saatine sabitlenmiş bir saat kurar.
func New() *ServerClock {
	now := time.Now()
	return &ServerClock{baseWallMs: now.UnixMilli(), base: now}
}

// NowMs, sunucu saatini milisaniye olarak döndürür.
// time.Since monoton okuma kullanır; duvar saati sıçramalarından etkilenmez.
func (c *ServerClock) NowMs() int64 {
	return c.baseWallMs + time.Since(c.base).Milliseconds()
}

package main

// Asgari WAV G/Ç: 16-bit PCM. Yazım mono'dur; okuma mono/stereo ve herhangi
// bir örnekleme hızını kabul eder (telefon kayıt uygulamaları çoğunlukla
// 44,1/48 kHz stereo üretir; stereo kanallar ortalamayla mono'ya iner).

import (
	"encoding/binary"
	"fmt"
	"os"
)

func writeWAV(path string, samples []float64, sampleRate int) error {
	data := make([]byte, 2*len(samples))
	for i, v := range samples {
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		binary.LittleEndian.PutUint16(data[2*i:], uint16(int16(v*32767)))
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var hdr [44]byte
	copy(hdr[0:], "RIFF")
	binary.LittleEndian.PutUint32(hdr[4:], uint32(36+len(data)))
	copy(hdr[8:], "WAVE")
	copy(hdr[12:], "fmt ")
	binary.LittleEndian.PutUint32(hdr[16:], 16)
	binary.LittleEndian.PutUint16(hdr[20:], 1) // PCM
	binary.LittleEndian.PutUint16(hdr[22:], 1) // mono
	binary.LittleEndian.PutUint32(hdr[24:], uint32(sampleRate))
	binary.LittleEndian.PutUint32(hdr[28:], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(hdr[32:], 2)
	binary.LittleEndian.PutUint16(hdr[34:], 16)
	copy(hdr[36:], "data")
	binary.LittleEndian.PutUint32(hdr[40:], uint32(len(data)))
	if _, err := f.Write(hdr[:]); err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

// readWAV, kanalları AYRI AYRI döndürür (mono: 1 kanal). Stereo kanallar
// ortalamayla mono'ya İNDİRİLMEZ: iki mikrofonun ortalaması 19 kHz'te faz
// iptaliyle beacon'ı söndürebilir; çağıran, bandı güçlü kanalı seçer.
func readWAV(path string) (channels [][]float64, sampleRate int, err error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	if len(raw) < 12 || string(raw[0:4]) != "RIFF" || string(raw[8:12]) != "WAVE" {
		return nil, 0, fmt.Errorf("%s: WAV değil", path)
	}

	var numCh, bits int
	var data []byte
	// Parça (chunk) gezintisi: fmt ve data dışındakiler (LIST vb.) atlanır.
	for off := 12; off+8 <= len(raw); {
		id := string(raw[off : off+4])
		size := int(binary.LittleEndian.Uint32(raw[off+4 : off+8]))
		body := off + 8
		if body+size > len(raw) {
			size = len(raw) - body
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return nil, 0, fmt.Errorf("%s: bozuk fmt parçası", path)
			}
			if format := binary.LittleEndian.Uint16(raw[body:]); format != 1 {
				return nil, 0, fmt.Errorf("%s: yalnız 16-bit PCM desteklenir (format %d)", path, format)
			}
			numCh = int(binary.LittleEndian.Uint16(raw[body+2:]))
			sampleRate = int(binary.LittleEndian.Uint32(raw[body+4:]))
			bits = int(binary.LittleEndian.Uint16(raw[body+14:]))
		case "data":
			data = raw[body : body+size]
		}
		off = body + size + size%2 // parçalar 2 bayta hizalıdır
	}
	if sampleRate == 0 || data == nil {
		return nil, 0, fmt.Errorf("%s: fmt/data parçası yok", path)
	}
	if bits != 16 || numCh < 1 || numCh > 2 {
		return nil, 0, fmt.Errorf("%s: yalnız 16-bit mono/stereo desteklenir (%d bit, %d kanal)", path, bits, numCh)
	}

	frame := 2 * numCh
	n := len(data) / frame
	out := make([][]float64, numCh)
	for c := range out {
		out[c] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		for c := 0; c < numCh; c++ {
			v := int16(binary.LittleEndian.Uint16(data[i*frame+2*c:]))
			out[c][i] = float64(v) / 32768
		}
	}
	return out, sampleRate, nil
}

// Package manifest, gösteri paketinin sözleşmesini tanımlar.
//
// ShowVersion'ın içeriği bu yapının KANONİK JSON'udur: yayınlanırken bir kez
// üretilir, SHA-256 özeti alınır ve bir daha değişmez. Telefon paketi CDN'den
// indirir ve özetle doğrular (karar dokümanı §3: sürümlü, değişmez, hash
// doğrulamalı). Zamanlar sekans başına göre milisaniyedir; mutlak zaman
// (duvar saati) manifestte asla yer almaz — sekansın ne ZAMAN başlayacağını
// canlı kue ya da otomatik program söyler, manifest yalnızca İÇİNDE ne
// olacağını söyler.
package manifest

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
)

// FormatVersion, manifest biçiminin sürümüdür; alan ekleyen her kırıcı
// değişiklikte artar.
const FormatVersion = 1

const (
	// Işığa duyarlılık kuralı: sürekli yanıp sönme <= 3 Hz (tel tarafındaki
	// gateway sınırıyla aynı).
	MaxFlashHz = 3

	maxSequences        = 200
	maxLinesPerSequence = 2000
	maxCuesPerLane      = 2000
	maxSequenceDuration = 2 * 60 * 60 * 1000 // 2 saat (ms)
)

// Kue şeridi türleri.
const (
	LaneScreen = "screen"
	LaneTorch  = "torch"
	LaneAudio  = "audio"
)

var colorRe = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)

type Manifest struct {
	FormatVersion int        `json:"format_version"`
	Title         string     `json:"title"`
	Sequences     []Sequence `json:"sequences"`
}

type Sequence struct {
	ID         string      `json:"id"`
	Title      string      `json:"title"`
	DurationMs int         `json:"duration_ms"`
	LyricLines []LyricLine `json:"lyric_lines,omitempty"`
	CueLanes   []CueLane   `json:"cue_lanes,omitempty"`
}

type LyricLine struct {
	AtMs       int    `json:"at_ms"`
	DurationMs int    `json:"duration_ms"`
	Text       string `json:"text"`
}

type CueLane struct {
	ID   string `json:"id"`
	Kind string `json:"kind"` // screen | torch | audio
	Cues []Cue  `json:"cues"`
}

type Cue struct {
	AtMs       int    `json:"at_ms"`
	DurationMs int    `json:"duration_ms"`
	Color      string `json:"color,omitempty"`    // screen: #RRGGBB
	FlashHz    int    `json:"flash_hz,omitempty"` // screen/torch: 0..3
	AssetID    string `json:"asset_id,omitempty"` // audio: zorunlu
}

// Parse, ham JSON'u katı biçimde çözer (bilinmeyen alan hatadır: yazım
// hatası sahada sessizce yutulmasın) ve doğrular.
func Parse(raw []byte) (Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return Manifest{}, fmt.Errorf("manifest çözülemedi: %w", err)
	}
	if dec.More() {
		return Manifest{}, fmt.Errorf("manifest tek JSON nesnesi olmalı")
	}
	if m.FormatVersion == 0 {
		m.FormatVersion = FormatVersion
	}
	if err := m.Validate(); err != nil {
		return Manifest{}, err
	}
	return m, nil
}

// Validate, yapısal ve içerik kurallarını denetler.
func (m Manifest) Validate() error {
	if m.FormatVersion != FormatVersion {
		return fmt.Errorf("desteklenmeyen format_version %d (beklenen %d)", m.FormatVersion, FormatVersion)
	}
	if m.Title == "" {
		return fmt.Errorf("title boş olamaz")
	}
	if len(m.Sequences) == 0 {
		return fmt.Errorf("en az bir sekans gerekli")
	}
	if len(m.Sequences) > maxSequences {
		return fmt.Errorf("sekans sayısı %d üst sınırı aşıyor", maxSequences)
	}

	seqIDs := map[string]bool{}
	for i, seq := range m.Sequences {
		where := fmt.Sprintf("sequences[%d]", i)
		if seq.ID == "" {
			return fmt.Errorf("%s: id boş olamaz", where)
		}
		if seqIDs[seq.ID] {
			return fmt.Errorf("%s: id %q tekrar ediyor", where, seq.ID)
		}
		seqIDs[seq.ID] = true
		if seq.DurationMs <= 0 || seq.DurationMs > maxSequenceDuration {
			return fmt.Errorf("%s: duration_ms 1..%d aralığında olmalı", where, maxSequenceDuration)
		}
		if len(seq.LyricLines) > maxLinesPerSequence {
			return fmt.Errorf("%s: söz satırı sayısı üst sınırı aşıyor", where)
		}
		for j, line := range seq.LyricLines {
			if line.Text == "" {
				return fmt.Errorf("%s.lyric_lines[%d]: text boş olamaz", where, j)
			}
			if line.AtMs < 0 || line.DurationMs < 0 || line.AtMs > seq.DurationMs {
				return fmt.Errorf("%s.lyric_lines[%d]: zamanlama sekansın dışında", where, j)
			}
		}

		laneIDs := map[string]bool{}
		for j, lane := range seq.CueLanes {
			laneWhere := fmt.Sprintf("%s.cue_lanes[%d]", where, j)
			if lane.ID == "" {
				return fmt.Errorf("%s: id boş olamaz", laneWhere)
			}
			if laneIDs[lane.ID] {
				return fmt.Errorf("%s: id %q tekrar ediyor", laneWhere, lane.ID)
			}
			laneIDs[lane.ID] = true
			if lane.Kind != LaneScreen && lane.Kind != LaneTorch && lane.Kind != LaneAudio {
				return fmt.Errorf("%s: kind %q geçersiz (screen|torch|audio)", laneWhere, lane.Kind)
			}
			if len(lane.Cues) > maxCuesPerLane {
				return fmt.Errorf("%s: kue sayısı üst sınırı aşıyor", laneWhere)
			}
			for k, cue := range lane.Cues {
				if err := validateCue(lane.Kind, cue, seq.DurationMs); err != nil {
					return fmt.Errorf("%s.cues[%d]: %w", laneWhere, k, err)
				}
			}
		}
	}
	return nil
}

func validateCue(kind string, c Cue, seqDurationMs int) error {
	if c.AtMs < 0 || c.AtMs > seqDurationMs {
		return fmt.Errorf("at_ms sekansın dışında")
	}
	if c.DurationMs < 0 {
		return fmt.Errorf("duration_ms negatif olamaz")
	}
	switch kind {
	case LaneScreen:
		if !colorRe.MatchString(c.Color) {
			return fmt.Errorf("screen kuesi için color #RRGGBB biçiminde olmalı")
		}
		if c.FlashHz < 0 || c.FlashHz > MaxFlashHz {
			return fmt.Errorf("flash_hz 0..%d aralığında olmalı (ışığa duyarlılık sınırı)", MaxFlashHz)
		}
		if c.AssetID != "" {
			return fmt.Errorf("screen kuesi asset_id taşıyamaz")
		}
	case LaneTorch:
		if c.FlashHz < 0 || c.FlashHz > MaxFlashHz {
			return fmt.Errorf("flash_hz 0..%d aralığında olmalı (ışığa duyarlılık sınırı)", MaxFlashHz)
		}
		if c.Color != "" || c.AssetID != "" {
			return fmt.Errorf("torch kuesi color/asset_id taşıyamaz")
		}
	case LaneAudio:
		if c.AssetID == "" {
			return fmt.Errorf("audio kuesi için asset_id zorunlu")
		}
		if c.Color != "" || c.FlashHz != 0 {
			return fmt.Errorf("audio kuesi color/flash_hz taşıyamaz")
		}
	}
	return nil
}

// Canonical, manifestin kanonik baytlarını ve SHA-256 özetini (hex) üretir.
// Kanoniklik, Go struct alan sırasıyla sabittir: aynı içerik daima aynı
// baytları, dolayısıyla aynı özeti verir.
func (m Manifest) Canonical() ([]byte, string, error) {
	data, err := json.Marshal(m)
	if err != nil {
		return nil, "", fmt.Errorf("manifest kodlanamadı: %w", err)
	}
	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}

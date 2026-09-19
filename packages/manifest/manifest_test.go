package manifest

import (
	"strings"
	"testing"
)

const validManifest = `{
  "title": "Marş Seti",
  "sequences": [
    {
      "id": "seq-1",
      "title": "Açılış",
      "duration_ms": 60000,
      "lyric_lines": [
        {"at_ms": 0, "duration_ms": 4000, "text": "Hep beraber!"}
      ],
      "cue_lanes": [
        {"id": "ekran", "kind": "screen", "cues": [
          {"at_ms": 0, "duration_ms": 4000, "color": "#FF2A2A", "flash_hz": 2}
        ]},
        {"id": "fener", "kind": "torch", "cues": [
          {"at_ms": 1000, "duration_ms": 2000, "flash_hz": 1}
        ]},
        {"id": "ses", "kind": "audio", "cues": [
          {"at_ms": 0, "duration_ms": 60000, "asset_id": "asset_abc"}
        ]}
      ]
    }
  ]
}`

func TestParseValid(t *testing.T) {
	m, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatalf("geçerli manifest reddedildi: %v", err)
	}
	if m.FormatVersion != FormatVersion {
		t.Fatalf("format_version %d olarak doldurulmalıydı, %d", FormatVersion, m.FormatVersion)
	}
}

func TestCanonicalDeterministic(t *testing.T) {
	m1, err := Parse([]byte(validManifest))
	if err != nil {
		t.Fatal(err)
	}
	// Aynı içerik farklı boşluklama ve format_version'ın açıkça yazılmış
	// haliyle gelse de kanonik baytlar ve özet aynı kalmalı.
	compact := strings.Join(strings.Fields(validManifest), " ")
	explicit := strings.Replace(compact, `{ "title"`, `{"format_version":1,"title"`, 1)
	m2, err := Parse([]byte(explicit))
	if err != nil {
		t.Fatal(err)
	}

	c1, h1, err := m1.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	c2, h2, err := m2.Canonical()
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 || string(c1) != string(c2) {
		t.Fatalf("aynı içerik farklı kanonik biçim verdi:\n%s\n%s", h1, h2)
	}
	if len(h1) != 64 {
		t.Fatalf("SHA-256 hex uzunluğu = %d", len(h1))
	}
}

func TestProgramValidation(t *testing.T) {
	base := `{"title":"X","sequences":[
	  {"id":"a","title":"t","duration_ms":10000},
	  {"id":"b","title":"t","duration_ms":5000}],
	  "program":%s}`

	// Geçerli: bitişik ve aralıklı öğeler.
	ok := `[{"sequence_id":"a","at_offset_ms":0},{"sequence_id":"b","at_offset_ms":10000}]`
	if _, err := Parse([]byte(strings.Replace(base, "%s", ok, 1))); err != nil {
		t.Fatalf("geçerli program reddedildi: %v", err)
	}

	bad := map[string]string{
		"olmayan sekans": `[{"sequence_id":"yok","at_offset_ms":0}]`,
		"negatif ofset":  `[{"sequence_id":"a","at_offset_ms":-1}]`,
		"üst üste binme": `[{"sequence_id":"a","at_offset_ms":0},{"sequence_id":"b","at_offset_ms":9999}]`,
		"sırasız":        `[{"sequence_id":"b","at_offset_ms":8000},{"sequence_id":"a","at_offset_ms":0}]`,
	}
	for name, prog := range bad {
		if _, err := Parse([]byte(strings.Replace(base, "%s", prog, 1))); err == nil {
			t.Errorf("%s: kabul edildi, reddedilmeliydi", name)
		}
	}

	// "program" sekans kimliği olarak ayrılmıştır.
	reserved := `{"title":"X","sequences":[{"id":"program","title":"t","duration_ms":1000}]}`
	if _, err := Parse([]byte(reserved)); err == nil {
		t.Error("ayrılmış sekans kimliği kabul edildi")
	}
}

func TestParseRejections(t *testing.T) {
	cases := map[string]string{
		"bilinmeyen alan":      `{"title":"X","yanlis_alan":1,"sequences":[{"id":"a","title":"t","duration_ms":1000}]}`,
		"sekanssız":            `{"title":"X","sequences":[]}`,
		"boş başlık":           `{"title":"","sequences":[{"id":"a","title":"t","duration_ms":1000}]}`,
		"çift sekans id":       `{"title":"X","sequences":[{"id":"a","title":"t","duration_ms":1000},{"id":"a","title":"t","duration_ms":1000}]}`,
		"negatif süre":         `{"title":"X","sequences":[{"id":"a","title":"t","duration_ms":-5}]}`,
		"yüksek flash_hz":      `{"title":"X","sequences":[{"id":"a","title":"t","duration_ms":1000,"cue_lanes":[{"id":"l","kind":"screen","cues":[{"at_ms":0,"color":"#FFFFFF","flash_hz":8}]}]}]}`,
		"bozuk renk":           `{"title":"X","sequences":[{"id":"a","title":"t","duration_ms":1000,"cue_lanes":[{"id":"l","kind":"screen","cues":[{"at_ms":0,"color":"kirmizi"}]}]}]}`,
		"asset'siz ses kuesi":  `{"title":"X","sequences":[{"id":"a","title":"t","duration_ms":1000,"cue_lanes":[{"id":"l","kind":"audio","cues":[{"at_ms":0}]}]}]}`,
		"geçersiz şerit türü":  `{"title":"X","sequences":[{"id":"a","title":"t","duration_ms":1000,"cue_lanes":[{"id":"l","kind":"lazer","cues":[]}]}]}`,
		"sekans dışı kue":      `{"title":"X","sequences":[{"id":"a","title":"t","duration_ms":1000,"cue_lanes":[{"id":"l","kind":"torch","cues":[{"at_ms":5000}]}]}]}`,
		"torch kuesinde renk":  `{"title":"X","sequences":[{"id":"a","title":"t","duration_ms":1000,"cue_lanes":[{"id":"l","kind":"torch","cues":[{"at_ms":0,"color":"#FFFFFF"}]}]}]}`,
		"boş söz metni":        `{"title":"X","sequences":[{"id":"a","title":"t","duration_ms":1000,"lyric_lines":[{"at_ms":0,"text":""}]}]}`,
		"yanlış format sürümü": `{"format_version":9,"title":"X","sequences":[{"id":"a","title":"t","duration_ms":1000}]}`,
	}
	for name, raw := range cases {
		if _, err := Parse([]byte(raw)); err == nil {
			t.Errorf("%s: kabul edildi, reddedilmeliydi", name)
		}
	}
}

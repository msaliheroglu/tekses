package api

// Deneysel: ses varlığından zamanlı söz taslağı çıkarma.
//
// Çözümleme dış bir komuta devredilir (TEKSES_TRANSCRIBER): komut, tek
// argüman olarak ses dosyasının yolunu alır ve stdout'a şu JSON'u basar:
//
//	{"segments": [{"start_ms": 1200, "end_ms": 4000, "text": "..."}]}
//
// deploy/transcribe-whisper.sh, whisper.cpp'yi bu sözleşmeye uyarlar.
// Şarkılarda konuşma tanıma hata payı yüksektir (enstrüman vokali bastırır);
// çıktı TASLAKTIR, moderatör panelde düzeltir. En isabetli yol hâlâ LRC
// içe aktarma ya da elle zamanlamadır. İşler bellekte tutulur; süreç
// yeniden başlarsa devam eden işler kaybolur (deneysel özellik için kabul).

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/msaliheroglu/tekses/packages/manifest"
	"github.com/msaliheroglu/tekses/services/control-api/internal/model"
)

const defaultTranscribeTimeout = 10 * time.Minute

// transcribeTimeout: yavaş CPU'larda (ör. ücretsiz ARM VM) uzun şarkılar
// varsayılana sığmayabilir; TEKSES_TRANSCRIBE_TIMEOUT ("30m" gibi) ile aşılır.
func transcribeTimeout() time.Duration {
	if v := os.Getenv("TEKSES_TRANSCRIBE_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return defaultTranscribeTimeout
}

type transcriptionJob struct {
	ID     string
	OrgID  string
	Status string // running | done | error
	Lines  []manifest.LyricLine
	Error  string
}

type transcriberOutput struct {
	Segments []struct {
		StartMs int64  `json:"start_ms"`
		EndMs   int64  `json:"end_ms"`
		Text    string `json:"text"`
	} `json:"segments"`
}

func (s *Server) handleTranscribeAsset(w http.ResponseWriter, r *http.Request, sess model.Session) {
	if s.transcriber == "" {
		writeErr(w, http.StatusNotImplemented,
			"otomatik söz çıkarma bu sunucuda yapılandırılmamış (TEKSES_TRANSCRIBER); LRC içe aktarmayı kullanın")
		return
	}
	name := r.PathValue("name")
	if !assetNameRe.MatchString(name) {
		writeErr(w, http.StatusNotFound, "varlık bulunamadı")
		return
	}
	exists, err := s.packages.Exists(r.Context(), name)
	if err != nil || !exists {
		writeErr(w, http.StatusNotFound, "varlık bulunamadı")
		return
	}

	job := &transcriptionJob{ID: newID("tr"), OrgID: sess.OrgID, Status: "running"}
	s.trMu.Lock()
	s.trJobs[job.ID] = job
	s.trMu.Unlock()

	go s.runTranscription(job, name)

	s.log.Info("söz çıkarma başladı", "iş", job.ID, "varlık", name)
	writeJSON(w, http.StatusAccepted, map[string]any{"transcription_id": job.ID, "status": job.Status})
}

func (s *Server) runTranscription(job *transcriptionJob, assetID string) {
	fail := func(msg string) {
		s.trMu.Lock()
		job.Status = "error"
		job.Error = msg
		s.trMu.Unlock()
		s.log.Warn("söz çıkarma başarısız", "iş", job.ID, "hata", msg)
	}

	ctx, cancel := context.WithTimeout(context.Background(), transcribeTimeout())
	defer cancel()

	data, err := s.packages.Get(ctx, assetID)
	if err != nil {
		fail("varlık okunamadı")
		return
	}
	tmp, err := os.CreateTemp("", "tekses-tr-*."+assetID[strings.LastIndexByte(assetID, '.')+1:])
	if err != nil {
		fail("geçici dosya açılamadı")
		return
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		fail("geçici dosya yazılamadı")
		return
	}
	_ = tmp.Close()

	out, err := exec.CommandContext(ctx, s.transcriber, tmp.Name()).Output()
	if err != nil {
		// Betiğin stderr'i teşhisin kendisidir (ffmpeg yok, model yok…);
		// kullanıcıya son satırlarıyla birlikte gösterilir.
		msg := fmt.Sprintf("çözümleyici komutu başarısız: %v", err)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
			stderr := strings.TrimSpace(string(exitErr.Stderr))
			if len(stderr) > 500 {
				stderr = "…" + stderr[len(stderr)-500:]
			}
			msg += " — " + stderr
		}
		fail(msg)
		return
	}
	var parsed transcriberOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		fail("çözümleyici çıktısı JSON değil")
		return
	}

	sort.Slice(parsed.Segments, func(i, j int) bool {
		return parsed.Segments[i].StartMs < parsed.Segments[j].StartMs
	})
	lines := make([]manifest.LyricLine, 0, len(parsed.Segments))
	for _, seg := range parsed.Segments {
		text := strings.TrimSpace(seg.Text)
		if text == "" || seg.StartMs < 0 || seg.EndMs < seg.StartMs {
			continue
		}
		if isNonSpeechAnnotation(text) {
			continue
		}
		lines = append(lines, manifest.LyricLine{
			AtMs:       int(seg.StartMs),
			DurationMs: int(seg.EndMs - seg.StartMs),
			Text:       text,
		})
	}

	s.trMu.Lock()
	job.Status = "done"
	job.Lines = lines
	s.trMu.Unlock()
	s.log.Info("söz çıkarma bitti", "iş", job.ID, "satır", len(lines))
}

// isNonSpeechAnnotation, Whisper'ın konuşma dışı etiketlerini ayıklar:
// "[MÜZİK ÇALIYOR]", "(alkış)", "♪ ♪" gibi bölümler söz satırı değildir —
// karaoke ekranında o aralık boş kalmalıdır.
func isNonSpeechAnnotation(text string) bool {
	if strings.HasPrefix(text, "[") && strings.HasSuffix(text, "]") {
		return true
	}
	if strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		return true
	}
	if strings.HasPrefix(text, "*") && strings.HasSuffix(text, "*") {
		return true
	}
	// Yalnızca nota işareti ve noktalamadan oluşan satırlar.
	stripped := strings.Map(func(r rune) rune {
		switch r {
		case '♪', '♫', '.', ',', '-', ' ':
			return -1
		}
		return r
	}, text)
	return stripped == ""
}

func (s *Server) handleGetTranscription(w http.ResponseWriter, r *http.Request, sess model.Session) {
	s.trMu.Lock()
	job, ok := s.trJobs[r.PathValue("id")]
	var snapshot transcriptionJob
	if ok {
		snapshot = *job
	}
	s.trMu.Unlock()
	// Başka kiracının işi "yok" görünür.
	if !ok || snapshot.OrgID != sess.OrgID {
		writeErr(w, http.StatusNotFound, "iş bulunamadı")
		return
	}
	resp := map[string]any{"transcription_id": snapshot.ID, "status": snapshot.Status}
	if snapshot.Status == "done" {
		resp["lyric_lines"] = snapshot.Lines
	}
	if snapshot.Status == "error" {
		resp["error"] = snapshot.Error
	}
	writeJSON(w, http.StatusOK, resp)
}

// TekSes yük üreteci — Faz 0 sürümü.
//
// N istemciyi gateway'e bağlar, her biri bağımsız saat senkronu yapar,
// ardından gelen kueyi bekler ve kendi ofsetine göre yerel ateşleme anını
// hesaplar. Tüm istemciler aynı süreçte (yani aynı gerçek saatte) yaşadığı
// için istemciler arası ateşleme anı YAYILIMI, protokolün senkron hatasını
// doğrudan ölçer. -jitter ile dengesiz hücresel ağın asimetrik gecikmesi
// taklit edilebilir.
//
// Bu araç ağ/radyo gerçekliğini değil protokol ve kestirim doğruluğunu
// ölçer; gerçek ölçüm 5–10 telefon + 240 fps kamera ile yapılır
// (docs/faz0-senkron-denemesi.md). Faz 2'de 100k istemcilik yük testine
// evrilecek.
//
// Kullanım:
//
//	go run ./tools/loadgen -n 50 -server ws://localhost:8080/ws -cue -jitter 40
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/msaliheroglu/tekses/packages/clocksync"
	"github.com/msaliheroglu/tekses/packages/proto/wire"
)

var processStart = time.Now()

// monoMs, tüm istemcilerin paylaştığı gerçek monoton saattir.
func monoMs() int64 { return time.Since(processStart).Milliseconds() }

type clientResult struct {
	id          int
	est         clocksync.Estimate
	runID       string
	fireLocalMs int64 // kendi ofsetine göre hesapladığı yerel ateşleme anı
	err         error
}

func main() {
	server := flag.String("server", "ws://localhost:8080/ws", "gateway WebSocket adresi")
	n := flag.Int("n", 25, "istemci sayısı")
	samples := flag.Int("samples", 10, "istemci başına saat senkronu örneği")
	sampleInterval := flag.Duration("sampleInterval", 40*time.Millisecond, "örnekler arası bekleme")
	jitter := flag.Int64("jitter", 0, "yön başına 0..N ms rastgele yapay gecikme (ağ taklidi)")
	cue := flag.Bool("cue", false, "tüm istemciler senkron olunca kueyi kendisi tetiklesin")
	cueDelay := flag.Int64("cueDelay", 2000, "-cue ile tetiklenen kuenin gecikmesi (ms)")
	adminToken := flag.String("adminToken", "", "-cue için yönetici token'ı (varsa)")
	waitCue := flag.Duration("waitCue", 60*time.Second, "kue bekleme süresi")
	flag.Parse()

	results := make([]clientResult, *n)
	var synced sync.WaitGroup
	var done sync.WaitGroup

	for i := 0; i < *n; i++ {
		synced.Add(1)
		done.Add(1)
		go func(id int) {
			defer done.Done()
			results[id] = runClient(id, *server, *samples, *sampleInterval, *jitter, *waitCue, synced.Done)
		}(i)
	}

	synced.Wait()
	fmt.Printf("%d istemci bağlandı ve saat senkronu tamamlandı.\n", *n)

	if *cue {
		if err := triggerCue(*server, *cueDelay, *adminToken); err != nil {
			fmt.Fprintf(os.Stderr, "kue tetiklenemedi: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("kue tetiklendi (fireAt = şimdi + %d ms), yanıtlar bekleniyor...\n", *cueDelay)
	} else {
		fmt.Println("kue bekleniyor (curl ile POST /api/v0/cue tetikleyin)...")
	}

	done.Wait()
	report(results)
}

// runClient tek bir simüle katılımcıdır. Senkron bitince onSynced çağrılır;
// dönen sonuç kue alımını da içerir.
func runClient(id int, server string, samples int, sampleInterval time.Duration, jitter int64, waitCue time.Duration, onSynced func()) clientResult {
	syncedOnce := sync.OnceFunc(onSynced)
	defer syncedOnce()
	res := clientResult{id: id}

	conn, _, err := websocket.DefaultDialer.Dial(server, nil)
	if err != nil {
		res.err = fmt.Errorf("bağlantı: %w", err)
		return res
	}
	defer conn.Close()

	send := func(msgType string, msg any) error {
		data, err := wire.Encode(msgType, msg)
		if err != nil {
			return err
		}
		return conn.WriteMessage(websocket.TextMessage, data)
	}

	if err := send(wire.TypeHello, wire.Hello{ProtocolVersion: wire.ProtocolVersion, ClientKind: "loadgen"}); err != nil {
		res.err = fmt.Errorf("hello: %w", err)
		return res
	}

	readEnvelope := func(deadline time.Time) (wire.Envelope, error) {
		_ = conn.SetReadDeadline(deadline)
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return wire.Envelope{}, err
		}
		return wire.Decode(raw)
	}

	// welcome
	if env, err := readEnvelope(time.Now().Add(10 * time.Second)); err != nil || env.Type != wire.TypeWelcome {
		res.err = fmt.Errorf("welcome beklenirken: tür=%q hata=%v", env.Type, err)
		return res
	}

	// Saat senkronu turu. Yapay gecikme, t0 alındıktan sonra (gidiş) ve
	// çerçeve okunduktan sonra (dönüş) uyunarak asimetrik ağ gecikmesini
	// taklit eder; kestirici düşük RTT'li örnekleri seçerek bununla başa
	// çıkmak zorundadır.
	var est clocksync.Estimator
	for seq := uint32(1); int(seq) <= samples; seq++ {
		t0 := monoMs()
		sleepJitter(jitter)
		if err := send(wire.TypeClockSyncRequest, wire.ClockSyncRequest{Seq: seq, ClientMonoMs: t0}); err != nil {
			res.err = fmt.Errorf("senkron isteği: %w", err)
			return res
		}
		for {
			env, err := readEnvelope(time.Now().Add(10 * time.Second))
			if err != nil {
				res.err = fmt.Errorf("senkron yanıtı: %w", err)
				return res
			}
			if env.Type != wire.TypeClockSyncResponse {
				continue // erken gelen kue vb. bu turda yok sayılır
			}
			var resp wire.ClockSyncResponse
			if err := json.Unmarshal(env.Data, &resp); err != nil || resp.Seq != seq {
				continue
			}
			sleepJitter(jitter)
			est.Add(clocksync.Sample{T0: resp.ClientMonoMs, T1: resp.ServerRecvMs, T2: resp.ServerSendMs, T3: monoMs()})
			break
		}
		time.Sleep(sampleInterval)
	}

	res.est, err = est.Estimate()
	if err != nil {
		res.err = err
		return res
	}
	syncedOnce()

	// Kue bekle; tekrarlar run_id ile tekilleştirilir, ilki esas alınır.
	cueDeadline := time.Now().Add(waitCue)
	for {
		env, err := readEnvelope(cueDeadline)
		if err != nil {
			res.err = fmt.Errorf("kue beklenirken: %w", err)
			return res
		}
		if env.Type != wire.TypeCueStart {
			continue
		}
		var cue wire.CueStart
		if err := json.Unmarshal(env.Data, &cue); err != nil {
			continue
		}
		res.runID = cue.RunID
		// sunucuSaati ≈ yerelMonoton + ofset  ⇒  yerelAteşleme = fireAt − ofset
		res.fireLocalMs = cue.FireAtServerMs - res.est.OffsetMs
		return res
	}
}

func sleepJitter(jitterMs int64) {
	if jitterMs > 0 {
		time.Sleep(time.Duration(rand.Int64N(jitterMs+1)) * time.Millisecond)
	}
}

// triggerCue, ws://host/ws adresinden http://host/api/v0/cue adresini türetip
// kue tetikler.
func triggerCue(server string, delayMs int64, adminToken string) error {
	u, err := url.Parse(server)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "ws":
		u.Scheme = "http"
	case "wss":
		u.Scheme = "https"
	}
	u.Path = "/api/v0/cue"

	body, _ := json.Marshal(map[string]any{"delayMs": delayMs, "cue_id": "loadgen-olcum"})
	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if adminToken != "" {
		req.Header.Set("Authorization", "Bearer "+adminToken)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(resp.Body)
		return fmt.Errorf("durum %d: %s", resp.StatusCode, strings.TrimSpace(buf.String()))
	}
	return nil
}

func report(results []clientResult) {
	var ok []clientResult
	for _, r := range results {
		if r.err != nil {
			fmt.Fprintf(os.Stderr, "istemci %d hata: %v\n", r.id, r.err)
			continue
		}
		ok = append(ok, r)
	}
	if len(ok) == 0 {
		fmt.Println("hiçbir istemci kue alamadı.")
		os.Exit(1)
	}

	fires := make([]float64, len(ok))
	offsets := make([]float64, len(ok))
	rtts := make([]float64, len(ok))
	for i, r := range ok {
		fires[i] = float64(r.fireLocalMs)
		offsets[i] = float64(r.est.OffsetMs)
		rtts[i] = float64(r.est.BestRTTMs)
	}
	sort.Float64s(fires)
	sort.Float64s(offsets)
	sort.Float64s(rtts)

	spread := fires[len(fires)-1] - fires[0]
	fmt.Println()
	fmt.Println("=== Faz 0 yazılım içi senkron ölçümü ===")
	fmt.Printf("istemci: %d başarılı / %d toplam (run_id %s)\n", len(ok), len(results), ok[0].runID)
	fmt.Printf("ofset kestirimi  : min %.0f ms, medyan %.0f ms, maks %.0f ms\n", offsets[0], percentile(offsets, 50), offsets[len(offsets)-1])
	fmt.Printf("en iyi RTT       : medyan %.0f ms, p95 %.0f ms\n", percentile(rtts, 50), percentile(rtts, 95))
	fmt.Printf("ateşleme yayılımı: maks−min %.0f ms, p95−p5 %.0f ms, σ %.1f ms\n",
		spread, percentile(fires, 95)-percentile(fires, 5), stddev(fires))
	if spread <= 30 {
		fmt.Println("sonuç: ≤30 ms hedefi bu koşulda TUTUYOR ✓")
	} else {
		fmt.Println("sonuç: ≤30 ms hedefi bu koşulda TUTMUYOR ✗ (jitter/örnek sayısını inceleyin)")
	}
}

func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return math.NaN()
	}
	idx := p / 100 * float64(len(sorted)-1)
	lo := int(math.Floor(idx))
	hi := int(math.Ceil(idx))
	if lo == hi {
		return sorted[lo]
	}
	frac := idx - float64(lo)
	return sorted[lo]*(1-frac) + sorted[hi]*frac
}

func stddev(xs []float64) float64 {
	if len(xs) < 2 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(len(xs))
	var sq float64
	for _, x := range xs {
		sq += (x - mean) * (x - mean)
	}
	return math.Sqrt(sq / float64(len(xs)-1))
}

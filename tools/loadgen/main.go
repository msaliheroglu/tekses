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
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

	"github.com/msaliheroglu/tekses/packages/clocksync"
	"github.com/msaliheroglu/tekses/packages/proto/wire"
)

var processStart = time.Now()

// monoMs, tüm istemcilerin paylaştığı gerçek monoton saattir.
func monoMs() int64 { return time.Since(processStart).Milliseconds() }

type clientResult struct {
	id            int
	est           clocksync.Estimate
	runID         string
	fireLocalMs   int64 // kendi ofsetine göre hesapladığı yerel ateşleme anı
	cueFrameBytes int   // alınan ilk kue çerçevesinin tel boyutu
	resyncMs      int64 // fırtına üyesiyse: kopuştan yeniden senkrona geçen süre
	err           error
}

// stormCfg, yeniden bağlanma fırtınası senaryosudur (F3.4): stadyumda ağ bir
// an kesilip gelince on binlerce telefon AYNI saniyede geri bağlanır. Üye
// istemciler senkrondan sonra bağlantıyı koparır, pause kadar bekler ve
// start kanalı kapatılınca hep birden döner (rampasız — fırtınanın kendisi).
type stormCfg struct {
	member bool
	start  <-chan struct{}
	pause  time.Duration
	// done, üyenin yeniden bağlanma sonucunu ana akışa bildirir:
	// ≥0 = kopuş→senkron süresi (ms), −1 = başarısız.
	done chan<- int64
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
	wireKind := flag.String("wire", "json", "tel kodlaması: json (v1) | proto (v2, ikili)")
	ramp := flag.Int("ramp", 1000, "saniyede açılan yeni bağlantı (0 = hepsi birden; büyük N'de fırtına yaratır)")
	storm := flag.Float64("storm", 0, "senkron sonrası kopup AYNI ANDA geri dönen istemci oranı (0..1) — yeniden bağlanma fırtınası (F3.4)")
	stormPause := flag.Duration("stormPause", 2*time.Second, "fırtına istemcilerinin kapalı kaldığı süre")
	flag.Parse()

	binary := *wireKind == "proto"
	if !binary && *wireKind != "json" {
		fmt.Fprintln(os.Stderr, "-wire json ya da proto olmalı")
		os.Exit(2)
	}
	if *storm < 0 || *storm > 1 {
		fmt.Fprintln(os.Stderr, "-storm 0..1 aralığında olmalı")
		os.Exit(2)
	}
	stormN := int(float64(*n) * *storm)

	results := make([]clientResult, *n)
	var synced sync.WaitGroup
	var done sync.WaitGroup

	// İlerleme: büyük N'de bağlanma/senkron dakikalar sürer; sayaç akmalı.
	var syncedCount atomic.Int64
	progressDone := make(chan struct{})
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-progressDone:
				return
			case <-t.C:
				fmt.Printf("... %d/%d istemci hazır (senkron ya da hata)\n", syncedCount.Load(), *n)
			}
		}
	}()

	// Rampa: bağlantılar saniyede -ramp adet açılır; binlerce eşzamanlı TCP
	// el sıkışması hem üreteci hem gateway'i yapay biçimde boğar.
	var gate <-chan time.Time
	if *ramp > 0 {
		ticker := time.NewTicker(time.Second / time.Duration(*ramp))
		defer ticker.Stop()
		gate = ticker.C
	}
	// Fırtına düzeneği: ilk stormN istemci üyedir; start kapatılınca kopar,
	// stormPause sonra hep birden döner ve sonucu resyncCh'e bildirir.
	stormStart := make(chan struct{})
	resyncCh := make(chan int64, stormN)

	for i := 0; i < *n; i++ {
		if gate != nil {
			<-gate
		}
		synced.Add(1)
		done.Add(1)
		go func(id int) {
			defer done.Done()
			cfg := stormCfg{member: id < stormN, start: stormStart, pause: *stormPause, done: resyncCh}
			results[id] = runClient(id, *server, binary, *samples, *sampleInterval, *jitter, *waitCue, cfg, func() {
				syncedCount.Add(1)
				synced.Done()
			})
		}(i)
	}

	synced.Wait()
	close(progressDone)
	fmt.Printf("bağlanma ve senkron aşaması bitti (%d istemci).\n", *n)

	if stormN > 0 {
		fmt.Printf("fırtına: %d istemci bağlantıyı koparıyor; %s sonra HEPSİ BİRDEN dönecek...\n", stormN, *stormPause)
		close(stormStart)
		var resyncs []float64
		failed := 0
		for i := 0; i < stormN; i++ {
			if ms := <-resyncCh; ms >= 0 {
				resyncs = append(resyncs, float64(ms))
			} else {
				failed++
			}
		}
		sort.Float64s(resyncs)
		if len(resyncs) > 0 {
			fmt.Printf("fırtına bitti: %d/%d yeniden senkron — süre medyan %.0f ms, p95 %.0f ms, maks %.0f ms\n",
				len(resyncs), stormN, percentile(resyncs, 50), percentile(resyncs, 95), resyncs[len(resyncs)-1])
		}
		if failed > 0 {
			fmt.Fprintf(os.Stderr, "fırtına: %d istemci geri dönemedi (ayrıntı hata özetinde)\n", failed)
		}
	}

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
// dönen sonuç kue alımını da içerir. storm.member ise istemci senkrondan
// sonra kopar ve start sinyaliyle diğer üyelerle AYNI ANDA geri döner.
func runClient(id int, server string, binary bool, samples int, sampleInterval time.Duration, jitter int64, waitCue time.Duration, storm stormCfg, onSynced func()) clientResult {
	syncedOnce := sync.OnceFunc(onSynced)
	defer syncedOnce()
	res := clientResult{id: id}

	frameKind := websocket.TextMessage
	protocolVersion := uint32(wire.ProtocolVersion)
	if binary {
		frameKind = websocket.BinaryMessage
		protocolVersion = wire.ProtocolVersionBinary
	}

	send := func(conn *websocket.Conn, msgType string, msg any) error {
		var data []byte
		var err error
		if binary {
			data, err = wire.EncodeBinary(msgType, msg)
		} else {
			data, err = wire.Encode(msgType, msg)
		}
		if err != nil {
			return err
		}
		return conn.WriteMessage(frameKind, data)
	}

	// readMessage, kodeğe göre çözüp (tür, gövde, çerçeve boyutu) döndürür.
	readMessage := func(conn *websocket.Conn, deadline time.Time) (string, any, int, error) {
		_ = conn.SetReadDeadline(deadline)
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return "", nil, 0, err
		}
		var msgType string
		var msg any
		if binary {
			msgType, msg, err = wire.DecodeBinary(raw)
		} else {
			msgType, msg, err = wire.DecodeMessage(raw)
		}
		return msgType, msg, len(raw), err
	}

	// syncRound: kurulu bağlantı üzerinde tek saat senkronu turu. Yapay
	// gecikme, t0 alındıktan sonra (gidiş) ve çerçeve okunduktan sonra
	// (dönüş) uyunarak asimetrik ağ gecikmesini taklit eder; kestirici düşük
	// RTT'li örnekleri seçerek bununla başa çıkmak zorundadır. Tur sırasında
	// gelen başka çerçeveler (erken kue, geç katılım tekrarı) yok sayılır.
	seqBase := uint32(0)
	syncRound := func(conn *websocket.Conn) (clocksync.Estimate, error) {
		var est clocksync.Estimator
		for i := 1; i <= samples; i++ {
			seqBase++
			seq := seqBase
			t0 := monoMs()
			sleepJitter(jitter)
			if err := send(conn, wire.TypeClockSyncRequest, wire.ClockSyncRequest{Seq: seq, ClientMonoMs: t0}); err != nil {
				return clocksync.Estimate{}, fmt.Errorf("senkron isteği: %w", err)
			}
			for {
				msgType, msg, _, err := readMessage(conn, time.Now().Add(10*time.Second))
				if err != nil {
					return clocksync.Estimate{}, fmt.Errorf("senkron yanıtı: %w", err)
				}
				if msgType != wire.TypeClockSyncResponse {
					continue
				}
				resp, respOk := msg.(wire.ClockSyncResponse)
				if !respOk || resp.Seq != seq {
					continue
				}
				sleepJitter(jitter)
				est.Add(clocksync.Sample{T0: resp.ClientMonoMs, T1: resp.ServerRecvMs, T2: resp.ServerSendMs, T3: monoMs()})
				break
			}
			time.Sleep(sampleInterval)
		}
		return est.Estimate()
	}

	// connectAndSync: bağlan + hello/welcome + saat senkronu turu. Fırtına
	// üyesi kopuştan sonra aynı yolu bir kez daha yürür.
	connectAndSync := func() (*websocket.Conn, clocksync.Estimate, error) {
		// Küçük tamponlar: 50k istemcide varsayılan 4 KiB tamponlar tek
		// başına yüzlerce MB tutar; teldeki en büyük çerçeve birkaç yüz bayt.
		dialer := &websocket.Dialer{
			ReadBufferSize:   1024,
			WriteBufferSize:  1024,
			HandshakeTimeout: 20 * time.Second,
		}
		conn, _, err := dialer.Dial(server, nil)
		if err != nil {
			return nil, clocksync.Estimate{}, fmt.Errorf("bağlantı: %w", err)
		}
		ok := false
		defer func() {
			if !ok {
				conn.Close()
			}
		}()

		if err := send(conn, wire.TypeHello, wire.Hello{ProtocolVersion: protocolVersion, ClientKind: "loadgen"}); err != nil {
			return nil, clocksync.Estimate{}, fmt.Errorf("hello: %w", err)
		}
		if msgType, _, _, err := readMessage(conn, time.Now().Add(10*time.Second)); err != nil || msgType != wire.TypeWelcome {
			return nil, clocksync.Estimate{}, fmt.Errorf("welcome beklenirken: tür=%q hata=%v", msgType, err)
		}

		estimate, err := syncRound(conn)
		if err != nil {
			return nil, clocksync.Estimate{}, err
		}
		ok = true
		return conn, estimate, nil
	}

	conn, estimate, err := connectAndSync()
	if err != nil {
		res.err = err
		return res
	}
	defer func() { conn.Close() }() // conn fırtınada değişebilir; kapanış son bağlantıya
	res.est = estimate
	syncedOnce()

	if storm.member {
		<-storm.start
		_ = conn.Close() // kaba kopuş: kapanış el sıkışması yok (ağ kesintisi taklidi)
		time.Sleep(storm.pause)
		t0 := time.Now()
		conn2, est2, err := connectAndSync()
		if err != nil {
			storm.done <- -1
			res.err = fmt.Errorf("fırtına yeniden bağlanması: %w", err)
			return res
		}
		conn, res.est = conn2, est2
		res.resyncMs = time.Since(t0).Milliseconds()
		// Kalite kapısı (telefonun aynası, realtime_client): kopmadan önceki
		// ofset taze ve daha kaliteliyse, izdihamda ölçülen tur onu EZMEZ
		// (saat kayması bu ölçekte ihmal). İkinci 20k koşumu bunun eksiğini
		// gösterdi: 5 sn'lik taze tur bile kuyruğu tam toplamıyordu çünkü
		// zaten iyi olan ofset önce kalitesiziyle değiştiriliyordu.
		if preStorm := estimate; preStorm.BestRTTMs+10 < res.est.BestRTTMs {
			res.est = preStorm
		}
		// Elde iyi ofset yoksa telefon gibi kısa aralıkla taze tur atılır.
		if res.est.BestRTTMs > 25 {
			time.Sleep(5*time.Second + time.Duration(rand.Int64N(2000))*time.Millisecond)
			if est3, err := syncRound(conn); err == nil && est3.BestRTTMs < res.est.BestRTTMs {
				res.est = est3
			}
		}
		storm.done <- res.resyncMs
	} else if res.est.BestRTTMs > 25 {
		// Sabit istemciler de telefonun davranışını taklit eder: rampa
		// sırasındaki ilk tur kalitesizse kısa aralıkla taze tur atılır
		// (telefon bunu 5 sn'lik yeniden senkronla kendiliğinden yapar).
		time.Sleep(5*time.Second + time.Duration(rand.Int64N(2000))*time.Millisecond)
		if est3, err := syncRound(conn); err == nil && est3.BestRTTMs < res.est.BestRTTMs {
			res.est = est3
		}
	}

	// Kue bekle; tekrarlar run_id ile tekilleştirilir, ilki esas alınır.
	cueDeadline := time.Now().Add(waitCue)
	for {
		msgType, msg, frameBytes, err := readMessage(conn, cueDeadline)
		if err != nil {
			res.err = fmt.Errorf("kue beklenirken: %w", err)
			return res
		}
		if msgType != wire.TypeCueStart {
			continue
		}
		cue, ok := msg.(wire.CueStart)
		if !ok {
			continue
		}
		res.runID = cue.RunID
		res.cueFrameBytes = frameBytes
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
	// Hatalar özetlenir: 50k istemcide satır satır dökmek raporu boğar.
	var ok []clientResult
	errCounts := map[string]int{}
	for _, r := range results {
		if r.err != nil {
			errCounts[errKey(r.err)]++
			continue
		}
		ok = append(ok, r)
	}
	if len(errCounts) > 0 {
		fmt.Fprintf(os.Stderr, "%d istemci hata aldı:\n", len(results)-len(ok))
		for msg, n := range errCounts {
			fmt.Fprintf(os.Stderr, "  %6d × %s\n", n, msg)
		}
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
	// Fırtına koşumunda yayılım grup grup da verilir: kuyruk fırtına
	// dönüşlerinde mi, sabit istemcilerde mi — teşhis buradan okunur.
	var stormFires, stableFires []float64
	for _, r := range ok {
		if r.resyncMs > 0 {
			stormFires = append(stormFires, float64(r.fireLocalMs))
		} else {
			stableFires = append(stableFires, float64(r.fireLocalMs))
		}
	}
	if len(stormFires) > 0 && len(stableFires) > 0 {
		sort.Float64s(stormFires)
		sort.Float64s(stableFires)
		fmt.Printf("  fırtına grubu  : %d istemci, kendi içinde maks−min %.0f ms, p95−p5 %.0f ms\n",
			len(stormFires), stormFires[len(stormFires)-1]-stormFires[0],
			percentile(stormFires, 95)-percentile(stormFires, 5))
		fmt.Printf("  sabit grup     : %d istemci, kendi içinde maks−min %.0f ms, p95−p5 %.0f ms\n",
			len(stableFires), stableFires[len(stableFires)-1]-stableFires[0],
			percentile(stableFires, 95)-percentile(stableFires, 5))
		fmt.Printf("  gruplar arası  : medyan farkı %.0f ms\n",
			percentile(stormFires, 50)-percentile(stableFires, 50))
	}
	fmt.Printf("kue çerçevesi    : %d bayt\n", ok[0].cueFrameBytes)
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

// errKey, hataları gruplamak için değişken kısımları (port, adres) kırpılmış
// kaba bir anahtar üretir.
func errKey(err error) string {
	msg := err.Error()
	if i := strings.Index(msg, "dial tcp"); i >= 0 {
		if j := strings.LastIndex(msg, ":"); j > i {
			return msg[:i] + "dial tcp …" + msg[j:]
		}
	}
	return msg
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

// TekSes ultrasonik beacon aracı (F2.7).
//
// Üç iş görür:
//
//  1. Beacon WAV üretimi — PA'dan çalınacak dosya:
//     go run ./tools/beacon -out beacon.wav -cue 0 -countdown 3000 -repeats 3
//     Geri sayım İLK patlamanın chirp başlangıcına göredir; yinelemeler
//     -interval aralıkla dizilir ve her birinin geri sayımı buna göre düşer
//     (hangisi duyulursa duyulsun aynı ateşleme anını verir).
//
//  2. PA test kiti — mekân hoparlörünün ultrasonik bandı geçirip
//     geçirmediğini ölçmek için ton/süpürme dosyaları:
//     go run ./tools/beacon -patest cikti-dizini
//
//  3. Kayıt çözümü — telefonla alınan mekân kaydında beacon aramak:
//     go run ./tools/beacon -decode kayit.wav
//     (16-bit PCM WAV, mono/stereo, ≥40 kHz; telefonun ses kaydedicisi uyar.)
//
// Saha kılavuzu: docs/ultrasonik-beacon.md
package main

import (
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/msaliheroglu/tekses/packages/beacon"
)

func main() {
	out := flag.String("out", "", "üretilecek beacon WAV yolu")
	cue := flag.Int("cue", 0, "cue_index (0 = otomatik program, 1..N = manifest sırası)")
	countdown := flag.Int("countdown", 3000, "ilk patlamanın chirp başlangıcından ateşlemeye kalan süre (ms)")
	repeats := flag.Int("repeats", 3, "patlama yinelemesi")
	interval := flag.Int("interval", 1000, "yinelemeler arası süre (ms, chirp başından chirp başına)")
	rate := flag.Int("rate", 48000, "örnekleme hızı (Hz)")
	patest := flag.String("patest", "", "PA test kitini bu dizine üret")
	decode := flag.String("decode", "", "verilen WAV kaydında beacon çöz")
	analyze := flag.String("analyze", "", "verilen WAV kaydını teşhis et (çözmeden: bant/chirp var mı)")
	flag.Parse()

	var err error
	switch {
	case *analyze != "":
		err = runAnalyze(*analyze)
	case *decode != "":
		err = runDecode(*decode)
	case *patest != "":
		err = runPATest(*patest, *rate)
	case *out != "":
		err = runEncode(*out, *cue, *countdown, *repeats, *interval, *rate)
	default:
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "hata:", err)
		os.Exit(1)
	}
}

func runEncode(path string, cue, countdown, repeats, intervalMs, rate int) error {
	if repeats < 1 || repeats > 16 {
		return fmt.Errorf("repeats 1..16 aralığında olmalı (seq alanı 4 bit)")
	}
	burstLen := int((beacon.ChirpSec + beacon.GapSec + beacon.PayloadBits*beacon.SymbolSec) * float64(rate))
	intervalN := intervalMs * rate / 1000
	if intervalN < burstLen {
		return fmt.Errorf("interval en az patlama süresi (%d ms) olmalı", burstLen*1000/rate)
	}

	total := make([]float64, (repeats-1)*intervalN+burstLen)
	for i := 0; i < repeats; i++ {
		cd := countdown - i*intervalMs
		if cd < 0 {
			return fmt.Errorf("countdown, %d. yineleme için negatife düşüyor (repeats/interval'ı küçültün)", i+1)
		}
		burst, err := beacon.Encode(beacon.Payload{
			Version: beacon.Version, CueIndex: cue, Seq: i + 1, CountdownMs: cd,
		}, rate)
		if err != nil {
			return err
		}
		copy(total[i*intervalN:], burst)
	}
	if err := writeWAV(path, total, rate); err != nil {
		return err
	}
	fmt.Printf("%s: %d yineleme, cue_index=%d, ilk geri sayım %d ms, %d Hz\n",
		path, repeats, cue, countdown, rate)
	fmt.Println("DİKKAT: dosyayı çalan zincirde ses işleme (EQ/limiter/AGC) 18 kHz üstünü kesebilir.")
	return nil
}

func runPATest(dir string, rate int) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	// Duyulur referans + banda tırmanan tonlar: PA'nın nerede kesildiği,
	// telefondaki spektrum uygulamasında bakınca tek tek görülür.
	freqs := []float64{1000, 10000, 16000, 17000, 18000, 18500, 19000, 19500, 20000}
	for _, f := range freqs {
		if err := writeWAV(filepath.Join(dir, fmt.Sprintf("ton_%05.0f.wav", f)), tone(f, 3, rate), rate); err != nil {
			return err
		}
	}
	if err := writeWAV(filepath.Join(dir, "supurme_15k_21k.wav"), sweep(15000, 21000, 6, rate), rate); err != nil {
		return err
	}
	// Gerçek beacon örneği: kayıt + `-decode` ile uçtan uca doğrulama.
	burst, err := beacon.Encode(beacon.Payload{Version: beacon.Version, CueIndex: 0, Seq: 1, CountdownMs: 3000}, rate)
	if err != nil {
		return err
	}
	if err := writeWAV(filepath.Join(dir, "ornek_beacon.wav"), burst, rate); err != nil {
		return err
	}
	fmt.Printf("PA test kiti %s dizinine yazıldı (%d dosya). Kılavuz: docs/ultrasonik-beacon.md\n", dir, len(freqs)+2)
	return nil
}

func runDecode(path string) error {
	chans, rate, err := readWAV(path)
	if err != nil {
		return err
	}
	samples, chNote := bestChannel(chans, rate)
	fmt.Println("kanal:", chNote)
	dets, err := beacon.DecodeAll(samples, rate, 16)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d Hz, %.1f sn — %d beacon çözüldü\n", path, rate, float64(len(samples))/float64(rate), len(dets))
	for _, d := range dets {
		at := float64(d.StartSample) / float64(rate)
		note := ""
		if d.Corrected > 0 {
			note = fmt.Sprintf("  (%d zayıf bit düzeltildi)", d.Corrected)
		}
		fmt.Printf("  t=%6.2f sn  cue_index=%d  seq=%d  geri sayım=%d ms  skor=%.2f%s\n",
			at, d.Payload.CueIndex, d.Payload.Seq, d.Payload.CountdownMs, d.Score, note)
	}
	return nil
}

// bestChannel, stereo kayıtta 17,5 kHz üstü enerjisi yüksek kanalı seçer
// (ortalama, faz iptaliyle beacon'ı söndürebilir — wav.go notu).
func bestChannel(chans [][]float64, rate int) ([]float64, string) {
	if len(chans) == 1 {
		return chans[0], "mono"
	}
	if beacon.InBandEnergy(chans[1], rate) > beacon.InBandEnergy(chans[0], rate) {
		return chans[1], "stereo → sağ kanal seçildi (bant enerjisi yüksek)"
	}
	return chans[0], "stereo → sol kanal seçildi (bant enerjisi yüksek)"
}

func runAnalyze(path string) error {
	chans, rate, err := readWAV(path)
	if err != nil {
		return err
	}
	samples, chNote := bestChannel(chans, rate)
	fmt.Println("kanal:", chNote)
	rep, err := beacon.Analyze(samples, rate)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d Hz, %.1f sn\n", path, rate, rep.DurationSec)
	fmt.Printf("  bant içi enerji oranı (>17,5 kHz) : %.4f\n", rep.InBandRatio)
	fmt.Printf("  bit taşıyıcıları (18,6/19,4 kHz)  : görüldü=%v\n", rep.CarrierSeen)
	fmt.Printf("  en iyi chirp skoru                : %.2f (t=%.2f sn; çözüm eşiği 0,35)\n",
		rep.BestChirpScore, rep.BestChirpAtSec)
	fmt.Printf("  yük çözümü (CRC)                  : ok=%v\n", rep.DemodOK)
	fmt.Printf("  sembol karar payı (P10)           : %.2f (temiz ~0,9+; <0,3 = bulanık)\n",
		rep.SymbolMarginP10)
	if rep.RawBits != "" {
		fmt.Printf("  ham bitler (ofset 0)              : %s\n", rep.RawBits)
		fmt.Println("                                      (version|cue|seq|countdown|crc)")
	}
	if len(rep.Offsets) > 0 {
		fmt.Print("  ofset taraması (ms→CRC/pay)       :")
		for _, o := range rep.Offsets {
			mark := "✗"
			if o.CRCOK {
				mark = "✓"
			}
			fmt.Printf(" %+.0f:%s%.2f", o.OffMs, mark, o.MarginP10)
		}
		fmt.Println()
	}
	fmt.Println()
	switch {
	case rep.DemodOK:
		fmt.Println("TEŞHİS: yük bu kayıtta çözülüyor — -decode ile tam sonucu alın.")
	case rep.InBandRatio < 0.001:
		fmt.Println("TEŞHİS: 17,5 kHz üstünde neredeyse HİÇ enerji yok — sinyal kayda hiç")
		fmt.Println("girmemiş. Sıra: (1) m4a/AAC sıkıştırması bandı siler → WAV/kayıpsız")
		fmt.Println("kaydedin; (2) hoparlör 19 kHz basamıyor olabilir (laptop hoparlörleri")
		fmt.Println("sıklıkla basamaz — harici hoparlör deneyin); (3) Bluetooth zinciri bandı")
		fmt.Println("öldürür (kablolu çalın); (4) ses seviyesini yükseltin, 1-2 m'den kaydedin.")
	case !rep.CarrierSeen || rep.BestChirpScore < 0.35:
		fmt.Println("TEŞHİS: bantta enerji VAR ama beacon deseni seçilemiyor — sinyal yolda")
		fmt.Println("bozulmuş. Sıra: kayıt uygulamasının gürültü bastırma/AGC ayarını kapatın,")
		fmt.Println("48 kHz WAV seçin; hoparlöre yaklaşın; ortam gürültüsünü azaltın.")
	case rep.SymbolMarginP10 < 0.3:
		fmt.Println("TEŞHİS: chirp güçlü ama veri sembolleri BULANIK (karar payı düşük) —")
		fmt.Println("tipik suçlu kayıplı kodek (m4a/AAC bu bandı kabaca kodlar) ya da ağır")
		fmt.Println("yankı. Sıra: (1) telefonda WAV/KAYIPSIZ kaydedin (iPhone: Sesli Notlar →")
		fmt.Println("Kayıpsız; Android: WAV kaydeden uygulama) ve tekrar deneyin; (2) yankısız")
		fmt.Println("konumda, hoparlöre 1-2 m mesafede kaydedin.")
	default:
		fmt.Println("TEŞHİS: chirp ve semboller iyi görünüyor ama CRC tutmuyor — sınır durum.")
		fmt.Println("Kaydı biraz daha yüksek seviye/yakın mesafeyle tekrarlayın; sürerse bu")
		fmt.Println("analiz çıktısını paylaşın.")
	}
	return nil
}

func tone(freq float64, seconds, rate int) []float64 {
	n := seconds * rate
	out := make([]float64, n)
	for i := range out {
		out[i] = 0.8 * env(i, n, rate/100) * sin2pi(freq*float64(i)/float64(rate))
	}
	return out
}

func sweep(from, to float64, seconds, rate int) []float64 {
	n := seconds * rate
	out := make([]float64, n)
	phase := 0.0
	for i := range out {
		f := from + (to-from)*float64(i)/float64(n)
		phase += f / float64(rate)
		out[i] = 0.8 * env(i, n, rate/100) * sin2pi(phase)
	}
	return out
}

func env(i, n, ramp int) float64 {
	switch {
	case i < ramp:
		return float64(i) / float64(ramp)
	case i >= n-ramp:
		return float64(n-1-i) / float64(ramp)
	default:
		return 1
	}
}

// sin2pi(x) = sin(2πx); faz önce 0..1'e indirgenir — math.Sin çok büyük
// argümanda hassasiyet yitirir (uzun süpürmelerde faz milyonlara çıkar).
func sin2pi(x float64) float64 {
	x -= math.Floor(x)
	return math.Sin(2 * math.Pi * x)
}

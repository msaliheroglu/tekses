# TekSes — Proje Özeti, Alınan Kararlar ve Plan
_30 Ağustos 2026 tarihli oturumun çıktısı. Yeni bir Claude oturumuna ilk mesaj olarak yapıştırılabilir._

---

## 1. Proje nedir?

**TekSes**, stadyum, konser, miting ve konferans gibi kalabalık etkinliklerde bir moderatörün on binlerce seyircinin telefonunda **senkronize tezahürat, marş ve şarkı koreografisi** yürütmesini sağlayan gerçek zamanlı bir platformdur.

Temel yetenekler:
- Canlı karaoke: şarkı sözleri ve ritim zamanlaması seyirci ekranında eşzamanlı akar.
- Ses ve görsel koreografi: isteğe bağlı senkron ses çalma; ekran rengi değişimi, ritmik yanıp sönme, telefon feneri/strobe.
- Roller: **Moderatör paneli** (web; etkinlik oluşturma, söz/koreografi zaman damgaları, katılım kodu ve QR üretimi, canlı veya zamanlanmış tetikleme) ve **Katılımcı uygulaması** (Android + iOS, sıfır sürtünmeli, hesapsız).
- Ön yükleme: odaya girildiği anda tüm şarkı verisi, görsel ipuçları ve ses dosyaları telefona indirilir.
- Ultra düşük gecikmeli tetikleme: dengesiz hücresel ağda on binlerce telefonda aynı anda çalışacak senkron mekanizması.

## 2. Kesinleşen kararlar

| Konu | Karar |
|---|---|
| Hedef ölçek | Tek odada **20.000–80.000+** katılımcı (tam stadyum) |
| Teknoloji yığını | **Flutter** (mobil) + **Go** (backend/gateway) + **Next.js** (moderatör paneli) + Postgres + NATS JetStream + S3 uyumlu depolama + CDN |
| Senkron stratejisi | Birincil: **zamanlanmış zaman çizelgesi + saat senkronu**. İkincil/isteğe bağlı: **ultrasonik akustik beacon**. Kural: telefon hangi kaynaktan ilk tetiği alırsa o kaynağa kilitlenir ve onunla devam eder ("ilk gelen kazanır, sonra sadık kal"); kilitli kaynak susarsa diğerine geçer. |
| Arka plan çalışma | **Yalnızca ön planda** (ekran açık, wakelock ile). Kilitli ekranda çalışma yok. |
| Ses kapsamı | Hem organizatörün kaydettiği tezahüratlar hem **lisanslı şarkılar** desteklenecek (lisans sorumluluğu organizatörde; şarkı bazında lisans notu alanı tutulur). |
| Barındırma | Şimdilik **Türkiye'de çalışıyormuş gibi** tasarla; pilot için Frankfurt bölgesi + Cloudflare CDN (İstanbul uç sunucuları). Ücretsiz katmanlarla başla, etkinlik günü ölçekle. |
| Kimlik / kiracılık | İlk günden **çok kiracılı** (her organizatör yalnızca kendi etkinliklerini görür); pilot için basit **e-posta + şifre** girişi. |
| Tarayıcı yedeği | MVP'de **yok**; Faz 3'e ertelendi (iOS Safari'de fener çalışmaz, yalnızca söz + ekran rengi). |
| Depo | **Monorepo**: https://github.com/msaliheroglu/tekses |

## 3. Mimarinin özü

**Yönetici ilke:** Ağ, hazırlık içindir; icra için değil. Tam stadyumda kue anında hücresel ağ çöker; bu yüzden telefon kue anında **hiçbir bağlantıya ihtiyaç duymaz**.

Telefon odaya girdiğinde üç şeyi önceden edinir:
1. **Gösteri paketi** (sözler, kue şeritleri, ses, renkler) — CDN'den, hash doğrulamalı, sürümlü ve değişmez.
2. **Saat ofseti** — WebSocket üzerinden NTP benzeri değişim (8–12 örnek, düşük RTT'li olanlar seçilir, medyan ofset); LTE'de ±5–30 ms; her 1–2 dakikada yenilenir; son iyi değer saklanır. Cihazda **monoton saat** kullanılır, duvar saati asla.
3. **Zaman çizelgesi** — moderatörün canlı komutu ("7. sekans sunucu saatiyle T'de başlar", T = şimdi + 2–5 sn, 3 kez tekrarlanır) veya paket içine gömülü otomatik program.

**Cue Arbiter (cihazda):** WS komutu, otomatik program ve ultrasonik beacon `CueStart` adayı üretir; bir çalıştırma (runId) için ilk geçerli aday kaynağını kilitler. ~250 ms tolerans: WS ile ultrasonik aynı anda gelirse hassas olan (WS) kazanır. Sekans başladıktan sonra tüm koreografi yerel saatten akar; yalnızca müdahaleler (HOLD, STOP, SKIP, BLACKOUT) canlı gönderilir.

**Ultrasonik beacon:** 18,5–20 kHz; chirp önek + küçük FSK yük (cueId, seq, geri sayım, CRC); `fireAt − 3 sn`'den itibaren tekrarlanır. Ses hızı nedeniyle ±300 ms hata taşır → erişilebilirlik yedeği, hassasiyet kaynağı değil. Mekân PA testi şart (çoğu PA 16 kHz üstünü keser).

**Gerçekçi hassasiyet hedefleri:** Ekran rengi/flaş ve fener için **≤30 ms** (göze eşzamanlı görünür). Milisaniye altı kablosuzla mümkün değil ve gerekmiyor. Telefon başına ses senkronu iyi; ama 60 bin hoparlörün stadyumda tek ses gibi *duyulması* fiziksel olarak imkânsız (100 m = 290 ms). **Işık gösterisi vaat edilir, akustik birlik değil.**

**Backend (Go):** Kontrol API'si (REST; etkinlik, gösteri, paketleme, katılım kodu/QR, zamanlama, canlı konsol, telemetri) + durumsuz WebSocket gateway'leri (düğüm başına 50–100k bağlantı; 80k için 2–4 düğüm), oda dağıtımı ve kue tekrarı NATS JetStream ile, mesajlar protobuf (~40 bayt). Yeniden bağlanma fırtınasına karşı jitter'lı üstel geri çekilme; telefonun gösteri için yeniden bağlanması gerekmez. 100k istemci simüle eden Go yük üreteci ilk sınıf teslimattır.

**Mobil (Flutter) modülleri:** join, package_store, clock_sync, realtime_client, cue_arbiter, timeline_engine, renderer'lar (lyrics, screen_fx, torch, audio), ultrasonic_listener, telemetry. Fener ve zamanlanmış ses **native platform kanallarından** (Android CameraManager/AudioTrack, iOS AVCaptureDevice/AVAudioPlayer `play(atTime:)`) — Dart seviyesi zamanlama fazla titrek. Işığa duyarlılık için strobe hızı sınırı (sürekli ≤3 Hz) ve uygulama içi "yanıp sönmeyi kapat" seçeneği.

**Moderatör paneli (Next.js):** dalga formu üzerinde söz zamanlama (LRC içe/dışa aktarma), paralel kue şeritleri, telefon önizleme simülatörü, paket derleme/yayınlama, QR, katılımcı sayısı ve saat kalitesi ısı haritası, dev GO / HOLD / STOP / BLACKOUT düğmeli canlı konsol, otomatik program editörü.

**Veri modeli:** Organization → Event → Room (joinCode, activeShowVersion); Show → ShowVersion (değişmez manifest) → Sequence → { LyricLine[], CueLane[] → Cue, AudioAsset }; Schedule; Run; Participant.

Önerilen monorepo düzeni:
```
/apps/participant     Flutter uygulaması
/apps/moderator       Next.js paneli
/services/control-api Go kontrol API'si
/services/gateway     Go WebSocket gateway
/packages/proto       Protobuf sözleşmeleri
/tools/loadgen        Go yük üreteci
```

## 4. Ücretsiz altyapı planı

Geliştirme ve küçük pilot ücretsiz; gerçek stadyum gecesi için birkaç saatliğine 2–4 sunucu kiralanır (aylık birkaç yüz TL mertebesi).

- **Sunucu (API + gateway + Postgres + NATS, Docker):** Oracle Cloud Always Free ARM VM — Haziran 2026'dan beri **2 OCPU / 12 GB** (eskiden 4/24), Frankfurt bölgesi mevcut. Uyuyan ücretsiz servisler (Render, Koyeb) WebSocket için uygun değil.
- **Paket depolama + CDN:** Cloudflare R2 (10 GB ücretsiz, **çıkış trafiği ücretsiz** — 60k × 5 MB = 300 GB trafik burada bedava).
- **Postgres (yönetilen, isteğe bağlı):** Neon veya Supabase ücretsiz katmanı (Supabase: 500 MB, 7 gün hareketsizlikte duraklar). Başta Oracle VM'de de çalıştırılabilir.
- **Moderatör paneli:** Vercel Hobby veya Cloudflare Pages.
- **CI/CD:** GitHub Actions. **İzleme:** Grafana Cloud ücretsiz katmanı.
- **Kaçınılmaz ücretler:** Google Play (~25 $ tek sefer), Apple Developer (99 $/yıl).

## 5. Yol haritası

- **Faz 0 — Senkron denemesi (1–2 hafta):** Saat senkronu yanıtlayan Go gateway + belirlenen sunucu saatinde ekranı ve feneri yakan minimal Flutter uygulaması. 5–10 telefon Wi-Fi ve LTE'de 240 fps kamerayla filme alınıp gerçek senkron ölçülür. **Her şeyden önce bu yapılır; tezi ucuza doğrular.**
- **Faz 1 — MVP:** Kontrol API'si, paketleme + CDN, katılım akışı, sözler + ekran efektleri + fener, moderatör düzenleyici ve canlı konsol, otomatik program. Tek mekân pilotu.
- **Faz 2 — Ölçek ve ses:** Native zamanlanmış ses, 100k istemci yük testi, NATS tekrar, telemetri panoları, ultrasonik beacon + PA test kiti, etkinlik günü sunucu kiralama planı.
- **Faz 3 — Sertleştirme:** Işığa duyarlılık/güvenlik incelemesi, cihaz sınıfı fener kalibrasyon tablosu, tarayıcı katılımcı yedeği, analitik, faturalama.

## 6. GitHub bağlantısı — durum ve yapılacak

- Depo: `msaliheroglu/tekses` (main dalında yalnızca README var).
- Cowork oturumundan depo **okunabiliyor ama yazılamıyor**; Anthropic'in git köprüsü yalnızca görev açılırken seçilen depolara yazma izni veriyor. Kişisel erişim token'ı ile de aşılamıyor (güvenlik sınırı).
- **Sohbete yapıştırılan token GitHub'dan iptal edilmeli** (Settings → Developer settings → Fine-grained tokens).
- Claude GitHub uygulaması kurulumu **github.com/apps/claude** adresinden yapılır (claude.ai "organization settings" sayfası Team/Enterprise'a özeldir, kullanılmaz).
- Çözüm: tarayıcıda **claude.ai/code** → ortadaki görev kutusunun yanındaki depo seçicisinden `msaliheroglu/tekses` seç → görevi yaz → gönder. Depo listede yoksa Claude'a bağlı GitHub hesabının depoya erişimi yoktur (collaborator eklenmeli veya doğru hesap bağlanmalı).

## 7. Yeni oturuma yapıştırılacak başlangıç mesajı

> TekSes projesine devam et. Bu dosya oturumun özeti ve alınan kararlardır; mimari doküman ayrıca Claude projesinde `architecture/tekses-architecture-v0.1.md` yolunda kayıtlı. Türkçe yanıt ver. Depo: msaliheroglu/tekses. Önerilen monorepo düzenini kur, protobuf mesaj sözleşmelerini (ClockSync, CueStart, müdahaleler) yaz ve Faz 0 senkron denemesiyle başla: saat senkronu yanıtlayan Go gateway + belirlenen sunucu saatinde ekran ve fener yakan minimal Flutter uygulaması. Kod üretirken üretim kalitesinde, modüler ve adım adım ilerle.

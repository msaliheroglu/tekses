# Faz 1 (MVP) ve Faz 2 — Plan ve Durum

Bu dosya oturumlar arası devir defteridir: her Claude oturumu buradan devam
eder, her tamamlanan adımda burası güncellenip push edilir. Kota/oturum
kesintisi durumunda yeni oturuma verilecek tek komut yeterlidir:
**"TekSes Faz 1'e devam et"** — oturum `CLAUDE.md` → bu dosya → işaretsiz ilk
adım sırasını izler.

## Çalışma kuralları (kesinti dayanıklılığı)

- Her anlamlı adım ayrı commit + anında `git push` (branch:
  `claude/tekses-monorepo-phase-0-p0l6ij`). Push edilmemiş iş yok sayılır.
- Doğrulama her adımda: `go build ./... && go vet ./... && go test ./...`
  (+ panel için `npm run build`). Kırmızıyken yeni adıma geçilmez.
- Bir adım yarım kaldıysa aşağıya "YARIM:" notu düşülür (ne bitti, sıradaki
  somut hamle ne).

## Adımlar

- [x] **0. Faz 0 tamamlandı** — monorepo, proto sözleşmeleri, gateway,
  loadgen (yayılım 17–23 ms ✓), Flutter Faz 0 uygulaması, web konsol + /join.
- [x] **1. Bu plan dosyası + devam mekanizması**
- [x] **2. Control API temeli** — `services/control-api`: alan modeli
  (Organization→Event→Room; Show→ShowVersion), depolama arayüzü + bellek içi
  store, e-posta+şifre kaydı/girişi (bcrypt, bearer token), çok kiracılı
  Event/Room CRUD; uçtan uca httptest'ler.
- [x] **3. Gösteri manifesti + yayınlama** — değişmez ShowVersion (kanonik
  JSON + SHA-256), Sequence/LyricLine/CueLane/Cue doğrulaması (flashHz ≤ 3),
  `POST /shows/{id}/versions`, `POST /rooms/{id}/activate`, herkese açık
  `GET /join/{code}`.
- [x] **4. Gateway oda entegrasyonu** — gateway'de oda kavramı; hello'daki
  join_code doğrulaması control-api üzerinden (`TEKSES_CONTROL_URL`); kue ve
  müdahale yayınının `room_id` ile odaya daraltılması (boş = tümü, Faz 0
  uyumlu).
- [x] **7. Moderatör paneli (Next.js) — MVP** *(öne çekildi; kullanıcı web
  panelini görmek istiyor, Postgres/CDN panelden bağımsız)* — giriş/kayıt,
  etkinlik + oda yönetimi (QR ile `/join?code=…`), manifest yayınlama (JSON
  editörü; dalga formu/LRC editörü sonraki yineleme), sürüm etkinleştirme,
  canlı konsol (GO/HOLD/STOP/BLACKOUT, odaya daraltılmış). control-api ve
  gateway'e Next rewrites üzerinden vekillenir (CORS'suz). CI'da build.
- [x] **5. Postgres kalıcılığı** — gömülü migration'lar + pgx store
  (`TEKSES_DATABASE_URL`; ayarsızsa bellek içi). Tüm store gerçeklemeleri
  ortak uygunluk paketinden (`store/storetest`) geçer; Postgres testi
  `TEKSES_TEST_DATABASE_URL` ile yerelde gerçek Postgres 16'da doğrulandı,
  CI'da servis konteyneriyle koşuyor. Kayıt atomikleştirildi
  (CreateOrgWithUser).
- [x] **6. Paketleme** — içerik adresli paket deposu (`packages/blob`,
  atomik/idempotent FS sürücüsü, `TEKSES_PACKAGES_DIR`); yayında paket
  `/packages/<sha256>.json` altına yazılır, join yanıtı `manifest_url` verir,
  indirme immutable önbellek başlığıyla sunulur ve SHA-256 ile doğrulanır.
  **Kalan:** R2/S3 sürücüsü aynı arayüzün arkasına dağıtım aşamasında
  (Oracle VM + Cloudflare kurulurken) eklenecek; ses varlıkları Faz 2.
- [x] **8. Katılımcı uygulaması MVP** — package_store (katılım kodu →
  paket indirme + SHA-256 doğrulama), show_manifest modeli, saf
  timeline_engine (birim testli), söz akışı + manifest güdümlü ekran/fener;
  `cue_id` = sekans id sözleşmesi, eşleşmeyen kueler Faz 0 yükü olarak
  oynar. **Dikkat:** Dart bu ortamda derlenemiyor — telefonda ilk
  `flutter analyze && flutter test` çıktısı kullanıcıdan beklenecek.
- [x] **9. Otomatik program + Run kaydı** — kullanıcı (a) seçeneğini seçti
  (2026-09-19): program manifestin içinde (`program` listesi, doğrulamalı:
  var olan sekans, artan sıra, üst üste binme yok); ayrılmış `program` kue
  kimliği akışı başlatır, telefon `ProgramEngine` ile yerelden oynatır
  (birim testli). Konsola sekans/program seçici ve gateway'in son 50
  çalıştırmayı tutan `GET /api/v0/runs` kaydı eklendi. **Faz 2'ye devir:**
  kalıcı Run tablosu, saat kalitesi ısı haritası ve telemetri panoları.

## Doğrulama durumu

- [x] **Gerçek cihaz doğrulaması (2026-09-19):** release APK Android
  cihazda çalıştı — kodla katılım, paket indirme + özet doğrulama, saat
  senkronu (ofset/RTT görüldü) ve kue denemesi başarılı. Yol boyu düzelen
  saha hataları: analyze hataları (library sırası, eksik import), release
  manifest'te INTERNET izni, ASCII olmayan Windows yolu.
- [ ] Çoklu telefon + 240 fps kamera ile fiziksel senkron ölçümü (≤30 ms
  hedefi) — cihazlar toplanınca; kılavuz: docs/faz0-senkron-denemesi.md.
- [ ] GitHub Actions koşumlarının gözden geçirilmesi; "Katılımcı APK" iş
  akışının Run workflow düğmesi branch main'e merge edilince görünür.

## Faz 2 — Ölçek ve ses

- [x] **F2.0 PR:** Faz 1 main'e PR #2 ile açıldı (2026-09-19); merge kararı
  kullanıcıda. CI'ı bu oturum gözetliyor.
- [x] **F2.1 Dağıtım paketi:** Dockerfile'lar (gateway, control-api, panel
  standalone), `deploy/docker-compose.yml` (postgres + üç servis + Caddy
  otomatik TLS, tek alan adında yol bazlı dağıtım), `.env.example`,
  kurulum rehberi `docs/dagitim.md`. **Not:** bu geliştirme ortamında
  Docker daemon yok — imaj derlemeleri VM'deki ilk `docker compose up
  --build` ile doğrulanacak; sorun çıkarsa hata çıktısıyla düzeltilir.
- [x] **F2.2 VM kurulumu (2026-09-20):** Oracle Always Free VM (Ubuntu 22.04,
  A1.Flex) kuruldu — ilk deneme Oracle Linux imajıyla açıldığı için SSH
  reddetti (kullanıcı `ubuntu` yok), VM Ubuntu ile yeniden yaratıldı
  (IP 141.144.246.29). Docker + iptables 80/443 + DuckDNS alan adı +
  `deploy/` compose yığını: 5 konteyner ayakta (postgres healthy),
  `https://<alan>/healthz` → `{"status":"ok"}` — Caddy sertifikayı aldı,
  TekSes internette. **Kalan doğrulama:** panelden kayıt + şablon gösteri
  yayını + telefonun LTE üzerinden katılıp kue alması (aşağıda yürüyor).
- [x] **F2.3 Protobuf ikili teli (Go tarafı, 2026-09-19):** buf + protoc-gen-go
  ile üretilen stub'lar commit'li; `wire` paketi iki kodeği tek arayüzde
  taşıyor (EncodeBinary/DecodeBinary ↔ Encode/DecodeMessage). Gateway iki
  kodeği aynı anda konuşuyor (hello çerçevesinin biçimi belirler); yayınlar
  çift kodlamalı Frame ile. loadgen `-wire proto`: canlıda kue çerçevesi
  **58 bayt** (JSON 207), senkron yayılımı değişmedi. **Kalan:** Dart
  stub'ları Flutter'lı ortamda üretilecek (`buf.gen.dart.yaml` hazır) ve
  uygulama v2'ye geçecek; o güne dek Flutter v1 JSON'da (gateway destekliyor).
- [ ] **F2.4 Yük testi:** loadgen'i 100k istemciye ölçekleme (çok bağlantılı
  koşum, bellek/CPU profili), yeniden bağlanma fırtınası senaryosu.
  *Ön yoklama (2026-09-19, geliştirme konteyneri, 4 çekirdek):* 2.000 ikili
  istemci tek gateway'de sorunsuz — yayılım 3 ms. Gerçek 100k koşumu fd/port
  sınırları gereği F2.2 VM'inde (ya da ayrı yük makinesinde) yapılacak.
- [x] **F2.5 Native zamanlanmış ses (2026-09-19):** (a) varlık boru hattı —
  POST /api/v1/assets (içerik adresli, <sha256>.<uzantı>), herkese açık
  /assets/{id}, yayında varlık doğrulaması; telefon varlıkları katılırken
  indirir ve özetle doğrular (path_provider önbelleği). (b) tekses/audio
  platform kanalı: Android Handler.postAtTime, iOS AVAudioPlayer
  play(atTime:); Dart MonoClock↔platform saati eşlemesi; kanal dosyaları
  `native/` altında (kopyalama adımı README + APK iş akışında). Panel'e ses
  yükleme arayüzü eklendi. **Cihaz doğrulaması bekliyor** (native dosyalar
  kopyalanıp sesli manifest denenecek); R2 sürücüsü F2.2 dağıtımına bağlı.
- [x] **F2.5k Karaoke ve sözler (2026-09-19):** telefonda karaoke görünümü
  (aktif satır + sıradaki satır soluk; TimelineFrame.nextLyric, testli);
  panelde LRC içe aktarma (senkronlu sözler → lyric_lines) ve DENEYSEL
  otomatik söz çıkarma: takılabilir çözümleyici komutu (TEKSES_TRANSCRIBER,
  sözleşme transcribe.go; whisper.cpp uyarlayıcısı deploy/transcribe-
  whisper.sh), bellek içi iş kuyruğu, panelde taslak üretimi. Şarkılarda
  ASR hatalıdır — çıktı taslak; kesin yol LRC/elle zamanlama. Kelime bazlı
  vurgulama (enhanced LRC) sonraki yineleme.
- [ ] **F2.6 NATS JetStream oda dağıtımı** (çok düğümlü gateway) ve
  telemetri panoları (kalıcı Run tablosu, saat kalitesi ısı haritası).
- [ ] **F2.7 Ultrasonik beacon + PA test kiti** (karar dokümanı §3).

## Notlar

- İyileştirme adayı (F2.2 saha gözlemi): panelde "etkinleştir" bağlı
  telefonlara canlı yansımıyor — telefon paketi yalnızca katılırken indiriyor,
  etkinleştirme sonrası yeniden katılmak gerekiyor. Gateway üzerinden odaya
  "show_activated" bildirimi + uygulamada otomatik paket yenileme eklenebilir.

- Mimari doküman (`tekses-architecture-v0.1.md`) hâlâ depoda değil; kullanıcı
  paylaşınca `docs/architecture/` altına eklenecek.
- Kullanıcının bekleyen istekleri: Android APK derleyen CI işi (isteğe bağlı,
  sorulunca eklenecek).

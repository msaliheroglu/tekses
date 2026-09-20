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

- [x] **F2.0 PR:** Faz 1 main'e PR #2 ile merge edildi (2026-09-19). Faz 2
  işleri (dağıtım, çift kodek, ses+karaoke, görsel editör, VM söz çıkarma)
  **PR #3 ile main'e merge edildi (2026-09-20)**; çalışma dalı yeni main'in
  üzerine alındı.
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
  TekSes internette. Panelden kayıt, gösteri yayını/etkinleştirme ve
  telefonların katılıp konsoldan verilen kueyle KOREOGRAFİYİ OYNATMASI
  kullanıcı tarafından doğrulandı. Yol boyu düzelenler: panel API'leri
  Caddy'den yönlendirildi (Next rewrites imaja localhost gömüyordu),
  konsol yetkisi panel oturumuna bağlandı, /join koreografi kazandı.
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
  *Araçlar hazır (2026-09-20):* loadgen'e rampa (-ramp), ilerleme sayacı,
  hata özetleme, küçük tamponlar; `tools/loadgen/Dockerfile`; gateway
  compose'una nofile 1M; kademeli VM koşum kılavuzu **docs/yuk-testi.md**
  (2k→10k→20k→40k; conntrack + port aralığı ayarlarıyla). Yerel duman
  testi: 2.000 ikili istemci, yayılım 1 ms. **VM koşum sonuçları
  (Oracle A1.Flex, gateway+loadgen aynı VM):**
  - 10k ikili istemci (2026-09-20): 10000/10000 başarılı, yayılım
    maks−min 15 ms / p95−p5 5 ms / σ 1.6 ms, en iyi RTT medyan 2 ms —
    ≤30 ms hedefi TUTUYOR.
  - 20k ikili istemci (2026-09-20): 20000/20000 başarılı (kopma yok);
    yayılım maks−min 45 ms / p95−p5 25 ms / σ 7.6 ms, RTT medyan 6 ms /
    p95 50 ms. Gateway RSS ~520 MiB (%9), CPU %43. NOT: VM 6 GB'lık
    küçük kurulum çıktı (free: 5.8 Gi) — bozulma CPU sıkışması imzalı
    (üreteç+gateway aynı çekirdekleri paylaşıyor); istemcilerin %90'ı
    hâlâ 25 ms bandında. Ders: tek KÜÇÜK düğümün konforlu sınırı ~10-15k;
    80k için düğüm başına ~10k hedefiyle çoklu gateway (F2.6 NATS) ve/veya
    daha büyük makine. 4 OCPU/24 GB'a büyütüp yeniden ölçüm planlandı.
  - VM 4 OCPU / 24 GB'a büyütüldü (Always Free sınırı) → 20k ikili istemci
    TEKRARI: 20000/20000 başarılı, yayılım maks−min **1 ms**, RTT medyan
    0 ms — 1 OCPU'daki 45 ms'lik bozulmanın tamamı CPU sıkışmasıymış;
    protokol 20k'da kusursuz. Sıradaki kademe 40k.
- [x] **F2.5 Native zamanlanmış ses (2026-09-19):** (a) varlık boru hattı —
  POST /api/v1/assets (içerik adresli, <sha256>.<uzantı>), herkese açık
  /assets/{id}, yayında varlık doğrulaması; telefon varlıkları katılırken
  indirir ve özetle doğrular (path_provider önbelleği). (b) tekses/audio
  platform kanalı: Android Handler.postAtTime, iOS AVAudioPlayer
  play(atTime:); Dart MonoClock↔platform saati eşlemesi; kanal dosyaları
  `native/` altında (kopyalama adımı README + APK iş akışında). Panel'e ses
  yükleme arayüzü eklendi. **Cihaz doğrulaması TAMAM (2026-09-20):** kullanıcı
  MainActivity.kt'yi kopyalayıp APK'yi yeniden derledi, VM üzerinden yayınlanan
  gösteride şarkı telefonda zamanında çaldı. (İlk deneme sessizdi — kanal
  eksikti; uygulamaya ses teşhis satırı eklendi.) R2 sürücüsü ileriye kaldı.
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

- [x] "Etkinleştir" canlı bildirimi (2026-09-20): panel, etkinleştirme
  başarılı olunca gateway'in POST /api/v0/show-activated ucunu çağırıyor;
  gateway odaya v1 JSON telinde "show_activated" yayınlıyor (ikili kodlaması
  yok — SendFrame v2 istemcileri atlar; proto zarfı sonraki yineleme).
  Tarayıcı katılımcısı manifesti yeniden indiriyor; Flutter uygulaması
  PackageStore.join'i yeniden koşup paket + varlıkları tazeliyor (süren
  koreografi etkilenmez). Gateway testi: TestShowActivatedBroadcast.
  **Telefonda devreye girmesi için APK yeniden derlenmeli** (yalnız git pull
  + build; native kopyalama adımı değişmedi).
- VM'de söz çıkarma (2026-09-20, KULLANICI DOĞRULADI): control-api'nin
  whisper.cpp + Demucs gömülü imaj varyantı (`Dockerfile.whisper` +
  `deploy/docker-compose.whisper.yml`, docs/dagitim.md §6) VM'de çalışıyor —
  şarkıdan taslak üretildi. ARM tuzakları çözüldü: demucs==4.0.1 +
  torch/torchaudio==2.4.1 sabit (4.1.0'ın sphn'i aarch64'te derlenmiyor),
  ısınma get_model ile, dockerignore istisnası, UID 1000, işler tek tek
  sırada (trGate), zaman aşımı TEKSES_TRANSCRIBE_TIMEOUT (ekte 60m).
- Görsel gösteri editörü (2026-09-20, kullanıcı isteği): panelde manifest
  artık formla düzenleniyor — sekans kartları, müzik seçimi (süre otomatik),
  ekran adımları (renk seçici + flaş), fener, satır satır sözler, otomatik
  program özeti; "Editörde aç" eski sürümü yükler, "Gelişmiş (JSON)" duruyor.
  Dönüşümler `apps/moderator/lib/manifestEditor.ts` (saf; gidiş-dönüş testli,
  çıktı Go manifest.Parse ile doğrulandı).
- F2.2 saha bulgularıyla eklendi (2026-09-20): (a) konsol yetkisi panel
  oturumuyla — gateway, /api/v0 uçlarında TEKSES_ADMIN_TOKEN'ın yanında
  panel oturum token'ını da kabul eder (control-api /api/v1/auth/whoami,
  60 sn önbellek); (b) /join tarayıcı katılımcısı artık manifest koreografisi
  oynatıyor (ekran şeridi + karaoke sözleri; TimelineEngine/ProgramEngine'in
  JS karşılığı, node ile mantık testli). Fener/ses yalnızca uygulamada.

- Mimari doküman (`tekses-architecture-v0.1.md`) hâlâ depoda değil; kullanıcı
  paylaşınca `docs/architecture/` altına eklenecek.
- Kullanıcının bekleyen istekleri: Android APK derleyen CI işi (isteğe bağlı,
  sorulunca eklenecek).

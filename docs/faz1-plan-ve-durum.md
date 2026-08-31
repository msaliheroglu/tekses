# Faz 1 (MVP) — Plan ve Durum

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
- [ ] **6. Paketleme + CDN** — manifest ve varlıkları R2/S3'e iten paketleyici
  (dev için dosya sistemi sürücüsü), hash doğrulamalı indirme sözleşmesi.
- [ ] **8. Katılımcı uygulaması MVP** — manifest indirme (package_store),
  timeline_engine, söz akışı + ekran efektleri; kue zaten çalışıyor.
- [ ] **9. Otomatik program + Run kaydı** — Schedule, Run, asgari telemetri.

## Notlar

- Mimari doküman (`tekses-architecture-v0.1.md`) hâlâ depoda değil; kullanıcı
  paylaşınca `docs/architecture/` altına eklenecek.
- Kullanıcının bekleyen istekleri: Android APK derleyen CI işi (isteğe bağlı,
  sorulunca eklenecek).

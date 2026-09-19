# Faz 0 — Senkron denemesi

Amaç: **"zamanlanmış zaman çizelgesi + saat senkronu" tezini ucuza doğrulamak.**
Belirlenen sunucu anında 5–10 telefonun ekranı ve feneri aynı anda yanar;
240 fps kamerayla filme alınıp gerçek sapma ölçülür.

**Başarı ölçütü:** telefonlar arası görsel sapma **≤30 ms** (240 fps'te ≤7 kare).
Wi-Fi ve LTE'de ayrı ayrı ölçülür.

---

## 1. Protokol

Tel, Faz 0'da JSON'dur ve `packages/proto/tekses/v1/*.proto` şemasını alan
adlarıyla birebir izler (Faz 1'de ikili protobuf'a geçilir). Her çerçeve bir
zarftır: `{"type": "...", "data": {...}}`.

### El sıkışma

1. İstemci → `hello` `{protocol_version, join_code, client_kind}`
2. Sunucu → `welcome` `{server_time_ms, protocol_version, room_id}`

### Saat senkronu (NTP benzeri)

İstemci `clock_sync_request {seq, client_mono_ms}` yollar (t0 = kendi
**monoton** saati). Sunucu `clock_sync_response {seq, client_mono_ms,
server_recv_ms (t1), server_send_ms (t2)}` döner; istemci alış anını t3 olarak
damgalar.

- Örnek başına: `rtt = (t3−t0) − (t2−t1)`, `ofset = ((t1−t0) + (t2−t3)) / 2`
- Tur başına 10 örnek (~40 ms arayla), düşük RTT'li **yarı** seçilir,
  ofsetlerin **medyanı** alınır. Negatif RTT'li örnek atılır.
- Ofset tanımı: `sunucuSaati ≈ istemciMonoton + ofset`
- Her 60 sn'de bir tur yenilenir; son iyi değer kullanımda kalır.
- Sunucu saati de monoton kaynaktan türetilir (`services/gateway/internal/clock`);
  duvar saati sıçramaları ekseni oynatamaz.

Aynı algoritma iki yerde yaşar ve **birebir aynı tutulmalıdır**:
Go `packages/clocksync` ↔ Dart `apps/participant/lib/core/clock_sync.dart`.

### Kue

Moderatör (Faz 0'da curl) `POST /api/v0/cue` çağırır; gateway
`fire_at = şimdi + delayMs` hesaplar ve `cue_start {run_id, cue_id,
fire_at_server_ms, repeat_seq, payload{color, torch, flash_hz, duration_ms}}`
mesajını **aynı run_id ile 3 kez** (250 ms arayla) yayınlar. İstemci run_id
ile tekilleştirir, `yerelAteşleme = fire_at − ofset` hesaplar ve o andan
itibaren koreografiyi tümüyle yerel saatten akıtır. `flash_hz > 3`
reddedilir (ışığa duyarlılık sınırı).

### Müdahale

`POST /api/v0/intervention {kind: HOLD|STOP|SKIP|BLACKOUT, run_id}` →
`intervention` yayını. STOP/BLACKOUT ekranı karartır ve feneri söndürür;
HOLD son karede dondurur (fener güvenlik gereği söner).

---

## 2. Yazılım içi ön doğrulama (telefonsuz)

```bash
go run ./services/gateway/cmd/gateway            # terminal 1
go run ./tools/loadgen -n 50 -cue -jitter 40     # terminal 2
```

loadgen, N istemciyi aynı süreçte bağlar; her biri bağımsız senkron yapıp
kendi yerel ateşleme anını hesaplar. İstemciler aynı gerçek saati
paylaştığından ateşleme anlarının **yayılımı**, protokolün senkron hatasını
doğrudan verir. `-jitter 40`, yön başına 0–40 ms rastgele gecikmeyle dengesiz
hücresel ağı taklit eder.

Bu ortamda ölçülen referans (2026-08-30, 50 istemci, jitter 40 ms):
yayılım maks−min **17 ms**, σ **3,8 ms** → hedef yazılım katında tutuyor.
Bu araç radyo/ekran gerçekliğini ölçmez; gerçek ölçüm aşağıdadır.

## 2.5 Tarayıcıyla hızlı deneme (uygulama kurmadan, iPhone + Android)

Gateway iki gömülü sayfa sunar:

- **`/` — moderatör mini konsolu:** renk, gecikme, süre, yanıp sönme ve fener
  seçimli kue formu; HOLD/STOP/BLACKOUT/SKIP düğmeleri; canlı istemci sayısı.
  (Asıl Next.js paneli Faz 1'de; bu, curl'ün yerini tutan Faz 0 konsoludur.)
- **`/join` — tarayıcı katılımcısı:** telefon tarayıcısında açılır; saat
  senkronu ve kue zamanlaması Flutter uygulamasıyla aynı algoritmayla çalışır,
  ateşleme anında ekranı boyar.

Akış: gateway'i çalıştırın → bilgisayarda `http://<ip>:8080/` açın →
telefonlarda `http://<ip>:8080/join` açıp "Gösteriye katıl"a basın (telefonlar
sunucuyla aynı Wi-Fi'de olmalı; LTE denemesi için gateway'i internete açık bir
sunucuya koyun) → konsoldan KUE GÖNDER.

Sınırlar: tarayıcıda **fener yoktur** ve ekran kilitlenir/sekme arka plana
düşerse zamanlayıcılar kısılır — deneme boyunca ekran açık ve sayfa önde
kalmalı. Adres şifresiz `http://` olduğu için tarayıcının ekranı otomatik
açık tutma API'si (Wake Lock) çalışmaz; sayfa bunu görüp uyarı gösterir —
**denemeden önce her telefonda ekran kapanma süresini 5 dakika ya da üstüne
çıkarın** (iPhone: Ayarlar → Ekran ve Parlaklık → Otomatik Kilit; Android:
Ayarlar → Ekran → Ekran zaman aşımı). Tarayıcı sayfası ilk elden hızlı
doğrulama içindir; **rapor edilecek ölçüm Flutter release build ile yapılır**
(ekran boyama gecikmesi tarayıcıda daha oynak). Bu sayfa bir deneme aracıdır;
Faz 3'e ertelenen "tarayıcı katılımcı yedeği" ürün kararından bağımsızdır.

## 3. Telefonlarla gerçek deneme

### Hazırlık

1. Gateway'i telefonların erişebildiği bir makinede çalıştırın
   (`TEKSES_ADMIN_TOKEN` ayarlamak isteğe bağlı ama açık ağda önerilir).
2. Uygulamayı kurun: `apps/participant/README.md` (bir kerelik
   `flutter create` + platform dokunuşları), telefonlara **release** modda
   yükleyin: `flutter run --release`.
3. Her telefonda gateway adresini girip katılın; durum satırında
   `ofset ... ms · RTT ... ms` görünene dek bekleyin.
4. Telefonları yan yana dizin, parlaklığı sonuna kadar açın; 240 fps
   (slo-mo) çekim yapan bir kamera hazırlayın.

### Ölçüm

```bash
curl -X POST http://<sunucu>:8080/api/v0/cue \
  -H 'Content-Type: application/json' \
  -d '{"delayMs":5000,"durationMs":4000,"color":"#FF2A2A","torch":true,"flashHz":2}'
```

- Kamera kaydını kue tetiklenmeden önce başlatın.
- Kayıtta her telefonun İLK yanışının karesini işaretleyin;
  `sapma_ms = (kare_farkı / 240) × 1000`.
- En erken ile en geç telefon arasındaki fark ana metriktir; her koşum için
  min/medyan/maks not edin.

### Koşum matrisi

| Koşum | Ağ | Beklenti |
|---|---|---|
| 1 | Aynı Wi-Fi | ≤10–15 ms |
| 2 | LTE (farklı operatörler karışık) | ≤30 ms |
| 3 | LTE + `flashHz:0` (tek yanış, kare işaretlemesi en net) | ≤30 ms |
| 4 | Kue sonrası uçak modu (koreografi sürmeli) | kesinti yok |
| 5 | BLACKOUT müdahalesi | tüm ışıklar ~aynı anda söner |

### Sonuç değerlendirme

- **≤30 ms tutuyorsa:** tez doğrulandı → Faz 1 (MVP) başlar.
- **Tutmuyorsa** sırayla bakılacaklar: telefon başına RTT/ofset kalitesi
  (durum satırı), örnek sayısını 12'ye çıkarma, senkron turu sıklığı,
  ekran yenileme gecikmesi farkları (cihaz sınıfı kalibrasyonu Faz 3
  gündemine alınmıştı), fener sürücü gecikmesi (native kanal Faz 2).

## 4. Faz 0 kapsam sınırı

Ses çalma, gösteri paketi/CDN, katılım kodu, çok oda, ultrasonik beacon ve
moderatör paneli **bilerek yok**. Faz 0 yalnızca tek soruyu yanıtlar:
*dengesiz ağda saat senkronuyla ≤30 ms görsel eşzamanlılık alınabiliyor mu?*

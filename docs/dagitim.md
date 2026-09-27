# Dağıtım — pilot (tek VM + otomatik TLS)

Hedef: sistemi internete açmak ki telefonlar **LTE üzerinden** katılabilsin —
asıl tez (dengesiz hücresel ağda senkron) ancak böyle sınanır. Karar
dokümanındaki ücretsiz plan izlenir: Oracle Cloud Always Free VM (Frankfurt)
+ tek alan adı + Caddy'nin otomatik Let's Encrypt TLS'i. R2/CDN, paket
trafiği büyüyünce aynı sözleşmeyle devreye girecek (blob arayüzü hazır).

## 1. Ön koşullar

- **Oracle Cloud hesabı** → Always Free **ARM VM** (VM.Standard.A1.Flex,
  2 OCPU / 12 GB, Ubuntu 22.04+), Frankfurt bölgesi.
- **Alan adı**: VM'in genel IP'sine A kaydı (ücretsiz: DuckDNS alt alanı).
- VM güvenlik listesinde (VCN → Security List) **80 ve 443/TCP** girişe açık.
  Ubuntu içinde de: `sudo iptables -I INPUT -p tcp --dport 80 -j ACCEPT`
  ve 443 için aynısı (Oracle imajları iptables ile kısıtlı gelir) —
  kalıcılaştırmak için `netfilter-persistent`.

## 2. Kurulum (VM'de)

```bash
# Docker + compose eklentisi
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER && newgrp docker

git clone https://github.com/msaliheroglu/tekses && cd tekses/deploy
cp .env.example .env
nano .env          # TEKSES_DOMAIN, POSTGRES_PASSWORD, TEKSES_ADMIN_TOKEN

docker compose up -d --build
```

İlk açılışta Caddy sertifikayı alır (alan adının IP'ye işaret etmesi
yeterli). Doğrulama:

```bash
curl https://ALAN/healthz                 # gateway: {"status":"ok",...}
curl https://ALAN/api/v1/join/XXXXXX      # control-api: 404 "katılım kodu geçersiz" = ayakta
# tarayıcıda https://ALAN → panel giriş sayfası
```

## 3. Yol tablosu (tek alan adı)

| Yol | Servis |
|---|---|
| `/ws` | gateway (WebSocket) |
| `/api/v0/*` | gateway (kue/müdahale/runs/presence/clockstats — `TEKSES_ADMIN_TOKEN` YA DA geçerli panel oturumu ister) |
| `/control/internal/*` | 404 (iç uçlar internete kapalı; gateway iç ağdan `TEKSES_INTERNAL_TOKEN` ile erişir) |
| `/join` | gateway (tarayıcı katılımcısı: gösteri koreografisi ekran+söz olarak oynar; https olduğu için Wake Lock da çalışır) |
| `/api/v1/*`, `/packages/*`, `/assets/*` | control-api |
| `/control/*` (önek soyulur) | control-api (panelin API çağrıları) |
| `/gw/*` (önek soyulur) | gateway (panelin konsol çağrıları) |
| diğer her şey | moderatör paneli |

## 4. İstemci ayarları

- **Telefon uygulaması:** Gateway `wss://ALAN/ws`, Control `https://ALAN`.
  (TLS'li adreslerde Android cleartext istisnasına gerek kalmaz.)
- **Panel:** `https://ALAN` — Canlı Konsol için panelde oturum açmış olmak
  yeterlidir (gateway, oturumu control-api'ye doğrulatır). Konsoldaki token
  alanı yalnızca panel oturumu olmadan (ör. otomasyon/acil durum) `.env`'deki
  `TEKSES_ADMIN_TOKEN` ile kullanım içindir.
- **QR kodları** derlemeye gömülü `https://ALAN` adresini kodlar
  (compose bunu `TEKSES_DOMAIN`'den geçirir).

## 5. Güncelleme ve bakım

```bash
cd tekses && git pull && cd deploy
docker compose up -d --build     # yalnızca değişen imajlar yeniden derlenir
docker compose logs -f gateway   # canlı log
docker compose exec postgres pg_dump -U postgres tekses > yedek.sql
```

## 6. İsteğe bağlı: otomatik söz çıkarma (deneysel)

Panel, yüklenen sesten zamanlı söz TASLAĞI çıkarabilir. VM'de etkinleştirmek
tek komut (deploy/ dizininde):

```bash
docker compose -f docker-compose.yml -f docker-compose.whisper.yml up -d --build control-api
```

Bu, control-api'yi whisper.cpp + çok dilli "small" model + **Demucs vokal
ayrıştırma** (torch CPU + htdemucs ağırlıkları) gömülü varyantla
(`services/control-api/Dockerfile.whisper`) yeniden derler. İlk derleme
15-25 dk sürer, imaj birkaç GB büyür; sonrasında paneldeki "Sözleri çıkar"
düğmesi çalışır. Geri almak: `docker compose up -d --build control-api`.

Beklentiyi doğru kurun: **vokal ayrıştırma CPU'da ağırdır** — 2 OCPU
Ampere'de 4 dakikalık şarkı 20-40 dk sürebilir; işler tek tek sıraya alınır
(panelde durum "queued"). Always Free sınırı 4 OCPU / 24 GB'dır: VM'i
büyütmek ücretsizdir ve süreyi yaklaşık yarıya indirir (Instance → Edit →
shape). Demucs'suz hızlı mod için compose ekine `TEKSES_NO_DEMUCS: "1"`
ekleyin — ama o zaman mikslenmiş şarkılarda Whisper vokal bulamaz, çıktı
boş kalır. Çıktı her durumda TASLAKTIR; şarkı sözleri için en isabetli yol
**LRC içe aktarma**dır. Çözümleme süresi sınırı `TEKSES_TRANSCRIBE_TIMEOUT`
ile ayarlanır (whisper compose eki 60m verir; varsayılan 10m).

## 7. Sonrası (etkinlik günü ölçeği — Faz 2 devamı)

- Paket indirmeleri R2 + Cloudflare CDN'e taşındı (§8) — etkinlikten önce
  kurulmalı: 60k telefonun ~5 MB'lık indirmesi VM'yi değil R2'yi yorar.
- Etkinlik günü 2–4 geçici gateway düğümü kiralanıp NATS JetStream ile oda
  dağıtımı yapılır (düğüm başına 50–100k bağlantı hedefi).
- Mağaza dağıtımı için Android keystore/iOS imzalama Faz 3.

## 8. İsteğe bağlı: Cloudflare R2 paket/varlık deposu (F3.3)

Boş bırakılırsa her şey bugünkü gibi VM diskinde çalışır. R2'ye geçiş,
paketlerin/ses dosyalarının kalıcılığını VM'den ayırır ve indirmeleri
CDN'e taşır (R2'de çıkış trafiği ücretsizdir — karar dokümanı §4).

1. **R2'yi etkinleştir:** R2, hesap seviyesindedir (bir alan adının içindeyken
   görünmez; sol üstten hesap ana sayfasına dönün). Sol menü → **R2 Object
   Storage**. İlk girişte "Create bucket" YOKTUR — önce **Purchase R2 Plan /
   Get started** ile R2'yi hesaba ekleyin. Bu adım ücretsiz kota için de bir
   **ödeme yöntemi** (kart/PayPal) ister; kota (10 GB depolama, 1M yazma +
   10M okuma/ay) aşılmadıkça tahsilat yapılmaz, çıkış trafiği her durumda
   ücretsizdir.
2. **Kova:** artık görünen **Create bucket** → isim `tekses`, konum ipucu
   **EU** (VM Frankfurt'ta), sınıf Standard.
3. **API anahtarı:** R2 sayfası → **API → Manage API tokens** → Create API
   Token, izin "Object Read & Write", kapsam yalnız bu kova. Çıkan Access
   Key ID / Secret Access Key bir kez gösterilir — hemen not edin. S3 uç
   noktası (`https://<hesap-id>.r2.cloudflarestorage.com`) aynı ekranda yazar.
4. **Herkese açık erişim** — üç seçenek (DuckDNS alt alanı Cloudflare'a
   taşınamadığı için "kendi alan adın Cloudflare DNS'te" şartına dikkat):
   - **A — public erişim yok (en kolay):** `TEKSES_ASSET_PUBLIC_BASE`'i hiç
     yazmayın. Dosyalar R2'de durur (kalıcılık ✓) ama telefonlara
     control-api üzerinden servis edilir; prova için yeterli, CDN kazanımı yok.
   - **B — r2.dev alt alanı:** kova → Settings → **Public Development URL**
     (eski adı "Public access / R2.dev subdomain") → Enable → onay kutusuna
     `allow` yazıp onaylayın; çıkan `https://pub-….r2.dev` adresini
     `TEKSES_ASSET_PUBLIC_BASE` yapın. Hız sınırlıdır: prova için olur,
     etkinlik için olmaz.
   - **C — özel alan adı (etkinlik için şart):** Cloudflare DNS'te duran bir
     alan adı (ör. Cloudflare Registrar'dan ~10 $/yıl) → kova → Settings →
     Custom Domains → Connect domain (`cdn.ornek.com`). Gateway/panelin
     DuckDNS adresi aynen kalır; bu alan adı yalnız dosya indirmedir.
5. `deploy/.env` içine:

   ```
   TEKSES_BLOB_S3_ENDPOINT=https://<hesap-id>.r2.cloudflarestorage.com
   TEKSES_BLOB_S3_BUCKET=tekses
   TEKSES_BLOB_S3_ACCESS_KEY_ID=...
   TEKSES_BLOB_S3_SECRET_KEY=...
   TEKSES_ASSET_PUBLIC_BASE=https://cdn.ornek.com   # yalnız B/C seçeneğinde
   ```

6. `docker compose up -d --build control-api` — günlükte
   `paket deposu: s3/r2` görünmeli.
7. **Eski dosyaların taşınması** (daha önce yerel diske yayın yapıldıysa):
   anahtar düzeni birebir aynıdır, kopyalamak yeter:

   ```bash
   docker compose cp control-api:/data/packages ./packages-yedek
   # rclone ile: rclone copy ./packages-yedek r2:tekses
   ```

   Taşımak istemezseniz gösteriyi panelden bir kez yeniden yayınlamak da
   dosyaları R2'ye yazar.
8. Doğrulama: panelden gösteriyi bir kez yeniden yayınlayın ve ses yükleyin
   → yanıttaki `url` CDN tabanlı olmalı (A seçeneğinde control-api tabanlı);
   telefonla katılın → şarkı çalmalı. `TEKSES_ASSET_PUBLIC_BASE`'i kaldırıp
   yalnız S3 değişkenlerini bırakmak da geçerlidir: dosyalar R2'de durur ama
   telefonlara control-api üzerinden (vekil gibi) servis edilir.

Not: `TEKSES_ASSET_PUBLIC_BASE` ayarlıyken join yanıtı `asset_base_url` ve
mutlak `manifest_url` taşır; telefon uygulaması bunları doğrudan kullanır
(SHA-256 doğrulaması kaynaktan bağımsız aynıdır). Tarayıcı /join sayfası
kueleri telden aldığı için etkilenmez.

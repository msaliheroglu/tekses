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
| `/api/v0/*` | gateway (kue/müdahale/runs — `TEKSES_ADMIN_TOKEN` YA DA geçerli panel oturumu ister) |
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

Bu, control-api'yi whisper.cpp + çok dilli "small" model gömülü varyantla
(`services/control-api/Dockerfile.whisper`) yeniden derler. İlk derleme
10-15 dk sürer, imaj ~1 GB büyür; sonrasında paneldeki "Sözleri çıkar"
düğmesi çalışır. Geri almak: `docker compose up -d --build control-api`.

Beklentiyi doğru kurun: **mikslenmiş şarkılarda taslak kalitesi düşüktür** —
Whisper vokali enstrümandan ayıramaz; vokal ayrıştırma (Demucs) CPU'da tek
şarkıda bile zaman aşımını aştığı için bu imaja bilerek konmadı. Tezahürat,
anons gibi konuşma ağırlıklı seslerde iyi sonuç verir. Şarkı sözleri için en
isabetli yol **LRC içe aktarma**dır (panelde her zaman çalışır). Yüksek
kaliteli otomatik taslak isteyenler için Demucs'lu yerel boru hattı Windows'ta
çalıştırılabilir: `deploy/transcribe-whisper.ps1` başındaki adımlar +
`TEKSES_TRANSCRIBER=...\deploy\transcribe-whisper.bat` (yerel control-api ile).
Çözümleme süresi sınırı `TEKSES_TRANSCRIBE_TIMEOUT` ile ayarlanır
(whisper compose eki 30m verir; varsayılan 10m).

## 7. Sonrası (etkinlik günü ölçeği — Faz 2 devamı)

- Paket indirmeleri R2 + Cloudflare CDN'e taşınır (blob arayüzü hazır;
  yalnızca R2 sürücüsü ve `manifest_url`'in mutlak CDN adresi gerekir).
- Etkinlik günü 2–4 geçici gateway düğümü kiralanıp NATS JetStream ile oda
  dağıtımı yapılır (düğüm başına 50–100k bağlantı hedefi).
- Mağaza dağıtımı için Android keystore/iOS imzalama Faz 3.

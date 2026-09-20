# Yük testi (F2.4) — VM üzerinde kademeli koşum

Amaç: tek gateway'in kaç eşzamanlı WebSocket katılımcısını taşıdığını ve
ölçek büyüdükçe senkron yayılımının (ateşleme anı maks−min) bozulup
bozulmadığını ölçmek. Hedef: 80k telefona giden yolda VM'in sınırını bilmek.

Yöntem: loadgen, gateway'e **compose ağı üzerinden** bağlanır (Caddy/TLS'e
uğramaz — ölçülen şey gateway'dir; TLS el sıkışma maliyeti ayrı bir konudur
ve etkinlik günü CDN/çoklu düğümle çözülür). Üreteç ve gateway aynı VM'de
koşar: CPU'yu paylaşırlar, yani sonuçlar MUHAFAZAKÂRDIR — gerçek kapasite
bundan düşük değil, yüksek olur.

## 0. Hazırlık (bir kez)

```bash
cd ~/tekses && git pull

# Bağlantı izleme tablosu: on binlerce bağlantı conntrack'e sığmalı.
sudo sysctl -w net.netfilter.nf_conntrack_max=524288

# Loadgen imajı:
docker build -f tools/loadgen/Dockerfile -t tekses-loadgen .

# Gateway'i güncel kodla ve yeni dosya limitiyle yeniden yarat:
cd deploy
docker compose -f docker-compose.yml -f docker-compose.whisper.yml up -d --build gateway

# Yönetici token'ı kabuk değişkenine al (kue tetiklemek için):
TOKEN=$(grep '^TEKSES_ADMIN_TOKEN=' .env | cut -d= -f2-)
```

## 1. Koşum komutu

Her kademe aynı komuttur; yalnız `-n` değişir. `tmux` içinde çalıştırın.

```bash
docker run --rm --network tekses_default \
  --ulimit nofile=200000:200000 \
  --sysctl net.ipv4.ip_local_port_range="1024 65000" \
  tekses-loadgen \
  -server ws://gateway:8080/ws -wire proto \
  -n 10000 -ramp 1000 \
  -cue -cueDelay 5000 -adminToken "$TOKEN" -waitCue 300s
```

Koşum sürerken **ikinci bir SSH penceresinde** kaynakları izleyin ve en
yüksek değerleri not edin:

```bash
docker stats --no-stream tekses-gateway-1
free -h
```

## 2. Kademeler

| Kademe | -n | -ramp | Not |
|---|---|---|---|
| Isınma | 2000 | 500 | Geliştirme ortamı sonucuyla karşılaştırma (yayılım ~3 ms idi) |
| 1 | 10000 | 1000 | |
| 2 | 20000 | 1500 | |
| 3 | 40000 | 2000 | 2 OCPU'da senkron turu uzayabilir; sabırla bekleyin |

Her kademede not alın: başarılı istemci sayısı, **ateşleme yayılımı**
(maks−min ve p95−p5), en iyi RTT medyanı, gateway CPU % ve RSS, VM boş RAM.
Yayılım ≤30 ms kaldıkça ölçek o kademede "geçti" demektir; hatalar
(bağlantı reddi, zaman aşımı) hangi kademede başlıyorsa VM sınırı orasıdır.

Sınırlar: tek üreteç konteynerinden tek hedefe ~64k yerel port sığar; 40k
üstüne çıkmak için ya ikinci bir loadgen konteyneri paralel koşun ya da
VM'i 4 OCPU / 24 GB'a büyütüp (Always Free sınırı, ücretsiz) tek seferde
deneyin. 100k tam koşumu için ayrı bir yük makinesi (ikinci Always Free
VM) doğru araçtır — gateway'le CPU paylaşımı da ortadan kalkar.

## 3. Temizlik / dikkat

- Koşum bitince konteyner kendini siler (`--rm`); gateway'de kalıcı iz
  yalnızca log ve runs halkasıdır.
- Yük sırasında gerçek oda/etkinlik KULLANMAYIN: loadgen varsayılan
  "faz0" odasında yaşar, `-cue` tüm odalara değil kendi tetiklediği kueye
  bakar; yine de canlı bir deneme ile üst üste bindirmeyin.
- Koşumdan sonra `docker compose ps` ile beş servisin sağlıklı olduğunu
  doğrulayın.

Sonuçlar `docs/faz1-plan-ve-durum.md` F2.4 maddesine işlenir.

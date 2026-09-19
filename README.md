# TekSes

Stadyum, konser, miting ve konferans gibi kalabalık etkinliklerde bir moderatörün
on binlerce seyircinin telefonunda **senkronize tezahürat, marş ve şarkı
koreografisi** yürütmesini sağlayan gerçek zamanlı platform.

**Yönetici ilke:** Ağ, hazırlık içindir; icra için değil. Telefon odaya girerken
gösteri paketini, saat ofsetini ve zaman çizelgesini önceden edinir; kue anında
hiçbir bağlantıya ihtiyaç duymaz.

Proje özeti ve alınan kararlar: [`docs/tekses-proje-ozeti-ve-kararlar.md`](docs/tekses-proje-ozeti-ve-kararlar.md)

## Monorepo düzeni

| Yol | İçerik | Durum |
|---|---|---|
| [`apps/participant`](apps/participant) | Flutter katılımcı uygulaması (Android + iOS) | Faz 0: saat senkronu + ekran/fener kue |
| [`apps/moderator`](apps/moderator) | Next.js moderatör paneli | Faz 1'de başlayacak |
| [`services/control-api`](services/control-api) | Go kontrol API'si (REST) | Faz 1'de başlayacak |
| [`services/gateway`](services/gateway) | Go WebSocket gateway (saat senkronu, kue yayını) | Faz 0: çalışıyor |
| [`packages/proto`](packages/proto) | Protobuf mesaj sözleşmeleri + Faz 0 JSON tel türleri | Sözleşmeler yazıldı |
| [`tools/loadgen`](tools/loadgen) | Go yük üreteci / senkron ölçüm istemcisi | Faz 0: yazılım içi ölçüm |

## Faz 0 — Senkron denemesi

Amaç: zamanlanmış zaman çizelgesi + saat senkronu tezini ucuza doğrulamak.
Belirlenen sunucu saatinde 5–10 telefonun ekranı ve feneri aynı anda yanar;
240 fps kamerayla filme alınıp gerçek sapma ölçülür. Hedef: **≤30 ms**.

Adım adım kılavuz: [`docs/faz0-senkron-denemesi.md`](docs/faz0-senkron-denemesi.md)

Hızlı başlangıç (gateway + yazılım içi ölçüm):

```bash
# 1. Gateway'i başlat
go run ./services/gateway/cmd/gateway

# 2. Ayrı bir terminalde: 50 simüle istemciyle senkron sapmasını ölç
go run ./tools/loadgen -n 50 -server ws://localhost:8080/ws -cue

# 3. Telefonlar bağlıyken kue tetikle
curl -X POST http://localhost:8080/api/v0/cue \
  -H 'Content-Type: application/json' \
  -d '{"delayMs":3000,"durationMs":4000,"color":"#FF2A2A","torch":true,"flashHz":2}'
```

Kurulumsuz deneme (uygulama derlemeden, iPhone + Android):

- Moderatör mini konsolu: bilgisayarda `http://<sunucu-ip>:8080/`
- Katılımcı deneme sayfası: telefon tarayıcısında `http://<sunucu-ip>:8080/join`
  (aynı Wi-Fi'de; fener yalnızca Flutter uygulamasında çalışır)

## Geliştirme

- Go modülü depo kökündedir: `go build ./...`, `go test ./...`, `go vet ./...`
- Yol haritası: Faz 0 (senkron denemesi) → Faz 1 (MVP) → Faz 2 (ölçek ve ses) → Faz 3 (sertleştirme). Ayrıntılar karar dokümanında.

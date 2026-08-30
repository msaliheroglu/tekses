# TekSes — Claude oturum notları

- **Kullanıcıyla Türkçe konuş.** Kod tanımlayıcıları İngilizce, yorumlar ve dokümanlar Türkçe.
- Tüm proje kararları ve yol haritası: `docs/tekses-proje-ozeti-ve-kararlar.md`.
  Yeni bir karar alındığında veya bir karar değiştiğinde bu dosyayı güncelle —
  oturumlar arası hafıza bu depodur.
- **Aktif iş durumu: `docs/faz1-plan-ve-durum.md`.** Oturum yeni başladıysa ya
  da "devam et" dendiyse önce onu oku, işaretsiz ilk adımdan sür. Her anlamlı
  adım ayrı commit + anında push (kota/oturum her an kesilebilir).
- Ayrıntılı mimari doküman (`tekses-architecture-v0.1.md`) henüz depoda değil;
  kullanıcı paylaştığında `docs/architecture/` altına eklenmeli.

## Teknik çerçeve (özet)

- Yığın: Flutter (mobil) + Go (backend) + Next.js (panel) + Postgres + NATS JetStream + R2/CDN.
- Senkron: zamanlanmış zaman çizelgesi + NTP benzeri saat senkronu (birincil),
  ultrasonik beacon (ikincil). Cihazda monoton saat; duvar saati asla.
- Kue anında ağa ihtiyaç yok; her şey önceden telefona iner.
- Hassasiyet hedefi: ekran/fener ≤30 ms. Akustik birlik vaat edilmez.

## Depo pratikleri

- Go modülü depo kökünde tektir (`github.com/msaliheroglu/tekses`); servisler
  `services/`, araçlar `tools/`, paylaşılan Go paketleri `packages/` altında.
- Protobuf sözleşmeleri `packages/proto/tekses/v1/` — telin gerçeği bunlardır.
  Faz 0 JSON tel türleri (`packages/proto/wire`) proto alan adlarını birebir izler.
- Doğrulama: `go build ./... && go vet ./... && go test ./...`
- Flutter ve protoc bu geliştirme ortamında yok; Dart kodu telefonda/CI'da doğrulanır.

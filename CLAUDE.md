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
  Tel v1 = JSON (Flutter/tarayıcı), v2 = ikili protobuf; gateway ikisini aynı
  anda konuşur, hello çerçevesinin biçimi kodeki belirler. Go stub'ları
  `gen/go` altına commit'lidir; yenileme: `buf generate` (packages/proto
  README'sindeki kurulum notu).
- Doğrulama: `go build ./... && go vet ./... && go test ./...`
- Flutter bu geliştirme ortamında yok; Dart kodu telefonda/CI'da doğrulanır.

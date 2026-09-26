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
- Flutter bu ortamda kurulu DEĞİL ama **indirilebilir** (~3 dk): SDK'yı
  scratchpad'e açıp `flutter analyze` + `flutter test` burada koşulur. Dart
  değişikliklerini CI'ya (Katılımcı APK iş akışı) göndermeden önce yerelde
  doğrula — CI turu ~10 dk, yerel tur ~1 dk:

  ```bash
  cd "$SCRATCH" && curl -sSfL "https://storage.googleapis.com/flutter_infra_release/releases/$(
    curl -sSf https://storage.googleapis.com/flutter_infra_release/releases/releases_linux.json |
    python3 -c 'import json,sys;d=json.load(sys.stdin);h=d["current_release"]["stable"];print([r["archive"] for r in d["releases"] if r["hash"]==h][0])'
  )" | tar -xJ && git config --global --add safe.directory '*'
  export PATH="$SCRATCH/flutter/bin:$PATH"
  cd apps/participant && flutter pub get && flutter analyze && flutter test
  ```

  (Cihaz gerektiren şeyler — mikrofon, fener, gerçek zamanlama — yine
  telefonda doğrulanır; `flutter test` yalnızca saf Dart mantığını kapsar.)
- **analyze + test YETMEZ:** eklentilerin platform gerçeklemeleri (record_linux
  vb.) yalnızca `flutter build apk` Dart derlemesinde ele alınır; uyumsuz bir
  alt paket sürümü analyze/test'ten temiz geçip APK'yı kırar. Eklenti
  ekledikten/sürüm değiştirdikten sonra "Katılımcı APK" iş akışını koştur
  (~3 dk) — yerelde Android SDK yok.

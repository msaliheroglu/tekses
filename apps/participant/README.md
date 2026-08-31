# apps/participant — TekSes katılımcı uygulaması (Faz 0)

Minimal Flutter uygulaması: gateway'e bağlanır, NTP benzeri saat senkronu
yapar, `CueStart` alınca belirlenen sunucu anında ekranı boyar ve feneri
yakar. Kue alındıktan sonra koreografi tümüyle yerel monoton saatten akar;
kue anında ağ gerekmez.

## Modüller

| Dosya | Görev |
|---|---|
| `lib/core/mono_clock.dart` | Tek monoton saat (duvar saati asla kullanılmaz) |
| `lib/core/messages.dart` | Faz 0 JSON tel türleri (proto şemasını izler) |
| `lib/core/clock_sync.dart` | Ofset kestirimi — Go `packages/clocksync` ile birebir aynı algoritma |
| `lib/core/realtime_client.dart` | WS bağlantısı, jitter'lı yeniden bağlanma, senkron turları |
| `lib/core/cue_arbiter.dart` | "İlk gelen kazanır, sonra sadık kal" kaynak kilidi |
| `lib/core/cue_scheduler.dart` | Timer + sıkı bekleme ile ~1 ms hassas ateşleme |
| `lib/core/package_store.dart` | Katılım kodu çözümü + paket indirme + SHA-256 doğrulama |
| `lib/core/show_manifest.dart` | Gösteri manifesti modeli (packages/manifest şemasını izler) |
| `lib/core/timeline_engine.dart` | Saf zaman çizelgesi motoru: elapsed → söz + ışık karesi (testli) |
| `lib/core/torch_service.dart` | Fener denetimi (Faz 2'de native kanala taşınacak) |
| `lib/ui/` | Katılım (kodlu/kodsuz) ve gösteri ekranları |

Katılım kodu girilirse paket control-api'den iner, özetle doğrulanır ve
`cue_id` manifestteki bir sekansa denk gelen kueler zaman çizelgesi olarak
(sözler + kue şeritleri) oynar; kod boşsa Faz 0 davranışı sürer. Birim
testler: `flutter test` (bu depo ortamında Flutter yok; telefonda/CI'da koşar).

## Kurulum (bir kez)

Depoda yalnızca Dart kaynakları tutulur; Android/iOS iskeletini Flutter üretir:

```bash
cd apps/participant
flutter create --project-name tekses_participant --org app.tekses --platforms android,ios .
flutter pub get
```

Sonrasında iki platform dokunuşu gerekir:

- **Android** — Faz 0 yerel ağda şifresiz `ws://` kullanır:
  `android/app/src/main/AndroidManifest.xml` içindeki `<application ...>`
  etiketine `android:usesCleartextTraffic="true"` ekleyin.
- **iOS** — `ios/Runner/Info.plist` içine fener için kamera açıklaması ve
  Faz 0 için ATS istisnası ekleyin:

  ```xml
  <key>NSCameraUsageDescription</key>
  <string>Telefon fenerini gösteri koreografisi için kullanır.</string>
  <key>NSAppTransportSecurity</key>
  <dict>
    <key>NSAllowsLocalNetworking</key>
    <true/>
  </dict>
  ```

## Çalıştırma

```bash
flutter run --release   # zamanlama ölçümü daima release modda yapılır
```

Uygulamada gateway adresini girin (ör. `ws://192.168.1.10:8080/ws`), ekran
parlaklığını elle sonuna kadar açın. Deneme akışı için:
[`docs/faz0-senkron-denemesi.md`](../../docs/faz0-senkron-denemesi.md)

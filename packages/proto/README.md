# packages/proto — Mesaj sözleşmeleri

Telin gerçeği `tekses/v1/*.proto` dosyalarıdır:

- `clock.proto` — NTP benzeri saat senkronu (`ClockSyncRequest/Response`).
- `cue.proto` — `CueStart`, `CuePayload`, `Intervention` (HOLD / STOP / SKIP / BLACKOUT).
- `envelope.proto` — `Hello`, `Welcome` ve tüm mesajları saran `Envelope`.

## Tel sürümleri

Gateway iki kodeği aynı anda konuşur; istemcinin **hello çerçevesinin biçimi**
kodeki belirler:

| Sürüm | Çerçeve | Kodlama | Kim kullanıyor |
|---|---|---|---|
| v1 | metin | JSON (proto alan adlarını izler) | tarayıcı `/join` sayfası |
| v2 | ikili | protobuf `Envelope` (~56 bayt/kue; JSON ~207) | Flutter uygulaması (F3.0'dan beri), loadgen (`-wire proto`) |

İki gerçekleme `wire/testdata/golden_frames.txt` altın baytlarına karşı test
edilir (Go: `wire/golden_test.go`; Dart: `apps/participant/test/
wire_binary_test.dart`) — şema değişince dosya `-update` ile tazelenir ve iki
taraftaki testler birlikte güncellenir.

Go türleri `wire/` paketindedir: JSON yapıları + `Encode/Decode(Message)` ve
protobuf köprüsü `EncodeBinary/DecodeBinary` (`binary.go`). İki kodek aynı
wire yapılarıyla çalışır; çağıran kod kodekten bağımsızdır.

## Kod üretimi

Go stub'ları `gen/go/` altına **commit edilir** (derleme protoc istemez).
Şema değiştiğinde:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install github.com/bufbuild/buf/cmd/buf@latest
cd packages/proto && buf lint && buf generate
```

Dart stub'ları Flutter kurulu ortamda üretilir (Flutter'ın v2'ye geçişi için):

```bash
dart pub global activate protoc_plugin
cd packages/proto && buf generate --template buf.gen.dart.yaml
```

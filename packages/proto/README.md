# packages/proto — Mesaj sözleşmeleri

Telin gerçeği `tekses/v1/*.proto` dosyalarıdır:

- `clock.proto` — NTP benzeri saat senkronu (`ClockSyncRequest/Response`).
- `cue.proto` — `CueStart`, `CuePayload`, `Intervention` (HOLD / STOP / SKIP / BLACKOUT).
- `envelope.proto` — `Hello`, `Welcome` ve tüm mesajları saran `Envelope`.

## Faz 0 tel biçimi: JSON

Bu geliştirme aşamasında WebSocket teli **JSON**'dur ve proto şemasını alan
adlarıyla (snake_case) birebir izler; Go türleri `wire/` paketindedir ve hem
gateway hem loadgen tarafından kullanılır. 5–10 telefonluk denemede kodlama
biçimi ölçümü etkilemez.

**Faz 1'de** ikili protobuf'a geçilir (~40 bayt/kue): `buf generate` ile Go
stub'ları `gen/go/` altına, Dart stub'ları Flutter araç zinciriyle
`apps/participant/lib/gen/` altına üretilecek. Şema değişikliği yalnızca
`.proto` dosyalarında yapılır; `wire/` paketi o geçişte üretilmiş koda
devrolur.

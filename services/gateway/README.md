# services/gateway — WebSocket gateway (Faz 0)

Durumsuz gateway: saat senkronu yanıtlar, kue ve müdahale yayınlar.

```bash
go run ./services/gateway/cmd/gateway -addr :8080
```

| Uç | Görev |
|---|---|
| `GET /` | Moderatör mini konsolu (kue formu, müdahale düğmeleri, canlı istemci sayısı) |
| `GET /join` | Tarayıcı katılımcı deneme sayfası (telefonda kurulumsuz; fener yok) |
| `GET /healthz` | Sağlık + bağlı istemci sayısı + sunucu saati |
| `GET /ws` | Katılımcı WebSocket'i (hello, saat senkronu, kue alımı) |
| `POST /api/v0/cue` | Kue tetikle (`delayMs`, `durationMs`, `color`, `torch`, `flashHz`) |
| `POST /api/v0/intervention` | `HOLD` / `STOP` / `SKIP` / `BLACKOUT` yayınla |

`TEKSES_ADMIN_TOKEN` ayarlıysa `/api/*` uçları `Authorization: Bearer <token>`
ister. `TEKSES_CONTROL_URL` ayarlıysa hello'daki `join_code` control-api'den
odaya çözülür ve kue/müdahale istekleri `room_id` ile odaya daraltılabilir;
ayarsızsa herkes `faz0` odasına düşer (yerel deneme). Protokol ayrıntısı:
[`docs/faz0-senkron-denemesi.md`](../../docs/faz0-senkron-denemesi.md).

Faz 1+ yolu: oda başına hub, NATS JetStream dağıtımı, protobuf tel, düğüm
başına 50–100k bağlantı hedefi (karar dokümanı §3).

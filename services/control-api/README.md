# services/control-api — Kontrol API'si

Moderatör tarafının REST API'si: organizasyon/etkinlik/oda/gösteri yönetimi,
katılım kodu üretimi, değişmez gösteri sürümleri.

```bash
go run ./services/control-api/cmd/control-api -addr :8090
```

Depolama şimdilik bellek içi (`memstore`); Postgres kalıcılığı Faz 1 Adım 5'te
aynı `store.Store` arayüzünün arkasına gelecek.

| Uç | Görev |
|---|---|
| `POST /api/v1/auth/register` | Organizasyon + kullanıcı kaydı `{organization, email, password}` → token |
| `POST /api/v1/auth/login` | Giriş → token |
| `GET/POST /api/v1/events` | Etkinlik listesi / oluşturma (Bearer token) |
| `GET /api/v1/events/{id}` | Etkinlik ayrıntısı |
| `GET/POST /api/v1/events/{id}/rooms` | Oda listesi / oluşturma (joinCode üretilir) |
| `GET/POST /api/v1/shows` | Gösteri listesi / oluşturma |
| `GET/POST /api/v1/shows/{id}/versions` | Sürüm listesi / manifest yayınlama (doğrulama + kanonik JSON + SHA-256) |
| `GET /api/v1/show-versions/{id}` | Sürüm + tam manifest |
| `POST /api/v1/rooms/{id}/activate` | Odada gösteri sürümünü etkinleştir |
| `GET /api/v1/join/{code}` | **Herkese açık:** oda + aktif sürüm + manifest (telefon katılımı) |

Manifest sözleşmesi `packages/manifest` paketindedir; sürümler değişmezdir
(yayınlandıktan sonra hiçbir uç manifest baytlarını değiştiremez, telefon
SHA-256 ile doğrular).

Kiracılık: her istek oturumun organizasyonuna daraltılır; başka kiracının
kaynağı 404 görünür. Katılım kodları karışması kolay karakterler (0/O, 1/I/L)
içermeyen 6 haneli alfabeyle üretilir.

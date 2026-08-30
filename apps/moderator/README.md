# apps/moderator — Moderatör paneli (Next.js)

Faz 1 MVP: giriş/organizasyon kaydı, etkinlik + oda yönetimi (katılım kodu ve
QR), manifest yayınlama (JSON editörü), sürüm etkinleştirme ve canlı konsol
(GO / HOLD / STOP / BLACKOUT). Dalga formu üzerinde söz zamanlama ve LRC içe
aktarma sonraki yinelemede.

## Çalıştırma

```bash
cd apps/moderator
npm install
npm run dev        # http://localhost:3000
```

Panel, control-api ve gateway'e Next.js rewrites üzerinden vekillenir
(tarayıcı hep aynı origin'i görür, CORS gerekmez). Varsayılanlar yereldir;
ortam değişkenleriyle değiştirilir:

| Değişken | Varsayılan | Ne için |
|---|---|---|
| `CONTROL_API_URL` | `http://localhost:8090` | `/control/*` vekilinin hedefi |
| `GATEWAY_URL` | `http://localhost:8080` | `/gw/*` vekilinin hedefi (canlı konsol) |
| `NEXT_PUBLIC_GATEWAY_PUBLIC_URL` | `http://localhost:8080` | QR'ın kodladığı, telefonlardan erişilen adres |

Uçtan uca yerel akış: `control-api`yi ve `TEKSES_CONTROL_URL=http://localhost:8090`
ile `gateway`'i başlatın → panelde kaydolun → etkinlik + oda oluşturun →
telefon QR'ı okutup `/join?code=…` sayfasına gelir → gösteri yayınlayıp odada
etkinleştirin → Canlı Konsol'dan GO.

Dağıtım hedefi (karar): Vercel Hobby veya Cloudflare Pages.

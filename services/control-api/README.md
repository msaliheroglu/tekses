# services/control-api — Kontrol API'si

**Faz 1'de başlayacak.** REST API: etkinlik/gösteri yönetimi, paketleme,
katılım kodu + QR, zamanlama, canlı konsol, telemetri. Çok kiracılı
(Organization → Event → Room), pilot için e-posta + şifre girişi.
Veri modeli karar dokümanı §3'te.

Faz 0'da moderatör konsolunun yerini gateway'in `POST /api/v0/cue` ucu tutar.

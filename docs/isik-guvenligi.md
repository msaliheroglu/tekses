# Işığa duyarlılık ve fener güvenliği (F3.2 incelemesi)

Tarih: 2026-09-26. Kapsam: yanıp sönen ışığın (ekran + fener) fotosensitif
epilepsi riski ve fenerin ısıl yükü. On binlerce telefonun aynı anda yanıp
söndüğü bir gösteride bu bir ayrıntı değil, tasarım sınırıdır.

## Kural: flash_hz ≤ 3

Fotosensitif nöbet riski ~3 Hz üzerinde belirgin biçimde artar; erişilebilirlik
standartları da (WCAG 2.3.1 "üç yanıp sönme" eşiği) saniyede üçten sık flaşı
sınır kabul eder. TekSes'te sınır **3 Hz** ve %50 görev döngüsüdür; sistem
bunun üstünü hiçbir katmanda taşımaz.

### Katman katman denetim (savunma derinliği)

| Katman | Nerede | Davranış |
|---|---|---|
| Panel editörü | `apps/moderator` | Form 0..3 üretir; yayında sunucu yine doğrular. |
| Manifest doğrulaması | `packages/manifest` (`MaxFlashHz`) | `flash_hz` 0..3 dışıysa yayın REDDEDİLİR. |
| Gateway canlı kue | `handleCue` | `flashHz > 3` istek reddedilir (sessizce kırpılmaz — moderatör sınırı bilerek tasarlasın). |
| Telefon | `show_manifest.clampFlashHz` (manifest) + `messages.dart` (tel yükü) | Gelen değere GÜVENMEZ: 0..3'e kelepçeler. Bozuk/eski sunucu ya da kurcalanmış manifest telefonu hızlı flaşa döndüremez. |
| Tarayıcı /join | `join.html clampFlashHz` | Aynı kelepçe. |

Sınır değişecekse dört yerde birden değişmeli: `packages/manifest.MaxFlashHz`,
gateway `maxFlashHz`, telefon `clampFlashHz`, join.html `clampFlashHz`.

## Işığa duyarlı mod (telefonda, seyircinin kendi seçimi)

Gösteri ekranının sol altındaki **"ışığa duyarlı mod"** anahtarı yanıp sönmeyi
SABİT ışığa indirger: renk, kue süresi ve fener kararı aynı kalır, yalnızca
flaş fazı kalkar (telefon hep "yanık" yarımda kalır). Gösteri ortasında da
açılabilir ve süren koşuyu anında etkiler. Katılım ekranındaki kalıcı uyarı
seyirciyi bu anahtara yönlendirir.

Bilinçli sınır: mod cihaz yerelidir (sunucu bilmez) ve koreografinin
"karanlık" anlarını değiştirmez — yalnız flaş kaynaklı karanlık yarımları
kaldırır; tasarımdaki uzun karartmalar tasarlandığı gibi kalır.

## Fener ısıl yükü (inceleme sonucu)

- Fener ya sabittir ya ≤3 Hz'te %50 görev döngüsüyle yanıp söner; flaşlı
  bölümler ısıl olarak sabit yanmanın yarısıdır.
- Android/iOS fener LED'i donanım/sürücü katmanında ısıl kısılmaya tabidir;
  cihaz kendini korur (parlaklık düşer, uçlarda sistem feneri kapatır).
- **Kod sınırı bilinçli olarak YOK:** gösteri ortasında sürpriz fener kesintisi,
  ısıl riskten daha kötü bir sahne sonucudur; karar moderatör tasarımına
  bırakıldı.
- **Tasarım kılavuzu (manifest yazarına):** kesintisiz SABİT fener bölümünü
  ~3 dakikanın altında tutun, uzun bölümlerde flaşlı ya da fenersiz aralar
  bırakın. Cihaz sınıfına göre ölçüm F3.5 fener kalibrasyonuyla birlikte
  ele alınacak.
- Acil durumda **BLACKOUT** müdahalesi tüm fenerleri anında söndürür; HOLD da
  güvenlik gereği feneri kapatır (ekran son karede kalır).

## Moderatör sorumlulukları

- 3 Hz sınırı sistemin tavanıdır, hedefi değil: flaşı dramaturjinin
  gerektirdiği kadar kullanın.
- Etkinlik duyurusuna "gösteri yanıp sönen ışık içerir" uyarısını ekleyin;
  uygulama içi uyarı katılım ekranında hazır.
- Nöbet şüphesinde BLACKOUT'a basın — tüm ışıklar anında söner.

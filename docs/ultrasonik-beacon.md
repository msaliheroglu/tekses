# Ultrasonik beacon (F2.7) — sinyal sözleşmesi ve PA saha testi

Amaç: WebSocket'i kopmuş telefonlara kueyi mekân hoparlöründen (PA)
**duyulmaz** bantta ulaştırmak. Bu bir ERİŞİLEBİLİRLİK YEDEĞİDİR: ses hızı
(~343 m/sn → 100 m'de ~290 ms) ve yankı nedeniyle ±300 ms hata taşır;
hassasiyet kaynağı WS + saat senkronudur ve Cue Arbiter, WS kaynağı varken
beacon'ı yok sayar (karar dokümanı §3).

## Sinyal sözleşmesi (v1)

Referans gerçekleme ve testler: `packages/beacon` (Go). Telefon dinleyicisi
bu sözleşmenin Dart portu olacak.

- Bant: 18,5–20 kHz (çoğu yetişkin duymaz; telefon mikrofonları 48 kHz'te alır).
- Patlama = **120 ms chirp** (18500→19900 Hz, algılama + zaman senkronu)
  + 10 ms boşluk + **34 bit FSK** (sembol 20 ms; bit0 = 18,6 kHz, bit1 = 19,4 kHz).
- Yük: version(4) + cue_index(8) + seq(4) + geri sayım(10; 100 ms birim) + CRC-8(8).
- **cue_index eşlemesi:** 0 = otomatik program; i>0 = manifestteki
  `sequences[i-1]`. (Beacon bandı string kimlik taşıyamayacak kadar dar.)
- **Geri sayım, chirp'in BAŞLANGICINA göredir:** telefon, chirp'i duyduğu
  yerel ana geri sayımı ekleyerek ateşleme anını bulur.
- Yinelemeler ~1 sn arayla, her birinde geri sayım düşerek gider; `seq`
  telefon tarafında run tekilleştirmesine girer.

## Araç

```bash
# PA'dan çalınacak beacon dosyası (ateşleme = dosya başlangıcı + 3 sn):
go run ./tools/beacon -out beacon.wav -cue 0 -countdown 3000 -repeats 3

# PA test kiti (tonlar + süpürme + örnek beacon):
go run ./tools/beacon -patest pa-kiti

# Telefon kaydında beacon ara (16-bit WAV, mono/stereo, ≥40 kHz):
go run ./tools/beacon -decode kayit.wav
```

## PA saha testi (etkinlikten önce ŞART)

Çoğu PA zinciri 16 kHz üstünü keser (hoparlör sürücüsü, DSP/limiter, MP3
sıkıştırma). Beacon'a güvenmeden önce mekânda şu test yapılır:

1. `pa-kiti/` dosyalarını sıkıştırmasız taşıyın (WAV'ı MP3/AAC'ye çevirmek
   ya da Bluetooth'la çalmak ultrasonik bandı ÖLDÜRÜR — kablolu/USB çalın).
2. Ses masasından sırayla çalın: `ton_01000` (duyulur referans; seviye ayarı
   için), sonra 16k → 20k tonları ve `supurme_15k_21k.wav`.
3. Seyirci konumunda bir telefona spektrum çözümleyici kurun (Android:
   Spectroid; iOS: SpectrumView) ve hangi tonların göründüğünü not edin.
   **Geçer sayılmak için 18,5–19,9 kHz bandının tamamı, seyirci mesafesinde
   gürültü tabanının belirgin üstünde görünmeli.**
4. `ornek_beacon.wav`'ı çalarken aynı telefonla SES KAYDI alın (kayıt
   uygulaması; mümkünse 48 kHz ayarı) ve dosyayı bilgisayarda çözün:
   `go run ./tools/beacon -decode kayit.wav`. Yük `cue_index=0, geri
   sayım=3000 ms` olarak dönerse zincir uçtan uca çalışıyor demektir.
5. Kayıt çözülmüyorsa: masadaki EQ/limiter'ın yüksek bandını açtırın,
   "hiss filtresi/denoiser" kapattırın; olmuyorsa o mekânda beacon yedeği
   YOK varsayılır ve programa güvenilir (telefonlar kueyi katılımda almış
   olur, gösteri gömülü programla akar).

## Etkinlik günü akışı (beacon devredeyken)

1. Moderatör GO'ya basar (fireAt = şimdi + N sn).
2. Panel/operatör, aynı ana denk gelecek beacon dosyasını PA'ya verir:
   dosya, `fireAt − countdown` anında BAŞLAMALIDIR. Pilotta bu elle yapılır
   (dosyayı GO ile birlikte başlat, countdown = konsoldaki gecikme);
   otomatik ses masası tetiklemesi ileriki yineleme.
3. WS'li telefonlar kueyi telden alır (hassas); WS'siz telefonların
   mikrofonu chirp'i yakalar, geri sayımı ekler, aynı koreografiye ±300 ms
   içinde katılır.

## Durum ve kalanlar

- [x] Sinyal sözleşmesi + Go kodlayıcı/çözücü (`packages/beacon`; gürültü,
  zayıflama, 44,1 kHz kayıt, çoklu patlama, CRC testli).
- [x] Araç: beacon WAV, PA test kiti, kayıt çözümü (`tools/beacon`).
- [ ] Dart dinleyicisi (mikrofon + aynı Goertzel/korelasyon; Cue Arbiter'a
  `CueSource.ultrasonic` adayı) — cihazda mikrofon izni ve gerçek PA ile
  test ister; sözleşme sabitlendiği için port mekaniktir.
- [ ] Gerçek PA'da saha ölçümü (yukarıdaki prosedür) — kullanıcıyla.

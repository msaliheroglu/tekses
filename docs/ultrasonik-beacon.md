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
  zayıflama, 44,1 kHz kayıt, çoklu patlama, CRC testli; yankı için geç
  pencere profili, kilit hatası için ±20 ms; kayıplı kodeğin sildiği tek tük
  sembol için CRC kılavuzlu chase düzeltmesi ≤2 bit).
- [x] Araç: beacon WAV, PA test kiti, kayıt çözümü + `-analyze` teşhisi
  (`tools/beacon`).
- [x] **Hava testi (2026-09-21, KULLANICI DOĞRULADI):** PC hoparlöründen
  çalınan beacon, telefonla (m4a→AAC!) kaydedilip çözüldü — 2 zayıf bit
  chase ile düzeltilerek. Seans boyunca yakalanan gerçek dünya dersleri:
  stereo ortalaması 19 kHz'te faz iptali yapabilir (kanal seçimi eklendi),
  yansıma chirp kilidini tam sembol kaydırabilir, AAC zamansal maskelemesi
  '1-koşusu→0' sembollerini siler.
- [x] **Dart çözücüsü testli (2026-09-23):** `flutter test` ile 18 test
  geçiyor (gidiş-dönüş 48k/44,1k, negatif SNR, AAC maskesi + chase,
  yinelemeler, sessizlik, kapı beacon'dan 2 sn önce kurulduğunda yakalama,
  parça boyutundan bağımsızlık). Yerel doğrulama yöntemi CLAUDE.md'de.
  - **Çapraz doğrulama tekniği (port sapmasını bulmanın yolu):** Dart'ın
    ürettiği kaydı WAV'a yazıp `go run ./tools/beacon -decode` ile çözmek.
    İki gerçekleme aynı örneklerde ayrışıyorsa hata DSP'de değil, akış
    mantığındadır — bir kez tam böyle yakalandı: akış penceresinin sonu
    kapının kurulduğu ana göre hesaplanıyordu; kapıyı chirp değil sürekli
    mekân gürültüsü kurduğunda (sahanın TİPİK durumu) chirp bulunuyor ama
    yükü pencereye sığmıyor ve bölge "arandı" sayılıp beacon büsbütün
    kaçırılıyordu. Pencere sonu artık pencere BAŞINA göre.
  - **Çözücü AYRI ISOLATE'te koşar (zorunlu):** bir arama penceresi ~80 milyon
    çarpma demek. Ölçüm (masaüstü AOT, sürekli gürültülü akış = kapının hep
    kurulu olduğu en kötü durum): tek bir 20 ms'lik parçanın işlenmesi 185 ms,
    toplam yük gerçek zamanın ~%30'u. Ana isolate'te koşarsa bu donma tam
    beacon yakalandığı anda — ateşleme anı hesaplanırken — oluşur.
    `ultrasonic_worker.dart`: ana isolate yalnızca ham PCM'i MonoClock
    damgasıyla aktarır; `heardAtMono` damgadan geriye sayılarak çözücü
    tarafında hesaplanır. Telefon CPU'su masaüstünden 3-5 kat yavaş olduğundan
    en kötü durumda yetişmeyebilir; gerçek mekânda 18,5 kHz üstü gürültü
    neredeyse yok olduğu için kapı nadiren kurulur (tipik yük ~sıfır).
  - **Denendi, İŞE YARAMADI (tekrar denemeyin):** korelasyonda pencere
    enerjisini kayan toplamla taşımak. İç döngüdeki çarpmayı yarıya indirir
    ama ölçümde toplam yük değişmedi (%29,6 → %30,6; en kötü parça yalnızca
    185 → 162 ms) — arama bellek-bağımlı, baskın olan aynı iki diziyi okumak,
    çarpma sayısı değil. Gerçek kazanç ancak arama alanını daraltmaktan ya da
    taban bandına indirip desimasyon/FFT'den gelir; ikincisi skorları
    kaydıracağı için Go referansıyla BİRLİKTE değiştirilmeli.
- [~] Dart dinleyicisi YAZILDI (2026-09-21): `apps/participant/lib/core/`
  altında `ultrasonic.dart` (akış-tabanlı çözücü, Go portu + kapı/tampon
  mantığı) ve `ultrasonic_listener.dart` (record ile 48 kHz PCM16 akışı,
  MonoClock çıpası; OS yankı/gürültü bastırma ve autogain KAPALI). Gösteri
  ekranında "beacon dinle" anahtarı; algı `CueSource.ultrasonic` olarak
  Cue Arbiter'a girer (runId `beacon:<cue>:<ateşleme-saniyesi>`; kurulu WS
  koşusunun ateşlemesi ±2 sn içindeyse beacon yok sayılır; ultrasonik
  kaynakta ofset=0 — ateşleme anı zaten yereldir). **Cihaz doğrulaması
  bekliyor:** flutter test + APK derleme (RECORD_AUDIO) + hoparlörden
  beacon çalma.
- [ ] Gerçek PA'da saha ölçümü (yukarıdaki prosedür) — kullanıcıyla.

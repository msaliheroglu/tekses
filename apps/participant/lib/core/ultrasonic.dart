/// Ultrasonik beacon kodek — packages/beacon (Go) sözleşmesinin Dart portu.
///
/// Referans gerçekleme ve testlerin sahibi Go paketidir; buradaki sabitler
/// ve algoritmalar onunla BİREBİR aynı tutulmalıdır (sinyal v1):
/// 18,5–20 kHz; 120 ms chirp önek + 10 ms boşluk + 34 bit FSK (sembol
/// 20 ms; bit0 = 18,6 kHz, bit1 = 19,4 kHz). Yük: version(4) + cue_index(8)
/// + seq(4) + geri sayım(10; 100 ms birim) + CRC-8. Geri sayım CHIRP
/// BAŞLANGICINA göredir. Sağlamlıklar da porttur: 17,5 kHz yüksek-geçiren,
/// geç pencere profili (yankı), ±20 ms'e kadar kilit ofsetleri ve CRC
/// kılavuzlu chase düzeltmesi (≤2 zayıf bit — AAC/yankı tek tük sembolü
/// silebilir).
///
/// Bu dosya SAFTIR (mikrofon/izin/UI bilmez) ve `flutter test` ile sınanır;
/// mikrofon akışını ultrasonic_listener.dart bağlar.
library;

import 'dart:math' as math;
import 'dart:typed_data';

// --- sinyal sabitleri (Go: packages/beacon/beacon.go) ---

const int beaconVersion = 1;
const double chirpLowHz = 18500;
const double chirpHighHz = 19900;
const double bit0Hz = 18600;
const double bit1Hz = 19400;
const double chirpSec = 0.120;
const double gapSec = 0.010;
const double symbolSec = 0.020;
const int payloadBits = 34;
const int countdownResMs = 100;
const double _amplitude = 0.8;

/// Beacon'ın taşıdığı bilgi.
class BeaconPayload {
  const BeaconPayload({
    required this.cueIndex,
    required this.seq,
    required this.countdownMs,
  });

  /// 0 = otomatik program; i>0 = manifestteki sequences[i-1].
  final int cueIndex;
  final int seq;

  /// Chirp başlangıcından ateşlemeye kalan süre (100 ms çözünürlük).
  final int countdownMs;

  bool get isValid =>
      cueIndex >= 0 &&
      cueIndex <= 255 &&
      seq >= 0 &&
      seq <= 15 &&
      countdownMs >= 0 &&
      countdownMs <= 1023 * countdownResMs;

  @override
  bool operator ==(Object other) =>
      other is BeaconPayload &&
      other.cueIndex == cueIndex &&
      other.seq == seq &&
      other.countdownMs == countdownMs;

  @override
  int get hashCode => Object.hash(cueIndex, seq, countdownMs);

  @override
  String toString() =>
      'BeaconPayload(cue=$cueIndex seq=$seq countdown=${countdownMs}ms)';
}

/// Akışta bulunan tek bir beacon.
class BeaconDetection {
  const BeaconDetection({
    required this.payload,
    required this.chirpStartSample,
    required this.score,
    required this.correctedBits,
  });

  final BeaconPayload payload;

  /// Chirp'in AKIŞ genelindeki mutlak örnek konumu (feed edilen ilk örnek 0).
  /// Ateşleme anı = bu örneğin duyulduğu yerel an + payload.countdownMs.
  final int chirpStartSample;
  final double score;
  final int correctedBits; // 0 = temiz; 1-2 = chase düzeltmesi

  @override
  String toString() =>
      'BeaconDetection($payload @$chirpStartSample score=${score.toStringAsFixed(2)} fix=$correctedBits)';
}

// --- bit paketleme + CRC (Go ile birebir) ---

int _crc8(List<int> data) {
  var crc = 0;
  for (final d in data) {
    crc ^= d;
    for (var i = 0; i < 8; i++) {
      crc = (crc & 0x80) != 0 ? ((crc << 1) ^ 0x07) & 0xFF : (crc << 1) & 0xFF;
    }
  }
  return crc;
}

List<int> _packBits(List<int> bits) {
  final out = List<int>.filled((bits.length + 7) ~/ 8, 0);
  for (var i = 0; i < bits.length; i++) {
    if (bits[i] != 0) out[i ~/ 8] |= 1 << (7 - i % 8);
  }
  return out;
}

List<int> _payloadToBits(BeaconPayload p) {
  final b = <int>[];
  void push(int v, int n) {
    for (var i = n - 1; i >= 0; i--) {
      b.add((v >> i) & 1);
    }
  }

  push(beaconVersion, 4);
  push(p.cueIndex, 8);
  push(p.seq, 4);
  push(p.countdownMs ~/ countdownResMs, 10);
  push(_crc8(_packBits(b)), 8);
  return b;
}

BeaconPayload? _parseBits(List<int> bits) {
  if (bits.length != payloadBits) return null;
  int take(int off, int n) {
    var v = 0;
    for (var i = 0; i < n; i++) {
      v = (v << 1) | bits[off + i];
    }
    return v;
  }

  if (take(26, 8) != _crc8(_packBits(bits.sublist(0, 26)))) return null;
  if (take(0, 4) != beaconVersion) return null;
  final p = BeaconPayload(
    cueIndex: take(4, 8),
    seq: take(12, 4),
    countdownMs: take(16, 10) * countdownResMs,
  );
  return p.isValid ? p : null;
}

double _envelope(int i, int n) {
  var ramp = n ~/ 10;
  final r = 2 * n ~/ 100;
  if (r > 0 && r < ramp) ramp = r;
  if (ramp == 0) return 1;
  if (i < ramp) return i / ramp;
  if (i >= n - ramp) return (n - 1 - i) / ramp;
  return 1;
}

/// encodeBeacon, yükü tek bir patlamaya çevirir (test ve olası telefon
/// yayını için). Örnekler [-1, 1] aralığındadır.
Float64List encodeBeacon(BeaconPayload p, int sampleRate) {
  assert(p.isValid && sampleRate >= 40000);
  final sr = sampleRate.toDouble();
  final chirpN = (chirpSec * sr).toInt();
  final gapN = (gapSec * sr).toInt();
  final symN = (symbolSec * sr).toInt();
  final out = Float64List(chirpN + gapN + payloadBits * symN);

  var phase = 0.0;
  var w = 0;
  for (var i = 0; i < chirpN; i++) {
    final t = i / chirpN;
    final freq = chirpLowHz + (chirpHighHz - chirpLowHz) * t;
    phase += 2 * math.pi * freq / sr;
    out[w++] = _amplitude * _envelope(i, chirpN) * math.sin(phase);
  }
  w += gapN;
  for (final bit in _payloadToBits(p)) {
    final freq = bit != 0 ? bit1Hz : bit0Hz;
    for (var i = 0; i < symN; i++) {
      phase += 2 * math.pi * freq / sr;
      out[w++] = _amplitude * _envelope(i, symN) * math.sin(phase);
    }
  }
  return out;
}

// --- çözücü (Go: packages/beacon/decode.go) ---

const double _chirpThreshold = 0.35;
const List<double> _retryOffsetsMs = [0, -2, 2, -4, 4, -6, 6, -10, 10, -20, 20];
// Pencere profilleri: normal (orta %70) ve geç (%45-95, yankı kuyruğunu atlar).
const List<(int, int)> _windowProfiles = [(15, 85), (45, 95)];

double _goertzel(Float64List win, int lo, int hi, double sr, double freq) {
  final w = 2 * math.pi * freq / sr;
  final coeff = 2 * math.cos(w);
  var s1 = 0.0, s2 = 0.0;
  for (var i = lo; i < hi; i++) {
    final s0 = win[i] + coeff * s1 - s2;
    s2 = s1;
    s1 = s0;
  }
  return s1 * s1 + s2 * s2 - coeff * s1 * s2;
}

/// Akış çözücüsü: mikrofon örnekleri feed edilir, bulunan beacon'lar döner.
///
/// CPU bütçesi: pahalı chirp korelasyonu yalnızca YÜKSEK BANT KAPISI
/// tetiklenince koşar (17,5 kHz üstü RMS, kayan gürültü tabanının katına
/// çıkınca) — sahne sessizken maliyet, akışı süzen tek biquad'dır.
class UltrasonicDecoder {
  UltrasonicDecoder(this.sampleRate)
      : assert(sampleRate >= 40000),
        _sr = sampleRate.toDouble() {
    _chirpN = (chirpSec * _sr).toInt();
    _gapN = (gapSec * _sr).toInt();
    _symN = (symbolSec * _sr).toInt();
    _burstN = _chirpN + _gapN + payloadBits * _symN;
    _searchSpan = (_searchSpanSec * _sr).toInt();

    _tmpl = Float64List(_chirpN);
    var phase = 0.0;
    for (var i = 0; i < _chirpN; i++) {
      final t = i / _chirpN;
      final freq = chirpLowHz + (chirpHighHz - chirpLowHz) * t;
      phase += 2 * math.pi * freq / _sr;
      _tmpl[i] = _envelope(i, _chirpN) * math.sin(phase);
    }
    for (final v in _tmpl) {
      _tmplEnergy += v * v;
    }

    // RBJ biquad yüksek-geçiren (fc 17,5 kHz, Q 0,707) — akış hâlinde.
    const fc = 17500.0;
    final w0 = 2 * math.pi * fc / _sr;
    final cosw = math.cos(w0), sinw = math.sin(w0);
    final alpha = sinw / (2 * 0.7071);
    final a0 = 1 + alpha;
    _b0 = (1 + cosw) / 2 / a0;
    _b1 = -(1 + cosw) / a0;
    _b2 = (1 + cosw) / 2 / a0;
    _a1 = -2 * cosw / a0;
    _a2 = (1 - alpha) / a0;
  }

  final int sampleRate;
  final double _sr;
  late final int _chirpN, _gapN, _symN, _burstN, _searchSpan;
  late final Float64List _tmpl;
  double _tmplEnergy = 0;

  late final double _b0, _b1, _b2, _a1, _a2;
  double _x1 = 0, _x2 = 0, _y1 = 0, _y2 = 0;

  /// Süzülmüş örneklerin kayan tamponu ve tamponun akıştaki mutlak başlangıcı.
  final List<double> _buf = <double>[];
  int _bufStartAbs = 0;

  /// Kapı durumu: gürültü tabanı (üstel ortalama) ve arama ilerlemesi.
  double _noiseFloor = 1e-6;
  int _searchedUntilAbs = 0; // bu mutlak konuma dek arandı (çift raporu önler)
  int _loudUntilAbs = 0; // bant içi ses görülen son parçanın sonu

  static const double _gateFactor = 6.0; // taban × katsayı → tetik
  // Mutlak alt eşik: bant dışı sesin süzgeç kaçağı (ör. 1 kHz müzik) kapıyı
  // sürekli kurup boşuna korelasyon koşturmasın.
  static const double _gateAbsMin = 0.002;
  static const int _keepSec = 3;
  // Bir pencerede taranan bölgenin boyu. Pencere bunun ÜSTÜNE bir tam patlama
  // taşır (bölgede başlayan chirp'in yükü de pencerede olmalı), yani gecikme
  // ve iş yükü bu sayıyla ölçeklenir.
  static const double _searchSpanSec = 0.4;

  /// Yeni örnekleri işler; tamamlanan çözümleri döndürür (çoğunlukla boş).
  List<BeaconDetection> feed(Float64List samples) {
    // Süz ve tampona ekle; kapı için parça RMS'i topla.
    var chunkEnergy = 0.0;
    for (final x in samples) {
      final y = _b0 * x + _b1 * _x1 + _b2 * _x2 - _a1 * _y1 - _a2 * _y2;
      _x2 = _x1;
      _x1 = x;
      _y2 = _y1;
      _y1 = y;
      _buf.add(y);
      chunkEnergy += y * y;
    }
    if (samples.isNotEmpty) {
      final rms = math.sqrt(chunkEnergy / samples.length);
      final endAbs = _bufStartAbs + _buf.length;
      if (rms > _gateAbsMin && rms > _noiseFloor * _gateFactor) {
        _loudUntilAbs = endAbs;
      } else {
        // Taban yalnızca sessiz parçalarla güncellenir (sinyal tabanı şişirmesin).
        _noiseFloor = 0.95 * _noiseFloor + 0.05 * math.max(rms, 1e-7);
        // Bekleyen tüm pencerelerin kurulmuş olacağı kadar sessizlik geçtiyse
        // aramayı ileri sar: uzun sessizlikten sonra gelen ses, tamponun
        // tamamını boşuna taratmasın. Son bir chirp boyu yeniden aranabilir
        // kalır — chirp bu parçanın sonunda başlamış ve parça RMS'ini henüz
        // kapı eşiğine taşımamış olabilir.
        if (endAbs > _loudUntilAbs + _searchSpan + _burstN) {
          _searchedUntilAbs = math.max(_searchedUntilAbs, endAbs - _chirpN);
        }
      }
    }

    final out = <BeaconDetection>[];
    // Bant içi ses görülmüş ama aranmamış bölge kaldıkça pencere pencere
    // ilerle (tek bir uzun feed çağrısı da birden çok pencere işleyebilir).
    while (_searchedUntilAbs < _loudUntilAbs) {
      // Pencere, taranmamış İLK noktadan başlar: kapının geç kurulması
      // (sürekli gürültüde her parça yeniden tetikler) aradaki bölgeyi
      // atlatamaz; taranan bölge de yinelenmez.
      final windowStartAbs = math.max(_bufStartAbs, _searchedUntilAbs);
      // Pencere sonu, pencere BAŞINA göredir: taranacak bölge + bir tam
      // patlama. Kapının kurulduğu ana göre hesaplanırsa (eski hata), kapıyı
      // chirp değil sürekli gürültü kurduğunda chirp bulunur ama yükü
      // pencereye sığmaz — ve bölge "arandı" sayılıp beacon kaçırılır.
      final needEndAbs = windowStartAbs + _searchSpan + _burstN;
      if (_bufStartAbs + _buf.length < needEndAbs) break; // daha örnek gerek

      final lo = windowStartAbs - _bufStartAbs;
      final win = Float64List.fromList(
          _buf.sublist(lo, needEndAbs - _bufStartAbs));
      final dets = _decodeWindow(win);
      var lastBurstEndAbs = 0;
      for (final d in dets) {
        out.add(BeaconDetection(
          payload: d.payload,
          chirpStartSample: windowStartAbs + d.chirpStartSample,
          score: d.score,
          correctedBits: d.correctedBits,
        ));
        lastBurstEndAbs = windowStartAbs + d.chirpStartSample + _burstN;
      }
      // "Arandı" işareti yalnızca YÜKÜ TAM değerlendirilebilmiş bölgeye
      // konur: pencerenin son _burstN'lik kuyruğunda başlayan bir chirp'in
      // yükü pencereye sığmaz — o bölge bir SONRAKİ pencerede yeniden
      // aranmalı. Çözülmüş bir patlamanın ötesine atlamak ise aynı patlamayı
      // ikinci kez raporlamayı önler (chirp payı ile).
      _searchedUntilAbs = math.max<int>(windowStartAbs + _searchSpan,
          lastBurstEndAbs - _chirpN ~/ 2);
    }

    // Tamponu son _keepSec saniyeye kırp.
    final keep = _keepSec * sampleRate;
    if (_buf.length > keep) {
      final drop = _buf.length - keep;
      _buf.removeRange(0, drop);
      _bufStartAbs += drop;
    }
    return out;
  }

  /// Pencere içinde Go DecodeAll eşleniği (örnekler ZATEN süzülmüş).
  List<BeaconDetection> _decodeWindow(Float64List win) {
    final out = <BeaconDetection>[];
    var from = 0;
    while (out.length < 4) {
      final (start, score) = _findChirp(win, from);
      if (start < 0) break;
      final res = _demodulateWithRetry(win, start + _chirpN + _gapN);
      if (res != null) {
        out.add(BeaconDetection(
          payload: res.$1,
          chirpStartSample: start,
          score: score,
          correctedBits: res.$2,
        ));
        from = start + _burstN;
      } else {
        from = start + _chirpN ~/ 2;
      }
    }
    return out;
  }

  (int, double) _findChirp(Float64List win, int from) {
    const step = 4;
    var bestAt = -1;
    var bestScore = 0.0;
    final limit = win.length - _chirpN;
    for (var at = from; at <= limit; at += step) {
      final s = _corrScore(win, at);
      if (s > bestScore) {
        bestScore = s;
        bestAt = at;
      }
    }
    if (bestAt < 0 || bestScore < _chirpThreshold) return (-1, 0);
    final lo = math.max(from, bestAt - 8);
    final hi = math.min(limit, bestAt + 8);
    for (var at = lo; at <= hi; at++) {
      final s = _corrScore(win, at);
      if (s > bestScore) {
        bestScore = s;
        bestAt = at;
      }
    }
    return (bestAt, bestScore);
  }

  double _corrScore(Float64List win, int at) {
    var dot = 0.0, energy = 0.0;
    for (var i = 0; i < _chirpN; i++) {
      final sv = win[at + i];
      dot += sv * _tmpl[i];
      energy += sv * sv;
    }
    if (energy == 0) return 0;
    return dot * dot / (energy * _tmplEnergy);
  }

  /// (payload, düzeltilenBit) ya da null. Go demodulateWithRetry eşleniği:
  /// önce sıfır düzeltmeli tüm profil+ofsetler, sonra chase (≤2 zayıf bit).
  (BeaconPayload, int)? _demodulateWithRetry(Float64List win, int at) {
    for (final prof in _windowProfiles) {
      for (final offMs in _retryOffsetsMs) {
        final off = (offMs * _sr / 1000).toInt();
        if (at + off < 0) continue;
        final r = _demodBits(win, at + off, prof);
        if (r == null) continue;
        final p = _parseBits(r.$1);
        if (p != null) return (p, 0);
      }
    }
    for (final prof in _windowProfiles) {
      for (final offMs in const [0.0, -2.0, 2.0, -4.0, 4.0]) {
        final off = (offMs * _sr / 1000).toInt();
        if (at + off < 0) continue;
        final r = _demodBits(win, at + off, prof);
        if (r == null) continue;
        final bits = r.$1;
        final weakest = _weakestIndices(r.$2, 4);
        for (var i = 0; i < weakest.length; i++) {
          final p = _tryFlips(bits, [weakest[i]]);
          if (p != null) return (p, 1);
        }
        for (var i = 0; i < weakest.length; i++) {
          for (var j = i + 1; j < weakest.length; j++) {
            final p = _tryFlips(bits, [weakest[i], weakest[j]]);
            if (p != null) return (p, 2);
          }
        }
      }
    }
    return null;
  }

  (List<int>, List<double>)? _demodBits(Float64List win, int at, (int, int) prof) {
    final bits = List<int>.filled(payloadBits, 0);
    final margins = List<double>.filled(payloadBits, 0);
    for (var s = 0; s < payloadBits; s++) {
      final lo = at + s * _symN + _symN * prof.$1 ~/ 100;
      final hi = at + s * _symN + _symN * prof.$2 ~/ 100;
      if (lo < 0 || hi > win.length) return null;
      final e0 = _goertzel(win, lo, hi, _sr, bit0Hz);
      final e1 = _goertzel(win, lo, hi, _sr, bit1Hz);
      if (e1 > e0) bits[s] = 1;
      margins[s] = e0 + e1 > 0 ? (e1 - e0).abs() / (e1 + e0) : 0;
    }
    return (bits, margins);
  }

  static List<int> _weakestIndices(List<double> margins, int n) {
    final idx = List<int>.generate(margins.length, (i) => i);
    idx.sort((a, b) => margins[a].compareTo(margins[b]));
    return idx.take(n).toList();
  }

  static BeaconPayload? _tryFlips(List<int> bits, List<int> flips) {
    final c = List<int>.of(bits);
    for (final f in flips) {
      c[f] ^= 1;
    }
    return _parseBits(c);
  }
}

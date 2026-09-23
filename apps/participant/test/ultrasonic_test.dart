import 'dart:math' as math;
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:tekses_participant/core/ultrasonic.dart';

// Go paketindeki (packages/beacon) test beklentilerinin Dart aynası:
// iki gerçekleme aynı sinyal sözleşmesini çözmek zorundadır.

Float64List embed(Float64List burst, int before, int after) {
  final out = Float64List(before + burst.length + after);
  out.setRange(before, before + burst.length, burst);
  return out;
}

/// Akışı gerçekçi biçimde 20 ms'lik parçalara bölerek besler.
List<BeaconDetection> feedChunked(UltrasonicDecoder dec, Float64List rec) {
  final out = <BeaconDetection>[];
  const chunk = 960;
  for (var i = 0; i < rec.length; i += chunk) {
    final end = math.min(i + chunk, rec.length);
    out.addAll(dec.feed(Float64List.sublistView(rec, i, end)));
  }
  return out;
}

void main() {
  const payload = BeaconPayload(cueIndex: 3, seq: 7, countdownMs: 2500);

  test('gidiş-dönüş: 48k ve 44,1k akışta çözüm + konum', () {
    for (final sr in [48000, 44100]) {
      final burst = encodeBeacon(payload, sr);
      final rec = embed(burst, sr ~/ 3, sr);
      final dets = feedChunked(UltrasonicDecoder(sr), rec);
      expect(dets, hasLength(1), reason: 'sr=$sr');
      expect(dets.first.payload, payload);
      expect(dets.first.correctedBits, 0);
      // Chirp konumu ±3 ms içinde bulunmalı (geri sayım buna göre eklenir).
      expect((dets.first.chirpStartSample - sr ~/ 3).abs(),
          lessThan((sr * 0.003).toInt()), reason: 'sr=$sr');
    }
  });

  test('gürültü + zayıflama: negatif SNR kayıtta çözüm', () {
    const sr = 48000;
    final burst = encodeBeacon(payload, sr);
    final rng = math.Random(42);
    final rec = embed(burst, sr ~/ 2, sr ~/ 2);
    for (var i = 0; i < rec.length; i++) {
      rec[i] = rec[i] / 8 + 0.2 * (2 * rng.nextDouble() - 1);
    }
    final dets = feedChunked(UltrasonicDecoder(sr), rec);
    expect(dets, hasLength(1));
    expect(dets.first.payload, payload);
  });

  // Regresyon: beacon, kapıyı kuran sesten ÇOK sonra gelebilir. Akış
  // penceresinin sonu kapının kurulduğu ana göre hesaplanırsa (eski hata)
  // chirp bulunur ama yükü pencereye sığmaz, bölge "arandı" sayılır ve
  // beacon büsbütün kaçırılır. Mekân gürültüsü sürekli olduğu için bu
  // saha koşullarının TİPİK durumudur.
  test('sürekli gürültü: kapı beacon\'dan 2 sn önce kurulsa da yakalanır', () {
    const sr = 48000;
    final burst = encodeBeacon(payload, sr);
    final rng = math.Random(7);
    final rec = embed(burst, 2 * sr, sr ~/ 2);
    for (var i = 0; i < rec.length; i++) {
      rec[i] = rec[i] / 8 + 0.2 * (2 * rng.nextDouble() - 1);
    }
    final dets = feedChunked(UltrasonicDecoder(sr), rec);
    expect(dets, hasLength(1));
    expect(dets.first.payload, payload);
    expect((dets.first.chirpStartSample - 2 * sr).abs(),
        lessThan((sr * 0.003).toInt()));
  });

  // Regresyon: sonuç besleme parça boyutundan bağımsız olmalı (aynı kayıt
  // tek çağrıda da, 20 ms'lik parçalarla da aynı beacon'ı vermeli).
  test('parça boyutu sonucu değiştirmez', () {
    const sr = 48000;
    final rec = embed(encodeBeacon(payload, sr), sr ~/ 3, sr);
    for (final chunk in [rec.length, 4800, 960, 128]) {
      final dec = UltrasonicDecoder(sr);
      final dets = <BeaconDetection>[];
      for (var i = 0; i < rec.length; i += chunk) {
        final end = math.min(i + chunk, rec.length);
        dets.addAll(dec.feed(Float64List.sublistView(rec, i, end)));
      }
      expect(dets, hasLength(1), reason: 'parça=$chunk');
      expect(dets.first.payload, payload, reason: 'parça=$chunk');
      expect((dets.first.chirpStartSample - sr ~/ 3).abs(),
          lessThan((sr * 0.003).toInt()),
          reason: 'parça=$chunk');
    }
  });

  test('AAC maskesi: silinen 2 sembol chase ile kurtarılır', () {
    const sr = 48000;
    const p = BeaconPayload(cueIndex: 0, seq: 1, countdownMs: 3000);
    final burst = encodeBeacon(p, sr);
    final chirpN = (chirpSec * sr).toInt();
    final gapN = (gapSec * sr).toInt();
    final symN = (symbolSec * sr).toInt();
    // Saha vakası: CRC'deki '111→0' geçiş bitleri (29 ve 33) silinir,
    // yerlerine önceki taşıyıcının (bit1) zayıf kuyruğu kalır.
    for (final bitIdx in [29, 33]) {
      final lo = chirpN + gapN + bitIdx * symN;
      var phase = 0.0;
      for (var i = 0; i < symN; i++) {
        phase += 2 * math.pi * bit1Hz / sr;
        burst[lo + i] = 0.1 * math.sin(phase);
      }
    }
    final dets = feedChunked(UltrasonicDecoder(sr), embed(burst, sr ~/ 4, sr));
    expect(dets, hasLength(1));
    expect(dets.first.payload, p);
    expect(dets.first.correctedBits, 2);
  });

  test('yinelemeler: 1 sn arayla iki patlama ayrı ayrı çözülür', () {
    const sr = 48000;
    const p1 = BeaconPayload(cueIndex: 2, seq: 1, countdownMs: 3000);
    const p2 = BeaconPayload(cueIndex: 2, seq: 2, countdownMs: 2000);
    final b1 = encodeBeacon(p1, sr);
    final b2 = encodeBeacon(p2, sr);
    final rec = Float64List(sr ~/ 4 + sr + b2.length + sr);
    rec.setRange(sr ~/ 4, sr ~/ 4 + b1.length, b1);
    rec.setRange(sr ~/ 4 + sr, sr ~/ 4 + sr + b2.length, b2);
    final dets = feedChunked(UltrasonicDecoder(sr), rec);
    expect(dets.map((d) => d.payload).toList(), [p1, p2]);
    // İki chirp konumu arası tam yineleme aralığı (~1 sn) olmalı.
    expect((dets[1].chirpStartSample - dets[0].chirpStartSample - sr).abs(),
        lessThan((sr * 0.003).toInt()));
  });

  test('sessizlik ve bant dışı ses beacon üretmez', () {
    const sr = 48000;
    final dec = UltrasonicDecoder(sr);
    expect(feedChunked(dec, Float64List(2 * sr)), isEmpty);
    final low = Float64List(2 * sr);
    for (var i = 0; i < low.length; i++) {
      low[i] = 0.5 * math.sin(2 * math.pi * 1000 * i / sr);
    }
    expect(feedChunked(dec, low), isEmpty);
  });
}

import 'dart:async';
import 'dart:math' as math;
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:tekses_participant/core/ultrasonic.dart';
import 'package:tekses_participant/core/ultrasonic_worker.dart';

// Çözücü ayrı bir isolate'te koştuğu için sınırı geçen üç şey sınanır:
// PCM16 → örnek dönüşümü, algı alanlarının eksiksiz dönmesi ve zaman
// çıpasının (ana isolate'te alınan monoton damga) doğru geriye sayılması.

/// Örnekleri mikrofonun verdiği biçime çevirir: 16-bit küçük-uçlu mono PCM.
Uint8List toPcm16(Float64List samples) {
  final out = Uint8List(samples.length * 2);
  final view = ByteData.sublistView(out);
  for (var i = 0; i < samples.length; i++) {
    final q = (samples[i] * 32767).round().clamp(-32768, 32767);
    view.setInt16(2 * i, q, Endian.little);
  }
  return out;
}

void main() {
  test('isolate çözücüsü: PCM16 akışından algı ve monoton çıpa', () async {
    const sr = 48000;
    const payload = BeaconPayload(cueIndex: 5, seq: 2, countdownMs: 1800);
    final burst = encodeBeacon(payload, sr);
    // Patlama 0,5 sn'de başlar; kayıt 2 sn.
    final rec = Float64List(2 * sr);
    rec.setRange(sr ~/ 2, sr ~/ 2 + burst.length, burst);

    final got = Completer<(BeaconDetection, int)>();
    final worker = UltrasonicWorker(
      sampleRate: sr,
      onDetection: (det, heardAtMonoMs) {
        if (!got.isCompleted) got.complete((det, heardAtMonoMs));
      },
    );
    await worker.start();

    // 20 ms'lik parçalar, her biri gerçek akış gibi damgalı: parça i'nin son
    // örneği t = (i+1)·20 ms anında gelmiş sayılır.
    const chunk = 960;
    var monoMs = 10000;
    for (var i = 0; i < rec.length; i += chunk) {
      final end = math.min(i + chunk, rec.length);
      monoMs += (chunk * 1000) ~/ sr;
      worker.feed(toPcm16(Float64List.sublistView(rec, i, end)), monoMs);
    }

    final (det, heardAtMonoMs) = await got.future.timeout(
      const Duration(seconds: 20),
      onTimeout: () => throw StateError('isolate çözücüsünden algı gelmedi'),
    );
    await worker.stop();

    expect(det.payload, payload);
    expect(det.correctedBits, 0);
    // Chirp, akışın 0,5 sn'sinde: damgalar 10000 ms'den başladığı için
    // duyulma anı ~10500 ms olmalı (±10 ms parça granülerliği).
    expect((heardAtMonoMs - 10500).abs(), lessThan(10),
        reason: 'heardAtMono=$heardAtMonoMs');
    // Parçaların hepsi gönderildiğinden ateşleme anı da yerel olarak kurulur.
    expect(heardAtMonoMs + det.payload.countdownMs, closeTo(12300, 10));
  });

  test('sessiz akış algı üretmez ve durdurma temiz kapanır', () async {
    const sr = 48000;
    var count = 0;
    final worker = UltrasonicWorker(
      sampleRate: sr,
      onDetection: (_, __) => count++,
    );
    await worker.start();
    const chunk = 960;
    for (var i = 0; i < sr; i += chunk) {
      worker.feed(toPcm16(Float64List(chunk)), 1000 + i ~/ 48);
    }
    // Isolate'in kuyruğu boşaltmasına fırsat ver, sonra kapat.
    await Future<void>.delayed(const Duration(milliseconds: 300));
    await worker.stop();
    expect(count, 0);
    expect(worker.running, isFalse);
  });
}

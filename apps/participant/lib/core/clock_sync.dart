/// NTP benzeri saat ofseti kestirimi.
///
/// Algoritma Go tarafındaki packages/clocksync ve tarayıcıdaki join.html ile
/// birebir aynıdır ve öyle kalmalıdır: 8–12 örnek, en iyi RTT'nin +10 ms
/// BANDINDAKİ örnekler, ofset medyanı. Bant seçimi bilinçli ("en iyi yarı"
/// değil): 20k fırtına yük testi (2026-09-26) çift tepeli dağılımı yakaladı —
/// izdihamda örneklerin çoğu saptırılmışken "yarı" onları da medyana taşıyor,
/// en iyi RTT ise tertemiz görünüyordu.
/// Ofset tanımı: sunucuSaati ≈ istemciMonoton + ofset (ms).
library;

/// En iyi örneğin RTT'sine eklenen seçim bandı (packages/clocksync.RTTBandMs).
const int rttBandMs = 10;

class ClockSample {
  const ClockSample({
    required this.t0,
    required this.t1,
    required this.t2,
    required this.t3,
  });

  final int t0; // istek gönderildi (istemci monoton)
  final int t1; // istek alındı (sunucu)
  final int t2; // yanıt gönderildi (sunucu)
  final int t3; // yanıt alındı (istemci monoton)

  int get rtt => (t3 - t0) - (t2 - t1);
  int get offset => ((t1 - t0) + (t2 - t3)) ~/ 2;
}

class ClockEstimate {
  const ClockEstimate({
    required this.offsetMs,
    required this.bestRttMs,
    required this.usedSamples,
  });

  final int offsetMs;
  final int bestRttMs;
  final int usedSamples;
}

class ClockSyncEstimator {
  final List<ClockSample> _samples = [];

  int get length => _samples.length;

  void reset() => _samples.clear();

  /// Negatif RTT'li (bozuk zaman damgalı) örnekler atılır.
  void add(ClockSample s) {
    if (s.rtt < 0) return;
    _samples.add(s);
  }

  /// En iyi RTT'nin +[rttBandMs] bandındaki örneklerin ofset medyanı; hiç
  /// örnek yoksa null. Bandın dışındaki örnek belirgin daha uzun yol yürüdü
  /// demektir ve o yolun asimetrisi ofseti saptırır — medyana giremez.
  ClockEstimate? estimate() {
    if (_samples.isEmpty) return null;

    final byRtt = List<ClockSample>.of(_samples)
      ..sort((a, b) => a.rtt.compareTo(b.rtt));
    final cut = byRtt.first.rtt + rttBandMs;
    var keep = 1;
    while (keep < byRtt.length && byRtt[keep].rtt <= cut) {
      keep++;
    }
    final chosen = byRtt.sublist(0, keep);

    final offsets = chosen.map((s) => s.offset).toList()..sort();
    final median = keep.isOdd
        ? offsets[keep ~/ 2]
        : (offsets[keep ~/ 2 - 1] + offsets[keep ~/ 2]) ~/ 2;

    return ClockEstimate(
      offsetMs: median,
      bestRttMs: chosen.first.rtt,
      usedSamples: keep,
    );
  }
}

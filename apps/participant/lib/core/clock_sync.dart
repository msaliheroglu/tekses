/// NTP benzeri saat ofseti kestirimi.
///
/// Algoritma Go tarafındaki packages/clocksync ile birebir aynıdır ve öyle
/// kalmalıdır: 8–12 örnek, RTT'ye göre en iyi yarı, ofset medyanı.
/// Ofset tanımı: sunucuSaati ≈ istemciMonoton + ofset (ms).
library;

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

  /// Düşük RTT'li yarının ofset medyanı; hiç örnek yoksa null.
  ClockEstimate? estimate() {
    if (_samples.isEmpty) return null;

    final byRtt = List<ClockSample>.of(_samples)
      ..sort((a, b) => a.rtt.compareTo(b.rtt));
    // En iyi yarı (en az 1): yüksek RTT asimetrik gecikme taşır, ofseti saptırır.
    final keep = (byRtt.length + 1) ~/ 2;
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

import 'package:flutter_test/flutter_test.dart';
import 'package:tekses_participant/core/clock_sync.dart';

/// Go referansındaki estimator_test'in aynası (packages/clocksync):
/// iki gerçekleme aynı örneklerde aynı ofseti bulmalı.
ClockSample makeSample(int t0, int trueOffset, int up, int down, int proc) {
  final t1 = t0 + up + trueOffset;
  final t2 = t1 + proc;
  final t3 = t2 - trueOffset + down;
  return ClockSample(t0: t0, t1: t1, t2: t2, t3: t3);
}

void main() {
  test('simetrik gecikmede tek örnek gerçek ofseti bulur', () {
    final s = makeSample(1000, 500, 20, 20, 3);
    expect(s.offset, 500);
    expect(s.rtt, 40);
  });

  test('yüksek RTT bandın dışında kalır', () {
    final e = ClockSyncEstimator();
    for (var i = 0; i < 5; i++) {
      e.add(makeSample(1000 + i * 100, -1234, 15, 15, 2)); // temiz
    }
    for (var i = 0; i < 5; i++) {
      e.add(makeSample(2000 + i * 100, -1234, 400, 20, 2)); // saptırılmış
    }
    final est = e.estimate()!;
    expect(est.offsetMs, -1234);
    expect(est.usedSamples, 5);
    expect(est.bestRttMs, 30);
  });

  test('çift tepeli izdiham: azınlıktaki temiz örnekler kazanır', () {
    // 20k fırtına yük testinin yakaladığı durum: "en iyi yarı" çöp
    // örnekleri de medyana taşıyordu; RTT bandı taşımaz.
    final e = ClockSyncEstimator();
    e.add(makeSample(1000, 777, 2, 2, 1));
    e.add(makeSample(1100, 777, 2, 2, 1));
    for (var i = 0; i < 8; i++) {
      e.add(makeSample(2000 + i * 100, 777, 5, 55, 1)); // ofseti −25 ms kaydırır
    }
    final est = e.estimate()!;
    expect(est.offsetMs, 777);
    expect(est.usedSamples, 2);
  });

  test('negatif RTT atılır; boş kestirim null', () {
    final e = ClockSyncEstimator();
    e.add(const ClockSample(t0: 100, t1: 50, t2: 51, t3: 90));
    expect(e.length, 0);
    expect(e.estimate(), isNull);
  });
}

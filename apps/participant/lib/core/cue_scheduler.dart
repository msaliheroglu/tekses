import 'dart:async';

import 'mono_clock.dart';

/// Zamanlanmış bir ateşlemenin iptal kolu.
class ScheduledFire {
  ScheduledFire._(this._timer);

  Timer? _timer;
  bool _cancelled = false;

  void cancel() {
    _cancelled = true;
    _timer?.cancel();
  }
}

/// Sunucu saatindeki bir anı yerel monoton saate çevirip hassas ateşler.
///
/// Timer çözünürlüğü platformda ~1–10 ms oynayabildiği için hedeften
/// [spinWindowMs] önce uyanılır ve kalan süre monoton saat üzerinde sıkı
/// döngüyle beklenir; ateşleme sapması pratikte 1 ms altına iner.
class CueScheduler {
  static const int spinWindowMs = 40;

  /// [fireAtServerMs] anını `yerel = sunucu − ofset` ile çevirir ve
  /// [onFire] geri çağrısını o anda çalıştırır. An geçmişteyse hemen ateşler
  /// (geç katılan telefon koreografiye kaldığı yerden girer); geri çağrıya
  /// gecikme bilgisi verilir.
  static ScheduledFire schedule({
    required int fireAtServerMs,
    required int offsetMs,
    required void Function(int lateByMs) onFire,
  }) {
    final fireLocalMs = fireAtServerMs - offsetMs;
    final fire = ScheduledFire._(null);

    void fireNow() {
      if (fire._cancelled) return;
      final late = MonoClock.nowMs - fireLocalMs;
      onFire(late > 0 ? late : 0);
    }

    final delayMs = fireLocalMs - MonoClock.nowMs;
    if (delayMs <= 0) {
      fireNow();
      return fire;
    }

    final wakeMs = delayMs > spinWindowMs ? delayMs - spinWindowMs : 0;
    fire._timer = Timer(Duration(milliseconds: wakeMs), () {
      final targetUs = fireLocalMs * 1000;
      // Sıkı bekleme en çok spinWindowMs sürer; UI ipliğini hissedilir
      // biçimde tıkamaz ve kue anında zaten başka iş yoktur.
      while (MonoClock.nowUs < targetUs) {
        if (fire._cancelled) return;
      }
      fireNow();
    });
    return fire;
  }
}

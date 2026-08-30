/// Uygulama ömrü boyunca tek monoton saat.
///
/// Karar: cihazda daima monoton saat kullanılır, duvar saati ASLA. Duvar
/// saati NTP düzeltmesi ya da elle ayarla sıçrayabilir; koreografinin
/// zamanlaması bundan etkilenmemelidir. Sunucu saatiyle ilişki yalnızca
/// clock_sync ofsetiyle kurulur: sunucuSaati ≈ MonoClock.nowMs + ofset.
class MonoClock {
  MonoClock._();

  static final Stopwatch _watch = Stopwatch()..start();

  static int get nowMs => _watch.elapsedMilliseconds;
  static int get nowUs => _watch.elapsedMicroseconds;
}

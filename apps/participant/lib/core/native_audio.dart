import 'dart:async';

import 'package:flutter/services.dart';

import 'mono_clock.dart';

/// Native zamanlanmış ses.
///
/// Karar: Dart seviyesi zamanlama ses için fazla titrek — çalma anını
/// platformun kendi monoton saati planlar: Android'de Handler.postAtTime
/// (SystemClock.uptimeMillis ekseni), iOS'ta AVAudioPlayer.play(atTime:)
/// (ses donanımı saati). Kanal sözleşmesi (tekses/audio):
///
///   uptimeNow()                → platform monoton saati (ms)
///   prepare(id, path)          → dosyayı çalmaya hazırla
///   playAt(id, uptimeMs)       → platform saatinde tam o anda başlat
///   stopAll()                  → hepsini durdur (STOP/BLACKOUT)
///
/// Dart, kendi monoton saatini (MonoClock) platformunkine bir kez eşler ve
/// hedefi platform eksenine çevirerek yollar. Kanal yoksa (platform dosyası
/// kopyalanmamış ya da masaüstü) servis sessizce devre dışı kalır: ışık
/// koreografisi ses olmadan sürer.
class NativeAudio {
  static const _channel = MethodChannel('tekses/audio');

  bool _available = false;

  /// MonoClock → platform monoton saati ofseti (ms):
  /// platformSaati ≈ MonoClock.nowMs + _uptimeOffsetMs.
  int _uptimeOffsetMs = 0;

  bool get available => _available;

  /// Kanalı yoklar ve saat eşlemesini yapar. Birkaç örnekten en düşük
  /// gidiş-dönüşlü olanı seçilir (saat senkronundaki ilkeyle aynı).
  Future<void> init() async {
    try {
      var bestRtt = 1 << 30;
      for (var i = 0; i < 5; i++) {
        final t0 = MonoClock.nowMs;
        final uptime = await _channel.invokeMethod<int>('uptimeNow');
        final t1 = MonoClock.nowMs;
        if (uptime == null) continue;
        final rtt = t1 - t0;
        if (rtt < bestRtt) {
          bestRtt = rtt;
          _uptimeOffsetMs = uptime - (t0 + rtt ~/ 2);
        }
      }
      _available = true;
    } on MissingPluginException {
      _available = false; // platform kanalı kurulmamış; ses devre dışı
    } catch (_) {
      _available = false;
    }
  }

  /// Varlığı çalmaya hazırlar (kod çözme burada, çalma anında değil).
  Future<void> prepare(String id, String filePath) async {
    if (!_available) return;
    try {
      await _channel.invokeMethod('prepare', {'id': id, 'path': filePath});
    } catch (_) {
      // hazırlanamayan varlık çalınmaz; koreografi durmaz
    }
  }

  /// Varlığı, MonoClock ekseninde verilen anda başlatacak şekilde planlar.
  Future<void> playAtMono(String id, int fireLocalMs) async {
    if (!_available) return;
    try {
      await _channel.invokeMethod('playAt', {
        'id': id,
        'uptimeMs': fireLocalMs + _uptimeOffsetMs,
      });
    } catch (_) {
      // planlanamayan çalma yutulur
    }
  }

  Future<void> stopAll() async {
    if (!_available) return;
    try {
      await _channel.invokeMethod('stopAll');
    } catch (_) {}
  }
}

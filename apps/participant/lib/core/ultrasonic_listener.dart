import 'dart:async';
import 'dart:typed_data';

import 'package:record/record.dart';

import 'mono_clock.dart';
import 'ultrasonic.dart';
import 'ultrasonic_worker.dart';

/// Mikrofonu dinleyip beacon patlamalarını yakalar ve her algıyı MONOTON
/// yerel saate çevirerek bildirir.
///
/// Zaman çıpası: her PCM parçası işlendiğinde parçanın SON örneğinin geldiği
/// an MonoClock'tan damgalanır. Chirp'in duyulduğu an geriye doğru örnek
/// sayısından hesaplanır:
///   heardAtMono = sonParçaAnı − (sonMutlakÖrnek − chirpÖrneği) · 1000 / fs
/// Ateşleme anı = heardAtMono + payload.countdownMs (geri sayım chirp'in
/// BAŞLANGICINA görédir — sinyal sözleşmesi, docs/ultrasonik-beacon.md).
/// Ses tamponlama gecikmesi bu çıpayı ~duyma yönünde geciktirir; beacon
/// zaten ±300 ms'lik erişilebilirlik yedeği olduğu için kabul edilir.
///
/// Çözme işi ayrı bir isolate'te koşar (ultrasonic_worker.dart): korelasyon
/// pencereleri ana isolate'i yüz milisaniyelerce dondurabilir. Buradaki iş
/// yalnızca baytları damgalayıp aktarmaktır.
class UltrasonicListener {
  UltrasonicListener({required this.onDetection, this.onStatus});

  /// Algı + chirp'in duyulduğu monoton an (ms).
  final void Function(BeaconDetection det, int heardAtMonoMs) onDetection;
  final void Function(String note)? onStatus;

  static const int _sampleRate = 48000;

  final AudioRecorder _recorder = AudioRecorder();
  StreamSubscription<Uint8List>? _sub;
  UltrasonicWorker? _worker;

  bool get running => _sub != null;

  /// Mikrofon iznini ister ve dinlemeyi başlatır. İzin yoksa false döner.
  Future<bool> start() async {
    if (running) return true;
    if (!await _recorder.hasPermission()) {
      onStatus?.call('mikrofon izni verilmedi');
      return false;
    }
    // Çözücü ve mikrofon başlatması cihazda çeşitli biçimlerde düşebilir
    // (isolate doğmaz, kayıt aygıtı meşgul, biçim desteklenmez). Hiçbiri
    // gösteriyi düşürmemeli: beacon yalnızca YEDEK — durum satırında söyle,
    // false dön, koreografi programdan/WS'ten akmaya devam etsin.
    try {
      final worker = UltrasonicWorker(
        sampleRate: _sampleRate,
        onDetection: onDetection,
      );
      await worker.start();
      _worker = worker;
      final stream = await _recorder.startStream(const RecordConfig(
        encoder: AudioEncoder.pcm16bits,
        sampleRate: _sampleRate,
        numChannels: 1,
        // Ultrasonik bant için işletim sisteminin ses "iyileştirmeleri"
        // zehirdir: yankı/gürültü bastırma 18-20 kHz'i tıraşlayabilir.
        echoCancel: false,
        noiseSuppress: false,
        autoGain: false,
      ));
      _sub = stream.listen(_onChunk, onError: (Object err) {
        onStatus?.call('mikrofon akışı koptu: $err');
        stop();
      });
    } catch (err) {
      onStatus?.call('beacon başlatılamadı: $err');
      await stop();
      return false;
    }
    onStatus?.call('beacon dinleniyor');
    return true;
  }

  Future<void> stop() async {
    final sub = _sub;
    _sub = null;
    await _worker?.stop();
    _worker = null;
    await sub?.cancel();
    try {
      await _recorder.stop();
    } catch (_) {
      // kayıt zaten durmuş olabilir; sessizce geç
    }
  }

  Future<void> dispose() async {
    await stop();
    await _recorder.dispose();
  }

  /// Parçayı DAMGALAYIP çözücü isolate'ine devreder. Damga burada alınır:
  /// MonoClock tek olmalı ve parçanın son örneğinin geldiği ana en yakın an
  /// budur (kuyrukta beklemek damgayı bozmaz, parçayla birlikte taşınır).
  void _onChunk(Uint8List bytes) {
    _worker?.feed(bytes, MonoClock.nowMs);
  }
}

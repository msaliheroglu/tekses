import 'dart:async';
import 'dart:typed_data';

import 'package:record/record.dart';

import 'mono_clock.dart';
import 'ultrasonic.dart';

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
class UltrasonicListener {
  UltrasonicListener({required this.onDetection, this.onStatus});

  /// Algı + chirp'in duyulduğu monoton an (ms).
  final void Function(BeaconDetection det, int heardAtMonoMs) onDetection;
  final void Function(String note)? onStatus;

  static const int _sampleRate = 48000;

  final AudioRecorder _recorder = AudioRecorder();
  StreamSubscription<Uint8List>? _sub;
  UltrasonicDecoder? _decoder;

  /// Akışta işlenen toplam örnek sayısı ve son parçanın monoton damgası.
  int _endAbs = 0;
  int _endMonoMs = 0;

  /// Parça sınırında bölünen 16-bit örneğin ilk baytı.
  int? _pendingByte;

  bool get running => _sub != null;

  /// Mikrofon iznini ister ve dinlemeyi başlatır. İzin yoksa false döner.
  Future<bool> start() async {
    if (running) return true;
    if (!await _recorder.hasPermission()) {
      onStatus?.call('mikrofon izni verilmedi');
      return false;
    }
    _decoder = UltrasonicDecoder(_sampleRate);
    _endAbs = 0;
    _pendingByte = null;
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
    onStatus?.call('beacon dinleniyor');
    return true;
  }

  Future<void> stop() async {
    final sub = _sub;
    _sub = null;
    _decoder = null;
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

  void _onChunk(Uint8List bytes) {
    final decoder = _decoder;
    if (decoder == null) return;

    // 16-bit LE PCM → [-1, 1]. Parça tek sayıda bayt taşıyabilir; artan
    // bayt bir sonraki parçanın başına eklenir.
    var data = bytes;
    final pending = _pendingByte;
    if (pending != null) {
      data = Uint8List(bytes.length + 1)
        ..[0] = pending
        ..setRange(1, bytes.length + 1, bytes);
      _pendingByte = null;
    }
    final n = data.length ~/ 2;
    if (data.length.isOdd) _pendingByte = data[data.length - 1];
    if (n == 0) return;

    final view = ByteData.sublistView(data, 0, n * 2);
    final samples = Float64List(n);
    for (var i = 0; i < n; i++) {
      samples[i] = view.getInt16(2 * i, Endian.little) / 32768.0;
    }

    _endAbs += n;
    _endMonoMs = MonoClock.nowMs;

    for (final det in decoder.feed(samples)) {
      final heardAtMonoMs = _endMonoMs -
          ((_endAbs - det.chirpStartSample) * 1000 / _sampleRate).round();
      onDetection(det, heardAtMonoMs);
    }
  }
}

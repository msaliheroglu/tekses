import 'dart:async';
import 'dart:isolate';
import 'dart:typed_data';

import 'ultrasonic.dart';

/// Beacon çözücüsünü AYRI BİR ISOLATE'te koşturur; ana isolate'e yalnızca
/// hazır algılar döner.
///
/// **Neden zorunlu:** chirp korelasyonu bir arama penceresinde ~80 milyon
/// çarpma yapar. Ölçüm (masaüstü AOT, sürekli gürültülü akış — kapının hep
/// kurulu olduğu en kötü durum): tek bir 20 ms'lik parçanın işlenmesi
/// 185 ms sürüyor, toplam yük gerçek zamanın ~%30'u. Ana isolate'te
/// koşarsa bu donma tam beacon yakalandığı anda — yani kue ateşleme anı
/// hesaplanırken — arayüzü ve zamanlayıcıları yüz milisaniyelerce durdurur;
/// koreografi hedefi ≤30 ms olan bir uygulamada kabul edilemez.
///
/// **Zaman çıpası isolate sınırını geçer:** monoton damga ana isolate'te
/// alınır (MonoClock tek olmalı) ve parçayla birlikte gönderilir; çözücü
/// tarafı `heardAtMono`'yu o damgadan geriye sayarak hesaplar.
class UltrasonicWorker {
  UltrasonicWorker({required this.sampleRate, required this.onDetection});

  final int sampleRate;

  /// Algı + chirp'in duyulduğu monoton an (ms). Ana isolate'te çağrılır.
  final void Function(BeaconDetection det, int heardAtMonoMs) onDetection;

  Isolate? _iso;
  SendPort? _toIso;
  ReceivePort? _fromIso;
  Completer<void>? _ready;

  bool get running => _fromIso != null;

  /// Çözücü isolate'ini doğurur ve EL SIKIŞMAYI BEKLER: döndüğünde `feed`
  /// doğrudan iletir. Beklenmezse ilk parçalar (chirp'in kendisi olabilir)
  /// gidecek port bulunamadığı için düşer.
  Future<void> start() async {
    if (running) return;
    final from = ReceivePort();
    _fromIso = from;
    final ready = Completer<void>();
    _ready = ready;
    from.listen(_onMessage);
    _iso = await Isolate.spawn(
        _decoderEntry, <Object>[from.sendPort, sampleRate],
        debugName: 'beacon-decoder', onError: from.sendPort);
    await ready.future.timeout(const Duration(seconds: 5), onTimeout: () {
      throw StateError('beacon çözücü isolate\'i başlamadı');
    });
  }

  /// Ham 16-bit küçük-uçlu mono PCM parçası + parçanın SON örneğinin
  /// monoton damgası. Dönüşüm ve tamponlama çözücü isolate'inde yapılır
  /// (ana isolate'te örnek başına iş kalmaz).
  void feed(Uint8List pcm16le, int monoMs) {
    _toIso?.send(<Object>[pcm16le, monoMs]);
  }

  Future<void> stop() async {
    final from = _fromIso;
    _fromIso = null;
    _ready = null;
    _toIso?.send(null); // çözücü kendi portunu kapatır
    _toIso = null;
    from?.close();
    // Kapanışı beklemeye değmez; port kapandıktan sonra isolate boşta kalır.
    _iso?.kill(priority: Isolate.beforeNextEvent);
    _iso = null;
  }

  void _onMessage(Object? msg) {
    if (msg is SendPort) {
      _toIso = msg;
      if (_ready?.isCompleted == false) _ready!.complete();
      return;
    }
    // Isolate.spawn(onError:) hatayı [mesaj, yığın izi] olarak yollar.
    if (msg is List && msg.length == 2 && msg[0] is String?) {
      if (_ready?.isCompleted == false) {
        _ready!.completeError(StateError('beacon çözücüsü çöktü: ${msg[0]}'));
      }
      return;
    }
    if (msg is List && msg.length == 7) {
      onDetection(
        BeaconDetection(
          payload: BeaconPayload(
            cueIndex: msg[0] as int,
            seq: msg[1] as int,
            countdownMs: msg[2] as int,
          ),
          chirpStartSample: msg[3] as int,
          score: msg[4] as double,
          correctedBits: msg[5] as int,
        ),
        msg[6] as int,
      );
    }
  }
}

/// Çözücü isolate'i: PCM baytlarını örneklere çevirir, akış çözücüsünü
/// besler, algıları ana isolate'e yollar.
void _decoderEntry(List<Object> init) {
  final toMain = init[0] as SendPort;
  final sampleRate = init[1] as int;
  final decoder = UltrasonicDecoder(sampleRate);
  final from = ReceivePort();
  toMain.send(from.sendPort);

  var endAbs = 0;
  int? pendingByte; // parça sınırında bölünen 16-bit örneğin ilk baytı

  from.listen((Object? msg) {
    if (msg is! List) {
      from.close(); // null = kapat
      return;
    }
    var data = msg[0] as Uint8List;
    final monoMs = msg[1] as int;

    final carry = pendingByte;
    if (carry != null) {
      data = Uint8List(data.length + 1)
        ..[0] = carry
        ..setRange(1, data.length + 1, data);
      pendingByte = null;
    }
    final n = data.length ~/ 2;
    if (data.length.isOdd) pendingByte = data[data.length - 1];
    if (n == 0) return;

    final view = ByteData.sublistView(data, 0, n * 2);
    final samples = Float64List(n);
    for (var i = 0; i < n; i++) {
      samples[i] = view.getInt16(2 * i, Endian.little) / 32768.0;
    }
    endAbs += n;

    for (final det in decoder.feed(samples)) {
      // heardAtMono = sonParçaAnı − (sonMutlakÖrnek − chirpÖrneği)·1000/fs
      final heardAtMonoMs = monoMs -
          ((endAbs - det.chirpStartSample) * 1000 / sampleRate).round();
      toMain.send(<Object>[
        det.payload.cueIndex,
        det.payload.seq,
        det.payload.countdownMs,
        det.chirpStartSample,
        det.score,
        det.correctedBits,
        heardAtMonoMs,
      ]);
    }
  });
}

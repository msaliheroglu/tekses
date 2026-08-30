import 'dart:async';
import 'dart:math';

import 'package:web_socket_channel/web_socket_channel.dart';

import 'clock_sync.dart';
import 'messages.dart';
import 'mono_clock.dart';

/// Gateway WebSocket istemcisi.
///
/// Sorumlulukları: bağlanma ve jitter'lı üstel geri çekilmeyle yeniden
/// bağlanma, hello/welcome el sıkışması, periyodik saat senkronu turları,
/// kue ve müdahale mesajlarını dinleyicilere iletme.
///
/// Karar gereği telefon gösteri İÇİN yeniden bağlanmaya muhtaç değildir:
/// kue alındıktan sonra koreografi yerel saatten akar; bu istemci koparsa
/// yalnızca sonraki kueler ve müdahaleler kaçar, o da arka planda sessizce
/// yeniden bağlanarak telafi edilir.
class RealtimeClient {
  RealtimeClient({
    required this.uri,
    required this.onEstimate,
    required this.onCue,
    required this.onIntervention,
    required this.onStatus,
    this.joinCode = '',
    this.samplesPerRound = 10,
    this.samplePause = const Duration(milliseconds: 40),
    this.resyncEvery = const Duration(seconds: 60),
  });

  final Uri uri;
  final String joinCode;
  final int samplesPerRound;
  final Duration samplePause;
  final Duration resyncEvery;

  final void Function(ClockEstimate estimate) onEstimate;
  final void Function(CueStartMsg cue) onCue;
  final void Function(InterventionMsg intervention) onIntervention;
  final void Function(String status) onStatus;

  final _estimator = ClockSyncEstimator();
  final _random = Random();

  WebSocketChannel? _channel;
  StreamSubscription<dynamic>? _sub;
  bool _closed = false;
  int _reconnectAttempt = 0;

  int _seq = 0;
  int? _awaitingSeq;
  int _roundSent = 0;
  Timer? _pingTimeout;
  Timer? _nextPing;
  Timer? _resync;
  Timer? _reconnect;

  void connect() {
    if (_closed) return;
    onStatus('bağlanılıyor: $uri');
    final channel = WebSocketChannel.connect(uri);
    _channel = channel;
    _sub = channel.stream.listen(
      _onData,
      onError: (Object _) => _handleDisconnect(),
      onDone: _handleDisconnect,
      cancelOnError: true,
    );
    _send(typeHello, {
      'protocol_version': protocolVersion,
      'join_code': joinCode,
      'client_kind': 'flutter',
    });
  }

  void close() {
    _closed = true;
    _cancelTimers();
    _sub?.cancel();
    _channel?.sink.close();
  }

  void _cancelTimers() {
    _pingTimeout?.cancel();
    _nextPing?.cancel();
    _resync?.cancel();
    _reconnect?.cancel();
  }

  void _handleDisconnect() {
    if (_closed) return;
    _cancelTimers();
    _awaitingSeq = null;
    _reconnectAttempt = min(_reconnectAttempt + 1, 6);
    // Jitter'lı üstel geri çekilme: yeniden bağlanma fırtınası, binlerce
    // telefonun aynı saniyede yüklenmesi demektir; rastgelelik bunu yayar.
    final baseMs = 1000 * pow(2, _reconnectAttempt - 1).toInt();
    final delayMs = min(30000, baseMs) * (0.5 + _random.nextDouble());
    onStatus('bağlantı koptu; ${(delayMs / 1000).toStringAsFixed(1)} sn sonra yeniden denenecek');
    _reconnect = Timer(Duration(milliseconds: delayMs.round()), connect);
  }

  void _send(String type, Map<String, dynamic> data) {
    _channel?.sink.add(encodeEnvelope(type, data));
  }

  void _onData(dynamic raw) {
    final env = decodeEnvelope(raw);
    if (env == null) return;

    switch (env.type) {
      case typeWelcome:
        _reconnectAttempt = 0;
        final welcome = WelcomeMsg.fromJson(env.data);
        onStatus('odaya bağlanıldı: ${welcome.roomId}');
        _startSyncRound();
      case typeClockSyncResponse:
        _onClockResponse(ClockSyncResponseMsg.fromJson(env.data));
      case typeCueStart:
        final cue = CueStartMsg.fromJson(env.data);
        if (cue != null) onCue(cue);
      case typeIntervention:
        final intervention = InterventionMsg.fromJson(env.data);
        if (intervention != null) onIntervention(intervention);
    }
  }

  // --- saat senkronu turu ---

  void _startSyncRound() {
    _estimator.reset();
    _roundSent = 0;
    _sendNextPing();
  }

  void _sendNextPing() {
    if (_closed) return;
    _seq++;
    _roundSent++;
    _awaitingSeq = _seq;
    _send(typeClockSyncRequest, {
      'seq': _seq,
      'client_mono_ms': MonoClock.nowMs,
    });
    // Yanıt kaybolursa tur takılmasın; örnek atlanır.
    _pingTimeout = Timer(const Duration(seconds: 2), () {
      if (_awaitingSeq != null) {
        _awaitingSeq = null;
        _advanceRound();
      }
    });
  }

  void _onClockResponse(ClockSyncResponseMsg resp) {
    if (resp.seq != _awaitingSeq) return;
    final t3 = MonoClock.nowMs;
    _pingTimeout?.cancel();
    _awaitingSeq = null;
    _estimator.add(ClockSample(
      t0: resp.clientMonoMs,
      t1: resp.serverRecvMs,
      t2: resp.serverSendMs,
      t3: t3,
    ));
    _advanceRound();
  }

  void _advanceRound() {
    if (_roundSent < samplesPerRound) {
      _nextPing = Timer(samplePause, _sendNextPing);
      return;
    }
    final estimate = _estimator.estimate();
    if (estimate != null) {
      onEstimate(estimate);
      onStatus(
          'saat senkronu: ofset ${estimate.offsetMs} ms, en iyi RTT ${estimate.bestRttMs} ms');
    } else {
      onStatus('saat senkronu başarısız; yeniden denenecek');
    }
    // Ofset her 1–2 dakikada bir yenilenir; son iyi değer kullanımda kalır.
    _resync = Timer(resyncEvery, _startSyncRound);
  }
}

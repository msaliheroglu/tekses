import 'dart:async';

import 'package:flutter/material.dart';
import 'package:wakelock_plus/wakelock_plus.dart';

import '../core/clock_sync.dart';
import '../core/cue_arbiter.dart';
import '../core/cue_scheduler.dart';
import '../core/messages.dart';
import '../core/mono_clock.dart';
import '../core/realtime_client.dart';
import '../core/torch_service.dart';

/// Gösteri ekranı: bağlanır, saatini eşitler, kue bekler; ateşleme anında
/// ekranı boyar ve feneri yakar. Kue alındıktan sonra koreografi tümüyle
/// yerel monoton saatten akar — ağ kopsa bile efekt bozulmaz.
class ShowScreen extends StatefulWidget {
  const ShowScreen({super.key, required this.serverUri});

  final Uri serverUri;

  @override
  State<ShowScreen> createState() => _ShowScreenState();
}

class _ShowScreenState extends State<ShowScreen> {
  late final RealtimeClient _client;
  late final CueArbiter _arbiter;
  final _torch = TorchService();

  ClockEstimate? _estimate;
  String _status = 'başlatılıyor';

  CueStartMsg? _activeCue;
  int _fireLocalMs = 0;
  ScheduledFire? _pendingFire;
  Timer? _effectTicker;
  bool _held = false;

  Color _background = Colors.black;
  bool _torchTarget = false;

  @override
  void initState() {
    super.initState();
    WakelockPlus.enable();
    _torch.init();
    _arbiter = CueArbiter(onAccepted: _onCueAccepted);
    _client = RealtimeClient(
      uri: widget.serverUri,
      onEstimate: (est) => setState(() => _estimate = est),
      onCue: (cue) => _arbiter.offer(cue, CueSource.websocket),
      onIntervention: _onIntervention,
      onStatus: (s) => setState(() => _status = s),
    );
    _client.connect();
  }

  @override
  void dispose() {
    _stopEffect(toBlack: false);
    _client.close();
    _torch.off();
    WakelockPlus.disable();
    super.dispose();
  }

  // --- kue akışı ---

  void _onCueAccepted(CueStartMsg cue, CueSource source) {
    final estimate = _estimate;
    if (estimate == null) {
      setState(() => _status = 'kue geldi ama saat senkronu yok; atlandı');
      return;
    }
    _pendingFire?.cancel();
    _stopEffect(toBlack: true);
    _held = false;
    setState(() {
      _activeCue = cue;
      _status = 'kue alındı (${cue.cueId}); ateşleme bekleniyor';
    });
    _fireLocalMs = cue.fireAtServerMs - estimate.offsetMs;
    _pendingFire = CueScheduler.schedule(
      fireAtServerMs: cue.fireAtServerMs,
      offsetMs: estimate.offsetMs,
      onFire: (lateByMs) => _startEffect(cue, lateByMs),
    );
  }

  void _startEffect(CueStartMsg cue, int lateByMs) {
    if (!mounted) return;
    setState(() => _status = lateByMs > 0
        ? 'koreografi sürüyor (geç katılım: +$lateByMs ms)'
        : 'koreografi sürüyor');
    _applyFrame(cue);
    // Kare güncelleme: 60 Hz'e yakın periyotla faz yeniden hesaplanır.
    // Faz daima (şimdi − ateşlemeAnı) üzerinden bulunduğu için tüm
    // telefonlar tık sayısından bağımsız aynı anda aynı yarımdadır.
    _effectTicker = Timer.periodic(const Duration(milliseconds: 16), (_) {
      if (_held) return;
      _applyFrame(cue);
    });
  }

  void _applyFrame(CueStartMsg cue) {
    final elapsed = MonoClock.nowMs - _fireLocalMs;
    if (elapsed >= cue.payload.durationMs) {
      _stopEffect(toBlack: true);
      setState(() => _status = 'koreografi bitti; yeni kue bekleniyor');
      return;
    }

    final bool lit;
    if (cue.payload.flashHz == 0) {
      lit = true;
    } else {
      final halfPeriodMs = 500 ~/ cue.payload.flashHz;
      lit = (elapsed ~/ halfPeriodMs).isEven;
    }

    final color = lit ? _parseColor(cue.payload.color) : Colors.black;
    final torchWanted = lit && cue.payload.torch;
    if (color != _background || torchWanted != _torchTarget) {
      setState(() {
        _background = color;
        _torchTarget = torchWanted;
      });
      _torch.set(torchWanted);
    }
  }

  void _stopEffect({required bool toBlack}) {
    _effectTicker?.cancel();
    _effectTicker = null;
    _torchTarget = false;
    _torch.off();
    if (toBlack && mounted) {
      setState(() => _background = Colors.black);
    }
  }

  void _onIntervention(InterventionMsg intervention) {
    switch (intervention.kind) {
      case 'STOP':
      case 'BLACKOUT':
        _pendingFire?.cancel();
        _stopEffect(toBlack: true);
        setState(() {
          _activeCue = null;
          _status = intervention.kind == 'BLACKOUT' ? 'KARARTMA' : 'durduruldu';
        });
      case 'HOLD':
        // Ekran son karede kalır; güvenlik gereği fener söndürülür.
        _held = true;
        _torch.off();
        setState(() => _status = 'beklemede (HOLD)');
      case 'SKIP':
        // Faz 0'da sekans listesi yok; Faz 1'de timeline_engine ele alacak.
        setState(() => _status = 'SKIP alındı (Faz 0: işlem yok)');
    }
  }

  static Color _parseColor(String hex) {
    if (hex.length == 7 && hex.startsWith('#')) {
      final value = int.tryParse(hex.substring(1), radix: 16);
      if (value != null) return Color(0xFF000000 | value);
    }
    return Colors.white;
  }

  // --- görünüm ---

  @override
  Widget build(BuildContext context) {
    final estimate = _estimate;
    return Scaffold(
      backgroundColor: _background,
      body: SafeArea(
        child: Stack(
          children: [
            // Efekt tüm ekranı kaplar; durum yazısı köşede küçük kalır.
            Positioned(
              top: 8,
              left: 12,
              right: 12,
              child: DefaultTextStyle(
                style: TextStyle(
                  color: Colors.white.withValues(alpha: 0.55),
                  fontSize: 12,
                ),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(_status),
                    if (estimate != null)
                      Text(
                        'ofset ${estimate.offsetMs} ms · RTT ${estimate.bestRttMs} ms '
                        '· örnek ${estimate.usedSamples}'
                        '${_torch.available ? '' : ' · fener yok'}',
                      ),
                    if (_activeCue != null)
                      Text('run ${_activeCue!.runId.substring(0, 8)}'),
                  ],
                ),
              ),
            ),
            Positioned(
              bottom: 8,
              right: 12,
              child: TextButton(
                onPressed: () => Navigator.of(context).pop(),
                child: Text(
                  'ayrıl',
                  style: TextStyle(color: Colors.white.withValues(alpha: 0.4)),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

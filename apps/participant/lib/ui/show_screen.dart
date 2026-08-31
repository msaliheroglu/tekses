import 'dart:async';

import 'package:flutter/material.dart';
import 'package:wakelock_plus/wakelock_plus.dart';

import '../core/clock_sync.dart';
import '../core/cue_arbiter.dart';
import '../core/cue_scheduler.dart';
import '../core/messages.dart';
import '../core/mono_clock.dart';
import '../core/package_store.dart';
import '../core/realtime_client.dart';
import '../core/timeline_engine.dart';
import '../core/torch_service.dart';

/// Gösteri ekranı: bağlanır, saatini eşitler, kue bekler; ateşleme anında
/// koreografiyi oynatır. cue_id manifestteki bir sekansa denk geliyorsa
/// zaman çizelgesi (sözler + kue şeritleri) akar; değilse Faz 0 tarzı
/// doğrudan yük (renk/flash/fener) uygulanır. Kue alındıktan sonra her şey
/// yerel monoton saatten akar — ağ kopsa bile koreografi bozulmaz.
class ShowScreen extends StatefulWidget {
  const ShowScreen({
    super.key,
    required this.serverUri,
    this.joinInfo,
    this.joinCode = '',
  });

  final Uri serverUri;
  final JoinInfo? joinInfo;
  final String joinCode;

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
  TimelineEngine? _engine; // cue_id bir sekansa denk geldiyse dolu
  int _fireLocalMs = 0;
  ScheduledFire? _pendingFire;
  Timer? _effectTicker;
  bool _held = false;

  Color _background = Colors.black;
  bool _torchTarget = false;
  String _lyric = '';

  @override
  void initState() {
    super.initState();
    WakelockPlus.enable();
    _torch.init();
    _arbiter = CueArbiter(onAccepted: _onCueAccepted);
    _client = RealtimeClient(
      uri: widget.serverUri,
      joinCode: widget.joinCode,
      onEstimate: (est) => setState(() => _estimate = est),
      onCue: (cue) {
        // Senkron yoksa run kilitlenmez: sunucunun 250 ms arayla yolladığı
        // tekrarlar, ofset o sırada hazırlanmışsa kueyi kurtarabilsin.
        if (_estimate == null) {
          setState(() => _status = 'kue geldi ama saat senkronu yok; tekrar bekleniyor');
          return;
        }
        _arbiter.offer(cue, CueSource.websocket);
      },
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
    // cue_id manifestteki bir sekansı işaret ediyorsa zaman çizelgesi modu.
    final sequence = widget.joinInfo?.manifest?.sequenceById(cue.cueId);
    _engine = sequence == null ? null : TimelineEngine(sequence);
    setState(() {
      _activeCue = cue;
      _status = sequence == null
          ? 'kue alındı (${cue.cueId}); ateşleme bekleniyor'
          : 'sekans hazır: ${sequence.title}; ateşleme bekleniyor';
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

    final Color color;
    final bool torchWanted;
    final String lyric;

    final engine = _engine;
    if (engine != null) {
      // Zaman çizelgesi modu: kare tamamen saf motordan gelir.
      final frame = engine.frameAt(elapsed);
      if (frame.done) {
        _stopEffect(toBlack: true);
        setState(() => _status = 'sekans bitti; yeni kue bekleniyor');
        return;
      }
      color = frame.screenLit && frame.screenColor.isNotEmpty
          ? _parseColor(frame.screenColor)
          : Colors.black;
      torchWanted = frame.torchOn;
      lyric = frame.lyric;
    } else {
      // Faz 0 modu: yük doğrudan kuenin içinde.
      if (elapsed >= cue.payload.durationMs) {
        _stopEffect(toBlack: true);
        setState(() => _status = 'koreografi bitti; yeni kue bekleniyor');
        return;
      }
      final bool lit;
      if (cue.payload.flashHz == 0) {
        lit = true;
      } else {
        // floor(elapsed*hz/500): yarım periyodu (500/hz) yuvarlamadan sayar.
        // Tarayıcı istemcisi ve timeline_engine ile birebir aynı aritmetik;
        // kırpılmış tam sayı periyot (500 ~/ hz) 3 Hz'te ~4 ms/sn faz kaydırır.
        lit = ((elapsed * cue.payload.flashHz) ~/ 500).isEven;
      }
      color = lit ? _parseColor(cue.payload.color) : Colors.black;
      torchWanted = lit && cue.payload.torch;
      lyric = '';
    }

    if (color != _background || torchWanted != _torchTarget || lyric != _lyric) {
      setState(() {
        _background = color;
        _torchTarget = torchWanted;
        _lyric = lyric;
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
      setState(() {
        _background = Colors.black;
        _lyric = '';
      });
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
            // Söz satırı: ekranın ortasında, büyük ve dış çizgili — arka
            // plan hangi renkte olursa olsun okunur.
            if (_lyric.isNotEmpty)
              Center(
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 24),
                  child: Text(
                    _lyric,
                    textAlign: TextAlign.center,
                    style: const TextStyle(
                      fontSize: 40,
                      fontWeight: FontWeight.w800,
                      color: Colors.white,
                      shadows: [
                        Shadow(blurRadius: 12, color: Colors.black),
                        Shadow(blurRadius: 4, color: Colors.black),
                      ],
                    ),
                  ),
                ),
              ),
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

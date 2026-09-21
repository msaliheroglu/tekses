import 'dart:async';

import 'package:flutter/material.dart';
import 'package:wakelock_plus/wakelock_plus.dart';

import '../core/clock_sync.dart';
import '../core/cue_arbiter.dart';
import '../core/cue_scheduler.dart';
import '../core/messages.dart';
import '../core/mono_clock.dart';
import '../core/native_audio.dart';
import '../core/package_store.dart';
import '../core/realtime_client.dart';
import '../core/show_manifest.dart';
import '../core/timeline_engine.dart';
import '../core/torch_service.dart';
import '../core/ultrasonic.dart';
import '../core/ultrasonic_listener.dart';

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
    this.controlUri,
  });

  final Uri serverUri;
  final JoinInfo? joinInfo;
  final String joinCode;

  /// Kodla katılımda control-api adresi: show_activated sinyali gelince
  /// paket buradan yeniden indirilir (null = canlı yenileme yok, Faz 0).
  final Uri? controlUri;

  @override
  State<ShowScreen> createState() => _ShowScreenState();
}

class _ShowScreenState extends State<ShowScreen> {
  late final RealtimeClient _client;
  late final CueArbiter _arbiter;
  final _torch = TorchService();
  final _audio = NativeAudio();
  UltrasonicListener? _listener;

  /// Ultrasonik yedek: mikrofon dinlemesi kullanıcı eliyle açılır (pil +
  /// izin istemi gerekçesi); durum satırı ne olduğunu her an söyler.
  bool _micOn = false;
  String _beaconNote = '';

  /// Katılım bilgisi: show_activated sinyaliyle yerinde tazelenir (paket +
  /// varlıklar yeniden iner); ilk değer katılım ekranından gelir.
  JoinInfo? _joinInfo;
  bool _refreshingJoin = false;

  /// Aktif kuenin ses planı: ateşleme anına göre (atMs, hazırlanmış çalar id).
  List<({String playerId, int atMs})> _audioPlan = const [];

  ClockEstimate? _estimate;
  String _status = 'başlatılıyor';

  /// Ses teşhis satırı: kanal yoksa ya da dosya inmemişse kullanıcı
  /// SESSİZLİĞİN nedenini ekranda görür (sessizce yutulmasın).
  String _audioNote = '';

  CueStartMsg? _activeCue;
  FrameSource? _engine; // cue_id sekansa/programa denk geldiyse dolu
  int _fireLocalMs = 0;
  ScheduledFire? _pendingFire;
  Timer? _effectTicker;
  bool _held = false;

  Color _background = Colors.black;
  bool _torchTarget = false;
  String _lyric = '';
  String _nextLyric = '';

  @override
  void initState() {
    super.initState();
    WakelockPlus.enable();
    _joinInfo = widget.joinInfo;
    _torch.init();
    _audio.init().then((_) {
      if (mounted && !_audio.available) {
        setState(() =>
            _audioNote = 'ses kanalı yok — APK, native MainActivity ile derlenmeli (README)');
      }
    });
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
      onShowActivated: (_) => _refreshJoinInfo(),
      onStatus: (s) => setState(() => _status = s),
    );
    _client.connect();
  }

  /// show_activated sinyali: paket + varlıklar yeniden indirilir; süren
  /// koreografi etkilenmez (motor kendi manifest kopyasını tutar), yeni
  /// gösteri bir SONRAKİ kueyle çalınır.
  Future<void> _refreshJoinInfo() async {
    final controlUri = widget.controlUri;
    if (controlUri == null || widget.joinCode.isEmpty || _refreshingJoin) return;
    _refreshingJoin = true;
    if (mounted) setState(() => _status = 'gösteri güncellendi; paket yenileniyor…');
    try {
      final info = await PackageStore().join(controlUri, widget.joinCode);
      if (!mounted) return;
      setState(() {
        _joinInfo = info;
        _status = info.manifest == null
            ? 'oda güncellendi (aktif gösteri yok)'
            : 'gösteri hazır: ${info.manifest!.title}';
      });
    } catch (err) {
      // Yenileme başarısızsa eldeki paket geçerli kalır; sonraki sinyal ya da
      // yeniden katılım telafi eder.
      if (mounted) setState(() => _status = 'paket yenilenemedi: $err');
    } finally {
      _refreshingJoin = false;
    }
  }

  @override
  void dispose() {
    _stopEffect(toBlack: false);
    _client.close();
    _listener?.dispose();
    _torch.off();
    WakelockPlus.disable();
    super.dispose();
  }

  // --- ultrasonik yedek ---

  Future<void> _toggleMic() async {
    if (_micOn) {
      await _listener?.stop();
      setState(() {
        _micOn = false;
        _beaconNote = '';
      });
      return;
    }
    final listener = _listener ??= UltrasonicListener(
      onDetection: _onBeaconDetected,
      onStatus: (note) {
        if (mounted) setState(() => _beaconNote = 'beacon: $note');
      },
    );
    final ok = await listener.start();
    if (mounted) setState(() => _micOn = ok);
  }

  /// Beacon algısı → kue adayı. Geri sayım chirp'in duyulduğu MONOTON ana
  /// eklenir; ateşleme anı zaten yerel olduğu için saat senkronu GEREKMEZ
  /// (beacon tam da senkronsuz/WS'siz telefonlar için var).
  void _onBeaconDetected(BeaconDetection det, int heardAtMonoMs) {
    if (!mounted) return;
    final fireLocalMs = heardAtMonoMs + det.payload.countdownMs;

    // cue_index eşlemesi (sinyal sözleşmesi): 0 = otomatik program,
    // i>0 = manifestteki sequences[i-1]. Eşleşmeyen indeks yok sayılır.
    final manifest = _joinInfo?.manifest;
    final String cueId;
    if (det.payload.cueIndex == 0) {
      cueId = programCueId;
    } else if (manifest != null &&
        det.payload.cueIndex <= manifest.sequences.length) {
      cueId = manifest.sequences[det.payload.cueIndex - 1].id;
    } else {
      setState(() => _beaconNote =
          'beacon: bilinmeyen sekans #${det.payload.cueIndex}; yok sayıldı');
      return;
    }

    // WS kaynağı varken beacon yok sayılır (karar dokümanı §3): süren/kurulu
    // koşunun ateşlemesi ±2 sn içindeyse bu, aynı kuenin sesli kopyasıdır.
    if (_activeCue != null && (fireLocalMs - _fireLocalMs).abs() < 2000) {
      setState(() => _beaconNote = 'beacon: duyuldu, WS kuesi zaten kurulu');
      return;
    }

    // Yinelemeler (~1 sn arayla, geri sayım düşerek) aynı ateşleme SANİYESİNE
    // çözülür; sentetik runId bu yüzden tekrarları arbiter'da tekilleştirir.
    final runId =
        'beacon:${det.payload.cueIndex}:${(fireLocalMs + 500) ~/ 1000}';
    setState(() => _beaconNote =
        'beacon: kue #${det.payload.cueIndex} duyuldu (${det.payload.countdownMs} ms)'
        '${det.correctedBits > 0 ? ' · ${det.correctedBits} bit düzeltildi' : ''}');
    _arbiter.offer(
      CueStartMsg(
        runId: runId,
        cueId: cueId,
        // Ultrasonik kaynakta bu alan YEREL monoton ateşleme anını taşır;
        // _onCueAccepted ofset=0 ile okur (sunucu saati hiç işe karışmaz).
        fireAtServerMs: fireLocalMs,
        repeatSeq: det.payload.seq,
        payload: const CuePayloadMsg(
            color: '#FFFFFF', torch: false, flashHz: 0, durationMs: 3000),
      ),
      CueSource.ultrasonic,
    );
  }

  // --- kue akışı ---

  void _onCueAccepted(CueStartMsg cue, CueSource source) {
    // Sözleşme: ultrasonik kaynakta fireAtServerMs yerel monoton andır →
    // ofset 0; WS kaynağında sunucu anıdır → saat senkronu şarttır.
    final int offsetMs;
    if (source == CueSource.ultrasonic) {
      offsetMs = 0;
    } else {
      final estimate = _estimate;
      if (estimate == null) {
        setState(() => _status = 'kue geldi ama saat senkronu yok; atlandı');
        return;
      }
      offsetMs = estimate.offsetMs;
    }
    _pendingFire?.cancel();
    _stopEffect(toBlack: true);
    _held = false;
    // cue_id "program" ise gömülü otomatik program, bir sekansı işaret
    // ediyorsa tek sekans; ikisi de değilse Faz 0 yükü oynar.
    final manifest = _joinInfo?.manifest;
    String statusLabel;
    List<({String sequenceId, int baseMs})> played = const [];
    if (cue.cueId == programCueId && manifest != null && manifest.program.isNotEmpty) {
      _engine = ProgramEngine(manifest);
      played = [
        for (final item in manifest.program)
          (sequenceId: item.sequenceId, baseMs: item.atOffsetMs),
      ];
      statusLabel = 'otomatik program hazır (${manifest.program.length} sekans)';
    } else {
      final sequence = manifest?.sequenceById(cue.cueId);
      _engine = sequence == null ? null : TimelineEngine(sequence);
      if (sequence != null) played = [(sequenceId: sequence.id, baseMs: 0)];
      statusLabel = sequence == null
          ? 'kue alındı (${cue.cueId})'
          : 'sekans hazır: ${sequence.title}';
    }
    _prepareAudio(manifest, played);
    setState(() {
      _activeCue = cue;
      _status = '$statusLabel; ateşleme bekleniyor';
    });
    _fireLocalMs = cue.fireAtServerMs - offsetMs;
    _pendingFire = CueScheduler.schedule(
      fireAtServerMs: cue.fireAtServerMs,
      offsetMs: offsetMs,
      onFire: (lateByMs) => _startEffect(cue, lateByMs),
    );
  }

  /// Sekansların ses kuelerini hazırlar: her kue kendi çalar örneğini alır
  /// (aynı varlık iki kez çalınabilsin), kod çözme ateşleme öncesi biter.
  void _prepareAudio(ShowManifest? manifest,
      List<({String sequenceId, int baseMs})> played) {
    _audio.stopAll();
    final plan = <({String playerId, int atMs})>[];
    final paths = _joinInfo?.assetPaths ?? const {};
    var audioCues = 0, missing = 0;
    for (final entry in played) {
      final seq = manifest?.sequenceById(entry.sequenceId);
      if (seq == null) continue;
      for (final lane in seq.cueLanes) {
        if (lane.kind != 'audio') continue;
        for (final cue in lane.cues) {
          audioCues++;
          final path = paths[cue.assetId];
          if (path == null) {
            missing++; // varlık inmemiş; ışık koreografisi sürer
            continue;
          }
          final playerId = '${cue.assetId}#${plan.length}';
          _audio.prepare(playerId, path);
          plan.add((playerId: playerId, atMs: entry.baseMs + cue.atMs));
        }
      }
    }
    _audioPlan = plan;
    // Teşhis: gösteri ses istiyorsa ama çalamayacaksak nedeni ekrana yaz.
    if (audioCues == 0) {
      // gösteri sessiz; kanal uyarısı (init) varsa korunur
    } else if (!_audio.available) {
      _audioNote = 'ses kanalı yok — APK, native MainActivity ile derlenmeli (README)';
    } else if (missing > 0) {
      _audioNote = 'ses: $missing dosya inmemiş — odadan çıkıp yeniden katılın';
    } else {
      _audioNote = 'ses: ${plan.length} parça planlandı';
    }
  }

  void _startEffect(CueStartMsg cue, int lateByMs) {
    if (!mounted) return;
    // Ses planı ateşleme anında, kesinleşmiş fireLocal üzerinden platforma
    // devredilir; bu andan sonra çalma anını platformun kendi saati tutar.
    for (final entry in _audioPlan) {
      if (entry.atMs >= lateByMs) {
        _audio.playAtMono(entry.playerId, _fireLocalMs + entry.atMs);
      }
    }
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
    var nextLyric = '';

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
      nextLyric = frame.nextLyric;
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

    if (color != _background ||
        torchWanted != _torchTarget ||
        lyric != _lyric ||
        nextLyric != _nextLyric) {
      setState(() {
        _background = color;
        _torchTarget = torchWanted;
        _lyric = lyric;
        _nextLyric = nextLyric;
      });
      _torch.set(torchWanted);
    }
  }

  void _stopEffect({required bool toBlack}) {
    _effectTicker?.cancel();
    _effectTicker = null;
    _torchTarget = false;
    _torch.off();
    _audio.stopAll();
    if (toBlack && mounted) {
      setState(() {
        _background = Colors.black;
        _lyric = '';
        _nextLyric = '';
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
        // Ekran son karede kalır; güvenlik gereği fener ve ses susturulur.
        _held = true;
        _torch.off();
        _audio.stopAll();
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
            // Karaoke görünümü: aktif satır ortada büyük, sıradaki satır
            // altında soluk — arka plan hangi renkte olursa olsun okunur.
            if (_lyric.isNotEmpty || _nextLyric.isNotEmpty)
              Center(
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: 24),
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      if (_lyric.isNotEmpty)
                        Text(
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
                      if (_nextLyric.isNotEmpty) ...[
                        const SizedBox(height: 16),
                        Text(
                          _nextLyric,
                          textAlign: TextAlign.center,
                          style: TextStyle(
                            fontSize: 24,
                            fontWeight: FontWeight.w600,
                            color: Colors.white.withValues(alpha: 0.45),
                            shadows: const [Shadow(blurRadius: 8, color: Colors.black)],
                          ),
                        ),
                      ],
                    ],
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
                    if (_audioNote.isNotEmpty) Text(_audioNote),
                    if (_beaconNote.isNotEmpty) Text(_beaconNote),
                    if (_activeCue != null)
                      Text('run ${_activeCue!.runId.substring(0, 8)}'),
                  ],
                ),
              ),
            ),
            // Ultrasonik yedek anahtarı: WS'si kopabilecek telefonlarda
            // seyirci gösteriden önce açar; açıkken mikrofon PA beacon'ını
            // bekler. (Erişilebilirlik yedeği — hassasiyet kaynağı WS'dir.)
            Positioned(
              bottom: 8,
              left: 12,
              child: TextButton.icon(
                onPressed: _toggleMic,
                icon: Icon(
                  _micOn ? Icons.hearing : Icons.hearing_disabled,
                  size: 16,
                  color: Colors.white.withValues(alpha: _micOn ? 0.8 : 0.4),
                ),
                label: Text(
                  _micOn ? 'beacon açık' : 'beacon dinle',
                  style: TextStyle(
                    color: Colors.white.withValues(alpha: _micOn ? 0.8 : 0.4),
                  ),
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

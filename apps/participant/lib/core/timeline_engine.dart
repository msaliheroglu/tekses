/// Zaman çizelgesi motoru: bir sekansın herhangi bir anındaki kareyi üretir.
///
/// Tamamen SAFTIR: girdisi "sekans başından geçen süre" (elapsedMs), çıktısı
/// o anki söz satırı ve ışık durumudur. Ağ, saat ve UI bilmez — bu yüzden
/// birim testle doğrulanır ve tüm telefonlar aynı elapsed için aynı kareyi
/// üretir. Yanıp sönme fazı daima kue başlangıcına göre floor((e−at)·hz/500)
/// ile hesaplanır (Go/JS/Dart üçünde aynı aritmetik).
library;

import 'show_manifest.dart';
import 'spatial_effect.dart';
import 'venue.dart';

class TimelineFrame {
  const TimelineFrame({
    required this.done,
    required this.lyric,
    this.nextLyric = '',
    required this.screenColor,
    required this.screenLit,
    required this.torchOn,
  });

  /// Sekans bitti mi.
  final bool done;

  /// O an ekranda durması gereken söz satırı ('' = boş).
  final String lyric;

  /// Karaoke önizlemesi: sıradaki söz satırı ('' = kalmadı).
  final String nextLyric;

  /// Aktif screen kuesinin rengi (#RRGGBB; '' = kue yok → siyah).
  final String screenColor;

  /// Ekran şu an yanık yarımda mı (flash fazı).
  final bool screenLit;

  /// Fener şu an yanık mı.
  final bool torchOn;
}

/// Kare üreticisi sözleşmesi: hem tek sekans (TimelineEngine) hem otomatik
/// program (ProgramEngine) aynı arayüzle çalışır; gösteri ekranı ikisini
/// ayırt etmez.
abstract interface class FrameSource {
  TimelineFrame frameAt(int elapsedMs);

  /// Işığa duyarlı mod (F3.2): true iken yanıp sönme SABİT ışığa indirgenir
  /// (flashHz yok sayılır, kue süresi/rengi aynı kalır; uzamsal efekt kuenin
  /// sabit rengine döner). Gösteri ortasında açılabilir; bir sonraki
  /// kareden itibaren etkilidir.
  abstract bool disableFlash;

  /// Çözülmüş koltuk konumu (F4.1/F4.2): uzamsal efektler u/v/w normalize
  /// eksenlerini kullanır. null = koltuksuz telefon → mekânın ortası
  /// (0.5) sayılır; herkes katılır, kimse sessizce dışarıda kalmaz.
  abstract SeatPos? seatPos;
}

class TimelineEngine implements FrameSource {
  TimelineEngine(this.sequence, {this.disableFlash = false, this.seatPos});

  final ShowSequence sequence;

  @override
  bool disableFlash;

  @override
  SeatPos? seatPos;

  bool _lit(ShowCue cue, int elapsedMs) {
    if (disableFlash || cue.flashHz == 0) return true;
    final sinceCue = elapsedMs - cue.atMs;
    return ((sinceCue * cue.flashHz) ~/ 500).isEven;
  }

  /// Şeritte o an aktif kue (varsa). Aynı şeritte çakışma varsa son
  /// tanımlanan kazanır (yazarın en son eklediği niyettir).
  static ShowCue? _activeCue(CueLane lane, int elapsedMs) {
    ShowCue? active;
    for (final cue in lane.cues) {
      if (cue.activeAt(elapsedMs)) active = cue;
    }
    return active;
  }

  @override
  TimelineFrame frameAt(int elapsedMs) {
    if (elapsedMs >= sequence.durationMs) {
      return const TimelineFrame(
          done: true, lyric: '', screenColor: '', screenLit: false, torchOn: false);
    }

    var lyric = '';
    var nextLyric = '';
    var nextAtMs = 1 << 62;
    for (final line in sequence.lyricLines) {
      final end = line.durationMs == 0 ? sequence.durationMs : line.atMs + line.durationMs;
      if (elapsedMs >= line.atMs && elapsedMs < end) lyric = line.text;
      // Karaoke önizlemesi: henüz başlamamış en yakın satır.
      if (line.atMs > elapsedMs && line.atMs < nextAtMs) {
        nextAtMs = line.atMs;
        nextLyric = line.text;
      }
    }

    final u = seatPos?.u ?? 0.5, v = seatPos?.v ?? 0.5, w = seatPos?.w ?? 0.5;

    var screenColor = '';
    var screenLit = false;
    var torchOn = false;
    for (final lane in sequence.cueLanes) {
      final cue = _activeCue(lane, elapsedMs);
      if (cue == null) continue;
      switch (lane.kind) {
        case 'screen':
          final eff = cue.effect;
          if (eff == null) {
            screenColor = cue.color;
            screenLit = _lit(cue, elapsedMs);
          } else if (disableFlash) {
            // Duyarlı mod: efekt değerlendirilmez, kue rengi sabit yanar.
            screenColor = cue.color;
            screenLit = true;
          } else {
            final c = evalEffect(eff, cue.color, u, v, w, elapsedMs - cue.atMs);
            screenColor = c;
            screenLit = c.isNotEmpty;
          }
        case 'torch':
          final effT = cue.effect;
          torchOn = effT == null
              ? _lit(cue, elapsedMs)
              : disableFlash ||
                  evalEffect(effT, '#FFFFFF', u, v, w, elapsedMs - cue.atMs)
                      .isNotEmpty;
        case 'audio':
          // Zamanlanmış ses Faz 2'de native kanalla gelecek.
          break;
      }
    }

    return TimelineFrame(
      done: false,
      lyric: lyric,
      nextLyric: nextLyric,
      screenColor: screenColor,
      screenLit: screenLit,
      torchOn: torchOn,
    );
  }
}

/// Otomatik program motoru: program başlangıcından geçen süreye göre aktif
/// program öğesini bulur ve kareyi o öğenin sekans motoruna devreder.
/// Öğeler arasındaki boşlukta ekran karanlık bekler (done değil); son
/// öğenin sekansı bitince done olur. Bu da SAFTIR ve birim testlidir.
class ProgramEngine implements FrameSource {
  ProgramEngine(ShowManifest manifest,
      {bool disableFlash = false, SeatPos? seatPos})
      : _items = [
          for (final item in manifest.program)
            if (manifest.sequenceById(item.sequenceId) != null)
              (
                atOffsetMs: item.atOffsetMs,
                engine: TimelineEngine(manifest.sequenceById(item.sequenceId)!,
                    disableFlash: disableFlash, seatPos: seatPos),
              ),
        ],
        _disableFlash = disableFlash,
        _seatPos = seatPos;

  final List<({int atOffsetMs, TimelineEngine engine})> _items;

  bool _disableFlash;
  SeatPos? _seatPos;

  @override
  bool get disableFlash => _disableFlash;

  @override
  set disableFlash(bool value) {
    _disableFlash = value;
    for (final item in _items) {
      item.engine.disableFlash = value;
    }
  }

  @override
  SeatPos? get seatPos => _seatPos;

  @override
  set seatPos(SeatPos? value) {
    _seatPos = value;
    for (final item in _items) {
      item.engine.seatPos = value;
    }
  }

  static const _idle = TimelineFrame(
      done: false, lyric: '', screenColor: '', screenLit: false, torchOn: false);
  static const _done = TimelineFrame(
      done: true, lyric: '', screenColor: '', screenLit: false, torchOn: false);

  bool get isEmpty => _items.isEmpty;

  @override
  TimelineFrame frameAt(int elapsedMs) {
    if (_items.isEmpty) return _done;

    ({int atOffsetMs, TimelineEngine engine})? current;
    for (final item in _items) {
      if (item.atOffsetMs <= elapsedMs) current = item;
    }
    if (current == null) return _idle; // ilk öğe henüz başlamadı

    final frame = current.engine.frameAt(elapsedMs - current.atOffsetMs);
    if (!frame.done) return frame;
    // Aktif öğenin sekansı bitti: son öğeyse program bitti, değilse bir
    // sonraki öğeye kadar karanlık beklenir.
    return current == _items.last ? _done : _idle;
  }
}

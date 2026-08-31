/// Zaman çizelgesi motoru: bir sekansın herhangi bir anındaki kareyi üretir.
///
/// Tamamen SAFTIR: girdisi "sekans başından geçen süre" (elapsedMs), çıktısı
/// o anki söz satırı ve ışık durumudur. Ağ, saat ve UI bilmez — bu yüzden
/// birim testle doğrulanır ve tüm telefonlar aynı elapsed için aynı kareyi
/// üretir. Yanıp sönme fazı daima kue başlangıcına göre floor((e−at)·hz/500)
/// ile hesaplanır (Go/JS/Dart üçünde aynı aritmetik).
library;

import 'show_manifest.dart';

class TimelineFrame {
  const TimelineFrame({
    required this.done,
    required this.lyric,
    required this.screenColor,
    required this.screenLit,
    required this.torchOn,
  });

  /// Sekans bitti mi.
  final bool done;

  /// O an ekranda durması gereken söz satırı ('' = boş).
  final String lyric;

  /// Aktif screen kuesinin rengi (#RRGGBB; '' = kue yok → siyah).
  final String screenColor;

  /// Ekran şu an yanık yarımda mı (flash fazı).
  final bool screenLit;

  /// Fener şu an yanık mı.
  final bool torchOn;
}

class TimelineEngine {
  const TimelineEngine(this.sequence);

  final ShowSequence sequence;

  static bool _lit(ShowCue cue, int elapsedMs) {
    if (cue.flashHz == 0) return true;
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

  TimelineFrame frameAt(int elapsedMs) {
    if (elapsedMs >= sequence.durationMs) {
      return const TimelineFrame(
          done: true, lyric: '', screenColor: '', screenLit: false, torchOn: false);
    }

    var lyric = '';
    for (final line in sequence.lyricLines) {
      final end = line.durationMs == 0 ? sequence.durationMs : line.atMs + line.durationMs;
      if (elapsedMs >= line.atMs && elapsedMs < end) lyric = line.text;
    }

    var screenColor = '';
    var screenLit = false;
    var torchOn = false;
    for (final lane in sequence.cueLanes) {
      final cue = _activeCue(lane, elapsedMs);
      if (cue == null) continue;
      switch (lane.kind) {
        case 'screen':
          screenColor = cue.color;
          screenLit = _lit(cue, elapsedMs);
        case 'torch':
          torchOn = _lit(cue, elapsedMs);
        case 'audio':
          // Zamanlanmış ses Faz 2'de native kanalla gelecek.
          break;
      }
    }

    return TimelineFrame(
      done: false,
      lyric: lyric,
      screenColor: screenColor,
      screenLit: screenLit,
      torchOn: torchOn,
    );
  }
}

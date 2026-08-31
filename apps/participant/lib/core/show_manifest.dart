/// Gösteri manifesti modeli.
///
/// Şemanın gerçeği packages/manifest (Go) paketidir; alan adları birebir
/// izlenir. Zamanlar sekans başına göre milisaniyedir; mutlak zaman yoktur —
/// sekansın ne zaman başlayacağını CueStart söyler.
library;

class ShowManifest {
  const ShowManifest({required this.title, required this.sequences});

  final String title;
  final List<ShowSequence> sequences;

  ShowSequence? sequenceById(String id) {
    for (final seq in sequences) {
      if (seq.id == id) return seq;
    }
    return null;
  }

  static ShowManifest fromJson(Map<String, dynamic> j) => ShowManifest(
        title: j['title'] as String? ?? '',
        sequences: [
          for (final s in (j['sequences'] as List? ?? const []))
            ShowSequence.fromJson(s as Map<String, dynamic>),
        ],
      );
}

class ShowSequence {
  const ShowSequence({
    required this.id,
    required this.title,
    required this.durationMs,
    required this.lyricLines,
    required this.cueLanes,
  });

  final String id;
  final String title;
  final int durationMs;
  final List<LyricLine> lyricLines;
  final List<CueLane> cueLanes;

  static ShowSequence fromJson(Map<String, dynamic> j) => ShowSequence(
        id: j['id'] as String? ?? '',
        title: j['title'] as String? ?? '',
        durationMs: (j['duration_ms'] as num?)?.toInt() ?? 0,
        lyricLines: [
          for (final l in (j['lyric_lines'] as List? ?? const []))
            LyricLine.fromJson(l as Map<String, dynamic>),
        ],
        cueLanes: [
          for (final l in (j['cue_lanes'] as List? ?? const []))
            CueLane.fromJson(l as Map<String, dynamic>),
        ],
      );
}

class LyricLine {
  const LyricLine({required this.atMs, required this.durationMs, required this.text});

  final int atMs;
  final int durationMs;
  final String text;

  static LyricLine fromJson(Map<String, dynamic> j) => LyricLine(
        atMs: (j['at_ms'] as num?)?.toInt() ?? 0,
        durationMs: (j['duration_ms'] as num?)?.toInt() ?? 0,
        text: j['text'] as String? ?? '',
      );
}

class CueLane {
  const CueLane({required this.id, required this.kind, required this.cues});

  final String id;
  final String kind; // screen | torch | audio
  final List<ShowCue> cues;

  static CueLane fromJson(Map<String, dynamic> j) => CueLane(
        id: j['id'] as String? ?? '',
        kind: j['kind'] as String? ?? '',
        cues: [
          for (final c in (j['cues'] as List? ?? const []))
            ShowCue.fromJson(c as Map<String, dynamic>),
        ],
      );
}

class ShowCue {
  const ShowCue({
    required this.atMs,
    required this.durationMs,
    required this.color,
    required this.flashHz,
    required this.assetId,
  });

  final int atMs;
  final int durationMs;
  final String color;
  final int flashHz;
  final String assetId;

  bool activeAt(int elapsedMs) =>
      elapsedMs >= atMs && (durationMs == 0 || elapsedMs < atMs + durationMs);

  static ShowCue fromJson(Map<String, dynamic> j) => ShowCue(
        atMs: (j['at_ms'] as num?)?.toInt() ?? 0,
        durationMs: (j['duration_ms'] as num?)?.toInt() ?? 0,
        color: j['color'] as String? ?? '',
        flashHz: (j['flash_hz'] as num?)?.toInt() ?? 0,
        assetId: j['asset_id'] as String? ?? '',
      );
}

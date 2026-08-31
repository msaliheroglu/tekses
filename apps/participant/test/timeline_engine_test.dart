import 'package:flutter_test/flutter_test.dart';
import 'package:tekses_participant/core/show_manifest.dart';
import 'package:tekses_participant/core/timeline_engine.dart';

void main() {
  final sequence = ShowSequence(
    id: 'seq-1',
    title: 'Açılış',
    durationMs: 10000,
    lyricLines: const [
      LyricLine(atMs: 0, durationMs: 2000, text: 'Birinci satır'),
      LyricLine(atMs: 2000, durationMs: 0, text: 'Son satır'), // 0 = sekans sonuna dek
    ],
    cueLanes: const [
      CueLane(id: 'ekran', kind: 'screen', cues: [
        ShowCue(atMs: 1000, durationMs: 4000, color: '#FF2A2A', flashHz: 2, assetId: ''),
      ]),
      CueLane(id: 'fener', kind: 'torch', cues: [
        ShowCue(atMs: 1000, durationMs: 2000, color: '', flashHz: 0, assetId: ''),
      ]),
    ],
  );
  final engine = TimelineEngine(sequence);

  test('sozler zamana gore akar', () {
    expect(engine.frameAt(0).lyric, 'Birinci satır');
    expect(engine.frameAt(1999).lyric, 'Birinci satır');
    expect(engine.frameAt(2000).lyric, 'Son satır');
    expect(engine.frameAt(9999).lyric, 'Son satır');
  });

  test('ekran kuesi ve flash fazi kue baslangicina gore', () {
    expect(engine.frameAt(500).screenColor, ''); // kue henüz başlamadı
    // 2 Hz → yarım periyot 250 ms; faz kue başına (1000 ms) göre.
    expect(engine.frameAt(1000).screenLit, isTrue);
    expect(engine.frameAt(1249).screenLit, isTrue);
    expect(engine.frameAt(1250).screenLit, isFalse);
    expect(engine.frameAt(1500).screenLit, isTrue);
    expect(engine.frameAt(1000).screenColor, '#FF2A2A');
    expect(engine.frameAt(5000).screenColor, ''); // kue bitti
  });

  test('fener kuesi sabit yanar ve suresinde soner', () {
    expect(engine.frameAt(999).torchOn, isFalse);
    expect(engine.frameAt(1000).torchOn, isTrue);
    expect(engine.frameAt(2999).torchOn, isTrue);
    expect(engine.frameAt(3000).torchOn, isFalse);
  });

  test('sekans sonunda done', () {
    expect(engine.frameAt(9999).done, isFalse);
    final done = engine.frameAt(10000);
    expect(done.done, isTrue);
    expect(done.torchOn, isFalse);
    expect(done.lyric, '');
  });
}

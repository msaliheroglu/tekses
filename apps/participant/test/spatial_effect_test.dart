import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:tekses_participant/core/show_manifest.dart';
import 'package:tekses_participant/core/spatial_effect.dart';
import 'package:tekses_participant/core/timeline_engine.dart';
import 'package:tekses_participant/core/venue.dart';

/// Uzamsal efekt değerlendirmesinin Go referansıyla (packages/manifest)
/// çapraz doğrulaması. Dosyanın üreticisi:
/// go test ./packages/manifest -run TestEffectGolden -update
void main() {
  test('altın vektörler: Go ile aynı (efekt, konum, an) → aynı renk', () {
    final j = jsonDecode(
      File('../../packages/manifest/testdata/effect_vectors.json')
          .readAsStringSync(),
    ) as Map<String, dynamic>;
    final cases = j['cases'] as List;
    expect(cases, isNotEmpty);

    for (final c in cases.cast<Map<String, dynamic>>()) {
      final eff = EffectSpec.fromJson(c['effect'] as Map<String, dynamic>);
      final got = evalEffect(
        eff,
        c['cue_color'] as String,
        (c['u'] as num).toDouble(),
        (c['v'] as num).toDouble(),
        (c['w'] as num).toDouble(),
        c['block'] as String? ?? '',
        (c['since_ms'] as num).toInt(),
      );
      expect(got, c['want'] as String? ?? '', reason: c['name'] as String);
    }
  });

  test('motor: söz izleyen renk döngüsü satırla birlikte değişir', () {
    final seq = ShowSequence.fromJson({
      'id': 's1',
      'title': 'Marş',
      'duration_ms': 20000,
      'lyric_lines': [
        {'at_ms': 2000, 'duration_ms': 2000, 'text': 'Birinci satır'},
        {'at_ms': 5000, 'duration_ms': 2000, 'text': 'İkinci satır'},
      ],
      'cue_lanes': [
        {
          'id': 'l1',
          'kind': 'screen',
          'cues': [
            {
              'at_ms': 0,
              'duration_ms': 20000,
              'color': '#FFFFFF',
              'effect': {
                'kind': 'cycle',
                'colors': ['#FF0000', '#00FF00', '#0000FF'],
              },
            },
          ],
        },
      ],
    });
    final engine = TimelineEngine(seq);
    expect(engine.frameAt(1000).screenColor, '#FF0000'); // henüz söz yok
    expect(engine.frameAt(3000).screenColor, '#00FF00'); // 1. satır başladı
    expect(engine.frameAt(6000).screenColor, '#0000FF'); // 2. satır başladı
  });

  test('motor: blok filtresi yalnız kapsamdaki blokta oynar', () {
    final seq = ShowSequence.fromJson({
      'id': 's1',
      'title': 'Tek tribün',
      'duration_ms': 10000,
      'cue_lanes': [
        {
          'id': 'l1',
          'kind': 'screen',
          'cues': [
            {
              'at_ms': 0,
              'duration_ms': 10000,
              'color': '#FF0000',
              'effect': {
                'kind': 'wave',
                'period_ms': 2000,
                'width': 1.0,
                'blocks': ['KUZEY'],
              },
            },
          ],
        },
      ],
    });
    expect(TimelineEngine(seq, seatBlock: 'KUZEY').frameAt(1000).screenLit,
        isTrue);
    expect(
        TimelineEngine(seq, seatBlock: 'DOGU').frameAt(1000).screenLit, isFalse);
    // Koltuksuz telefon blok filtreli efektte kapsam dışıdır.
    expect(TimelineEngine(seq).frameAt(1000).screenLit, isFalse);
  });

  // Dalga efektli tek ekran kuesi: koltuğa göre kimi telefon bantta (yanık),
  // kimi dışında (karanlık) — motor düzeyinde uçtan uca.
  ShowSequence waveSequence() => ShowSequence.fromJson({
        'id': 's1',
        'title': 'Dalga',
        'duration_ms': 10000,
        'cue_lanes': [
          {
            'id': 'l1',
            'kind': 'screen',
            'cues': [
              {
                'at_ms': 0,
                'duration_ms': 10000,
                'color': '#FF0000',
                'effect': {'kind': 'wave', 'period_ms': 2000, 'width': 0.2},
              },
            ],
          },
        ],
      });

  const bandSeat = SeatPos(x: 0, y: 0, z: 0, u: 0.5, v: 0, w: 0); // cephede
  const farSeat = SeatPos(x: 0, y: 0, z: 0, u: 0.9, v: 0, w: 0); // bant dışı

  test('motor: dalga koltuğa göre farklı kare üretir', () {
    // t=1000: cephe 0.5'te.
    final inBand = TimelineEngine(waveSequence(), seatPos: bandSeat).frameAt(1000);
    expect(inBand.screenColor, '#FF0000');
    expect(inBand.screenLit, isTrue);

    final outBand = TimelineEngine(waveSequence(), seatPos: farSeat).frameAt(1000);
    expect(outBand.screenColor, '');
    expect(outBand.screenLit, isFalse);

    // Aynı koltuk, dalga üzerine gelince yanar: u=0.9 → cephe 0.9 at t=1800.
    final later = TimelineEngine(waveSequence(), seatPos: farSeat).frameAt(1800);
    expect(later.screenLit, isTrue);
  });

  test('motor: koltuksuz telefon mekânın ortası sayılır (u=0.5)', () {
    final frame = TimelineEngine(waveSequence()).frameAt(1000); // cephe 0.5
    expect(frame.screenLit, isTrue);
  });

  test('motor: duyarlı mod efekti sabit kue rengine indirger', () {
    final engine = TimelineEngine(waveSequence(), seatPos: farSeat)
      ..disableFlash = true;
    // Bant dışı koltuk bile sabit yanar; dalga karartması yok.
    final frame = engine.frameAt(1000);
    expect(frame.screenColor, '#FF0000');
    expect(frame.screenLit, isTrue);
  });

  test('motor: fener şeridinde dalga açık/kapalı verir', () {
    final seq = ShowSequence.fromJson({
      'id': 's1',
      'title': 'Fener dalgası',
      'duration_ms': 10000,
      'cue_lanes': [
        {
          'id': 'l1',
          'kind': 'torch',
          'cues': [
            {
              'at_ms': 0,
              'duration_ms': 10000,
              'effect': {'kind': 'wave', 'period_ms': 2000, 'width': 0.2},
            },
          ],
        },
      ],
    });
    expect(TimelineEngine(seq, seatPos: bandSeat).frameAt(1000).torchOn, isTrue);
    expect(TimelineEngine(seq, seatPos: farSeat).frameAt(1000).torchOn, isFalse);
  });

  test('ProgramEngine seatPos ayarı çocuk motorlara yayılır', () {
    final manifest = ShowManifest.fromJson({
      'title': 'D',
      'sequences': [
        {
          'id': 's1',
          'title': 'S1',
          'duration_ms': 10000,
          'cue_lanes': [
            {
              'id': 'l1',
              'kind': 'screen',
              'cues': [
                {
                  'at_ms': 0,
                  'duration_ms': 10000,
                  'color': '#FF0000',
                  'effect': {'kind': 'wave', 'period_ms': 2000, 'width': 0.2},
                },
              ],
            },
          ],
        },
      ],
      'program': [
        {'sequence_id': 's1', 'at_offset_ms': 0},
      ],
    });
    final engine = ProgramEngine(manifest, seatPos: farSeat);
    expect(engine.frameAt(1000).screenLit, isFalse);
    engine.seatPos = bandSeat; // geç çözülen koltuk süren koşuya işler
    expect(engine.frameAt(1000).screenLit, isTrue);
  });
}

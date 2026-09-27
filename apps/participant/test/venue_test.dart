import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:tekses_participant/core/show_manifest.dart';
import 'package:tekses_participant/core/venue.dart';

/// Mekân çözücüsünün Go referansıyla (packages/manifest) çapraz doğrulaması:
/// iki gerçekleme altın vektör dosyasındaki koltukları AYNI konuma çözmeli.
/// Dosyanın üreticisi: go test ./packages/manifest -run TestVenueGolden -update
void main() {
  void checkGolden(Venue venue, List cases) {
    expect(cases, isNotEmpty);
    for (final c in cases.cast<Map<String, dynamic>>()) {
      final seat = c['seat'] as String;
      final ref = SeatRef.parse(seat);
      expect(ref, isNotNull, reason: seat);
      final pos = venue.resolve(ref!);
      expect(pos, isNotNull, reason: seat);
      final got = [pos!.x, pos.y, pos.z, pos.u, pos.v, pos.w];
      final want = ['x', 'y', 'z', 'u', 'v', 'w']
          .map((k) => (c[k] as num).toDouble())
          .toList();
      for (var i = 0; i < 6; i++) {
        expect(got[i], closeTo(want[i], 1e-9), reason: '$seat bileşen $i');
      }
    }
  }

  test('altın vektörler: Go ile aynı koltuk → aynı konum (grid + yay)', () {
    final j = jsonDecode(
      File('../../packages/manifest/testdata/venue_vectors.json')
          .readAsStringSync(),
    ) as Map<String, dynamic>;
    checkGolden(
        Venue.fromJson(j['venue'] as Map<String, dynamic>), j['cases'] as List);
    // Yay (arc) blok: köşe/oval tribün — sıra başına koltuk yarıçapla artar.
    checkGolden(Venue.fromJson(j['arc_venue'] as Map<String, dynamic>),
        j['arc_cases'] as List);
  });

  test('yay bloğunda sıra kapasitesi sıraya göre değişir', () {
    final j = jsonDecode(
      File('../../packages/manifest/testdata/venue_vectors.json')
          .readAsStringSync(),
    ) as Map<String, dynamic>;
    final venue = Venue.fromJson(j['arc_venue'] as Map<String, dynamic>);
    final b = venue.blocks.single;
    expect(b.seatsInRow(1), 32); // 10 m yarıçap, çeyrek tur, 0.5 m aralık
    expect(b.seatsInRow(10), 55); // 17.2 m yarıçapta yay uzar
    expect(venue.resolve(const SeatRef(block: 'KOSE', row: 10, seat: 55)),
        isNotNull);
    expect(venue.resolve(const SeatRef(block: 'KOSE', row: 10, seat: 56)),
        isNull);
    expect(venue.resolve(const SeatRef(block: 'KOSE', row: 1, seat: 33)),
        isNull);
  });

  test('koltuk dizgisi çözümü: tireli blok, geçersiz biçimler', () {
    final r = SeatRef.parse('DOGU-ALT-3-7')!;
    expect(r.block, 'DOGU-ALT');
    expect(r.row, 3);
    expect(r.seat, 7);
    expect(r.toString(), 'DOGU-ALT-3-7');

    for (final bad in ['', 'A-1', 'A-0-1', 'A-1-0', 'A-x-1', '-1-2']) {
      expect(SeatRef.parse(bad), isNull, reason: bad);
    }
  });

  test('bilinmeyen blok ve ızgara dışı koltuk null döner', () {
    const venue = Venue(name: '', blocks: [
      VenueBlock(
        id: 'A',
        rows: 2,
        seatsPerRow: 3,
        origin: Vec3(0, 0, 0),
        rowVec: Vec3(0, 1, 0),
        seatVec: Vec3(1, 0, 0),
      ),
    ]);
    expect(venue.resolve(const SeatRef(block: 'B', row: 1, seat: 1)), isNull);
    expect(venue.resolve(const SeatRef(block: 'A', row: 3, seat: 1)), isNull);
    expect(venue.resolve(const SeatRef(block: 'A', row: 1, seat: 4)), isNull);
    expect(
      venue.resolve(const SeatRef(block: 'A', row: 2, seat: 3)),
      isNotNull,
    );
  });

  test('manifest: venue isteğe bağlı — yokken null, varken çözülür', () {
    final withoutVenue = ShowManifest.fromJson({
      'title': 'Deneme',
      'sequences': [
        {'id': 's1', 'title': 'S1', 'duration_ms': 1000},
      ],
    });
    expect(withoutVenue.venue, isNull);

    final withVenue = ShowManifest.fromJson({
      'title': 'Deneme',
      'sequences': [
        {'id': 's1', 'title': 'S1', 'duration_ms': 1000},
      ],
      'venue': {
        'name': 'Salon',
        'blocks': [
          {
            'id': 'A',
            'rows': 1,
            'seats_per_row': 2,
            'origin': {'x': 0, 'y': 0, 'z': 0},
            'seat_vec': {'x': 0.5, 'y': 0, 'z': 0},
          },
        ],
      },
    });
    expect(withVenue.venue, isNotNull);
    expect(withVenue.venue!.blocks.single.id, 'A');
    // Tek koltuk sırası: x ekseni normalize edilir, y/z sıfır genişlikte 0.5.
    final pos = withVenue.venue!
        .resolve(const SeatRef(block: 'A', row: 1, seat: 2))!;
    expect(pos.x, closeTo(0.5, 1e-9));
    expect(pos.u, closeTo(1, 1e-9));
    expect(pos.v, closeTo(0.5, 1e-9));
    expect(pos.w, closeTo(0.5, 1e-9));
  });
}

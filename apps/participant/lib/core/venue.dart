/// Mekân modeli (Faz 4, koltuk bazlı koreografi).
///
/// Şemanın ve matematiğin gerçeği packages/manifest/venue.go'dur; iki
/// gerçekleme packages/manifest/testdata/venue_vectors.json altın
/// vektörlerine karşı test edilir ve aynı koltuğu AYNI konuma çözmelidir.
/// Koltuk konumu = origin + (sıra-1)·row_vec + (koltuk-1)·seat_vec (metre);
/// efektler mekân sınır kutusuna normalize u/v/w (0..1) eksenlerini kullanır.
/// Doğrulama sunucudadır (yayın anında) — telefon savunmacı çözer: bilinmeyen
/// blok ya da ızgara dışı koltuk null döner ("koltuk bulunamadı").
library;

import 'dart:math' as math;

/// Sıfıra yapışık eksen/adım eşiği (packages/manifest.minStepMeters).
const double _minStepMeters = 0.001;

class Vec3 {
  const Vec3(this.x, this.y, this.z);

  final double x;
  final double y;
  final double z;

  static Vec3 fromJson(Map<String, dynamic>? j) => Vec3(
        ((j ?? const {})['x'] as num?)?.toDouble() ?? 0,
        ((j ?? const {})['y'] as num?)?.toDouble() ?? 0,
        ((j ?? const {})['z'] as num?)?.toDouble() ?? 0,
      );
}

class VenueBlock {
  const VenueBlock({
    required this.id,
    this.kind = '',
    required this.rows,
    required this.seatsPerRow,
    required this.origin,
    required this.rowVec,
    required this.seatVec,
    this.center = const Vec3(0, 0, 0),
    this.radius = 0,
    this.rowStep = 0,
    this.rake = 0,
    this.angleStartDeg = 0,
    this.angleEndDeg = 0,
    this.seatStep = 0,
  });

  final String id;
  final String kind; // '' | 'grid' | 'arc'
  final int rows;

  // grid alanları:
  final int seatsPerRow;
  final Vec3 origin; // 1. sıra 1. koltuk (metre)
  final Vec3 rowVec; // sıra r → r+1
  final Vec3 seatVec; // koltuk s → s+1

  // arc alanları (köşe/oval tribün): koltuklar center etrafında yaya dizilir.
  final Vec3 center;
  final double radius; // 1. sıranın yarıçapı (m)
  final double rowStep; // sıra başına yarıçap artışı
  final double rake; // sıra başına yükselme
  final double angleStartDeg; // koltuk 1 bu uçta
  final double angleEndDeg;
  final double seatStep; // yay boyunca koltuk aralığı

  bool get isArc => kind == 'arc';

  static VenueBlock fromJson(Map<String, dynamic> j) => VenueBlock(
        id: j['id'] as String? ?? '',
        kind: j['kind'] as String? ?? '',
        rows: (j['rows'] as num?)?.toInt() ?? 0,
        seatsPerRow: (j['seats_per_row'] as num?)?.toInt() ?? 0,
        origin: Vec3.fromJson(j['origin'] as Map<String, dynamic>?),
        rowVec: Vec3.fromJson(j['row_vec'] as Map<String, dynamic>?),
        seatVec: Vec3.fromJson(j['seat_vec'] as Map<String, dynamic>?),
        center: Vec3.fromJson(j['center'] as Map<String, dynamic>?),
        radius: (j['radius'] as num?)?.toDouble() ?? 0,
        rowStep: (j['row_step'] as num?)?.toDouble() ?? 0,
        rake: (j['rake'] as num?)?.toDouble() ?? 0,
        angleStartDeg: (j['angle_start_deg'] as num?)?.toDouble() ?? 0,
        angleEndDeg: (j['angle_end_deg'] as num?)?.toDouble() ?? 0,
        seatStep: (j['seat_step'] as num?)?.toDouble() ?? 0,
      );

  /// O sıradaki koltuk sayısı: grid'de sabit, yayda yay uzunluğundan
  /// (Go Block.SeatsInRow ile birebir).
  int seatsInRow(int row) {
    if (!isArc) return seatsPerRow;
    if (seatStep <= 0) return 1;
    final r = radius + (row - 1) * rowStep;
    final span = (angleEndDeg - angleStartDeg) * math.pi / 180;
    final n = (r * span / seatStep).floor() + 1;
    return n < 1 ? 1 : n;
  }

  /// Bloktaki (1 tabanlı) sıra/koltuğun metre konumu (Go seatWorld aynası).
  Vec3 seatWorld(int row, int seat) {
    if (isArc) {
      final r = radius + (row - 1) * rowStep;
      final theta = angleStartDeg * math.pi / 180 + (seat - 1) * seatStep / r;
      return Vec3(
        center.x + r * math.cos(theta),
        center.y + r * math.sin(theta),
        center.z + (row - 1) * rake,
      );
    }
    return Vec3(
      origin.x + rowVec.x * (row - 1) + seatVec.x * (seat - 1),
      origin.y + rowVec.y * (row - 1) + seatVec.y * (seat - 1),
      origin.z + rowVec.z * (row - 1) + seatVec.z * (seat - 1),
    );
  }

  /// Sınır kutusunu geren örnek noktalar (Go boundsSamples aynası): grid'de
  /// dört köşe; yayda uç sıralar × (uçlar + aradaki 90° katları).
  List<Vec3> boundsSamples() {
    if (!isArc) {
      return [
        seatWorld(1, 1),
        seatWorld(1, seatsPerRow),
        seatWorld(rows, 1),
        seatWorld(rows, seatsPerRow),
      ];
    }
    final angles = <double>[angleStartDeg, angleEndDeg];
    for (var k = (angleStartDeg / 90).ceilToDouble(); k * 90 < angleEndDeg; k++) {
      if (k * 90 > angleStartDeg) angles.add(k * 90);
    }
    final out = <Vec3>[];
    for (final row in [1, rows]) {
      final r = radius + (row - 1) * rowStep;
      final z = center.z + (row - 1) * rake;
      for (final deg in angles) {
        final th = deg * math.pi / 180;
        out.add(Vec3(center.x + r * math.cos(th), center.y + r * math.sin(th), z));
      }
    }
    return out;
  }
}

/// Koltuk adresi (1 tabanlı). Dizgi biçimi "BLOK-SIRA-KOLTUK"; blok kimliği
/// tire içerebilir, son iki parça daima sıra/koltuktur (Go ParseSeatRef ile
/// aynı kural; büyük/küçük harf katlanmaz).
class SeatRef {
  const SeatRef({required this.block, required this.row, required this.seat});

  final String block;
  final int row;
  final int seat;

  static SeatRef? parse(String s) {
    final parts = s.split('-');
    if (parts.length < 3) return null;
    final row = int.tryParse(parts[parts.length - 2]);
    final seat = int.tryParse(parts[parts.length - 1]);
    if (row == null || row < 1 || seat == null || seat < 1) return null;
    final block = parts.sublist(0, parts.length - 2).join('-');
    if (block.isEmpty) return null;
    return SeatRef(block: block, row: row, seat: seat);
  }

  @override
  String toString() => '$block-$row-$seat';
}

/// Çözülmüş koltuk konumu: metre (x/y/z) + mekân sınır kutusuna normalize
/// 0..1 eksenler (u/v/w; sıfır genişlikli eksen 0.5).
class SeatPos {
  const SeatPos({
    required this.x,
    required this.y,
    required this.z,
    required this.u,
    required this.v,
    required this.w,
  });

  final double x;
  final double y;
  final double z;
  final double u;
  final double v;
  final double w;
}

class Venue {
  const Venue({required this.name, required this.blocks});

  final String name;
  final List<VenueBlock> blocks;

  static Venue fromJson(Map<String, dynamic> j) => Venue(
        name: j['name'] as String? ?? '',
        blocks: [
          for (final b in (j['blocks'] as List? ?? const []))
            VenueBlock.fromJson(b as Map<String, dynamic>),
        ],
      );

  /// Koltuğu konuma çözer; bilinmeyen blok ya da ızgara dışı sıra/koltuk
  /// null döner. Sınır kutusu blok köşelerinden hesaplanır (konum doğrusal,
  /// uç değerler köşededir — Go Bounds ile aynı).
  SeatPos? resolve(SeatRef ref) {
    VenueBlock? blk;
    for (final b in blocks) {
      if (b.id == ref.block) {
        blk = b;
        break;
      }
    }
    if (blk == null) return null;
    if (ref.row < 1 || ref.row > blk.rows) return null;
    if (ref.seat < 1 || ref.seat > blk.seatsInRow(ref.row)) return null;

    final p = blk.seatWorld(ref.row, ref.seat);

    var first = true;
    var minX = 0.0, minY = 0.0, minZ = 0.0, maxX = 0.0, maxY = 0.0, maxZ = 0.0;
    for (final b in blocks) {
      for (final c in b.boundsSamples()) {
        if (first) {
          minX = maxX = c.x;
          minY = maxY = c.y;
          minZ = maxZ = c.z;
          first = false;
          continue;
        }
        minX = math.min(minX, c.x);
        maxX = math.max(maxX, c.x);
        minY = math.min(minY, c.y);
        maxY = math.max(maxY, c.y);
        minZ = math.min(minZ, c.z);
        maxZ = math.max(maxZ, c.z);
      }
    }

    return SeatPos(
      x: p.x,
      y: p.y,
      z: p.z,
      u: _normAxis(p.x, minX, maxX),
      v: _normAxis(p.y, minY, maxY),
      w: _normAxis(p.z, minZ, maxZ),
    );
  }
}

double _normAxis(double p, double lo, double hi) {
  if (hi - lo < _minStepMeters) return 0.5;
  return (p - lo) / (hi - lo);
}

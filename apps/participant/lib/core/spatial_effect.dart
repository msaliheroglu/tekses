/// Uzamsal efekt değerlendirmesi (F4.2).
///
/// Referans gerçekleme packages/manifest/effect.go'dur; buradaki aritmetik
/// onunla ve join.html JS'iyle BİREBİRDİR ve
/// packages/manifest/testdata/effect_vectors.json altın vektörlerine karşı
/// test edilir. Değiştirirken üçünü birden değiştir, vektörleri yeniden üret.
/// Doğrulama sunucudadır (yayın anında); telefon geleni savunmacı okur.
library;

import 'dart:math' as math;

/// Efekt türleri (packages/manifest ile aynı adlar).
const String effectWave = 'wave';
const String effectGradient = 'gradient';
const String effectBitmap = 'bitmap';
const String effectCycle = 'cycle';

class EffectBitmapSpec {
  const EffectBitmapSpec({required this.palette, required this.rows});

  final List<String> palette;
  final List<String> rows;

  static EffectBitmapSpec fromJson(Map<String, dynamic> j) => EffectBitmapSpec(
        palette: [
          for (final c in (j['palette'] as List? ?? const [])) c as String
        ],
        rows: [for (final r in (j['rows'] as List? ?? const [])) r as String],
      );
}

/// Bitmap yerleştirme penceresi (normalize yatay×dikey dikdörtgen).
class EffectAreaSpec {
  const EffectAreaSpec({
    required this.u0,
    required this.w0,
    required this.u1,
    required this.w1,
  });

  final double u0;
  final double w0;
  final double u1;
  final double w1;

  static EffectAreaSpec fromJson(Map<String, dynamic> j) => EffectAreaSpec(
        u0: (j['u0'] as num?)?.toDouble() ?? 0,
        w0: (j['w0'] as num?)?.toDouble() ?? 0,
        u1: (j['u1'] as num?)?.toDouble() ?? 1,
        w1: (j['w1'] as num?)?.toDouble() ?? 1,
      );
}

class EffectSpec {
  const EffectSpec({
    required this.kind,
    this.axis = '',
    this.reverse = false,
    this.periodMs = 0,
    this.width = 0,
    this.color2 = '',
    this.bitmap,
    this.blocks = const [],
    this.area,
    this.colors = const [],
  });

  final String kind;
  final String axis; // '' = u; 'ring' = mekân merkezi etrafındaki açı
  final bool reverse;
  final int periodMs;
  final double width;
  final String color2;
  final EffectBitmapSpec? bitmap;

  /// Yalnız bu bloklar oynar (boş = herkes). Koltuksuz telefon filtre
  /// varken kapsam dışıdır.
  final List<String> blocks;
  final EffectAreaSpec? area;

  /// cycle: renk listesi. periodMs = 0 → söz satırını izler (motor düzeyi).
  final List<String> colors;

  static EffectSpec fromJson(Map<String, dynamic> j) => EffectSpec(
        kind: j['kind'] as String? ?? '',
        axis: j['axis'] as String? ?? '',
        reverse: j['reverse'] as bool? ?? false,
        periodMs: (j['period_ms'] as num?)?.toInt() ?? 0,
        width: (j['width'] as num?)?.toDouble() ?? 0,
        color2: j['color2'] as String? ?? '',
        bitmap: j['bitmap'] == null
            ? null
            : EffectBitmapSpec.fromJson(j['bitmap'] as Map<String, dynamic>),
        blocks: [
          for (final b in (j['blocks'] as List? ?? const [])) b as String
        ],
        area: j['area'] == null
            ? null
            : EffectAreaSpec.fromJson(j['area'] as Map<String, dynamic>),
        colors: [
          for (final c in (j['colors'] as List? ?? const [])) c as String
        ],
      );
}

double _axisPos(EffectSpec e, double u, double v, double w) {
  var p = u;
  if (e.axis == 'v') p = v;
  if (e.axis == 'w') p = w;
  if (e.axis == 'ring') {
    // Mekân merkezinin etrafındaki açı payı (0..1; +u yönünden başlar,
    // saat yönünün tersine): dalga/kaydırma stadyumu tribün tribün turlar.
    p = math.atan2(v - 0.5, u - 0.5) / (2 * math.pi);
    if (p < 0) p += 1;
  }
  return e.reverse ? 1 - p : p;
}

int _channel(String c, int at) => int.parse(c.substring(at, at + 2), radix: 16);

String _lerpColor(String c1, String c2, double t) {
  String hex2(int v) => v.toRadixString(16).padLeft(2, '0').toUpperCase();
  final r = (_channel(c1, 1) + (_channel(c2, 1) - _channel(c1, 1)) * t).round();
  final g = (_channel(c1, 3) + (_channel(c2, 3) - _channel(c1, 3)) * t).round();
  final b = (_channel(c1, 5) + (_channel(c2, 5) - _channel(c1, 5)) * t).round();
  return '#${hex2(r)}${hex2(g)}${hex2(b)}';
}

int? _hexDigit(int codeUnit) {
  if (codeUnit >= 0x30 && codeUnit <= 0x39) return codeUnit - 0x30; // 0-9
  if (codeUnit >= 0x61 && codeUnit <= 0x66) return codeUnit - 0x61 + 10; // a-f
  return null;
}

/// Efektin (u,v,w) konumundaki, [block] bloğundaki telefona kue
/// başlangıcından [sinceMs] sonra basacağı renk; '' = kapalı (siyah /
/// fener sönük). Bozuk/bilinmeyen efekt de '' döner (savunmacı: sunucu
/// doğrulaması normalde buna izin vermez). cycle'da periodMs = 0 (söz
/// izleme) MOTOR düzeyinde işlenir; burada ilk renk döner.
String evalEffect(EffectSpec e, String cueColor, double u, double v, double w,
    String block, int sinceMs) {
  if (e.blocks.isNotEmpty && !e.blocks.contains(block)) {
    return ''; // blok filtresi: kapsam dışı (koltuksuz telefon dahil)
  }
  switch (e.kind) {
    case effectWave:
      if (e.periodMs <= 0) return '';
      final p = _axisPos(e, u, v, w);
      final front = (sinceMs % e.periodMs) / e.periodMs;
      var d = (p - front).abs();
      if (d > 0.5) d = 1 - d; // süpürme sargılıdır: 0.95 ile 0.05 komşudur
      return d <= e.width / 2 ? cueColor : e.color2;
    case effectGradient:
      if (e.color2.length != 7 || cueColor.length != 7) return '';
      return _lerpColor(cueColor, e.color2, _axisPos(e, u, v, w));
    case effectCycle:
      if (e.colors.isEmpty) return '';
      if (e.periodMs <= 0) return e.colors[0];
      return e.colors[(sinceMs ~/ e.periodMs) % e.colors.length];
    case effectBitmap:
      final b = e.bitmap;
      if (b == null || b.rows.isEmpty || b.rows[0].isEmpty) return '';
      final cols = b.rows[0].length;
      var p = _axisPos(e, u, v, w);
      var q = w; // dikey daima w
      final a = e.area;
      if (a != null) {
        // Yerleştirme penceresi: dışı kapalı, içi 0..1'e haritalanır.
        if (p < a.u0 || p > a.u1 || q < a.w0 || q > a.w1) return '';
        p = (p - a.u0) / (a.u1 - a.u0);
        q = (q - a.w0) / (a.w1 - a.w0);
      }
      if (e.periodMs > 0) {
        // Kaydırma: desen p ekseninde sola akar, dönem başına bir tam tur.
        p += (sinceMs % e.periodMs) / e.periodMs;
        if (p >= 1) p -= 1;
      }
      var col = (p * cols).floor();
      if (col >= cols) col = cols - 1; // p=1 ucu son sütuna yapışır
      if (col < 0) col = 0;
      final nrows = b.rows.length;
      var rowIdx = ((1 - q) * nrows).floor(); // ilk satır = tepe
      if (rowIdx >= nrows) rowIdx = nrows - 1;
      if (rowIdx < 0) rowIdx = 0;
      final row = b.rows[rowIdx];
      if (col >= row.length) return '';
      final ch = row.codeUnitAt(col);
      if (ch == 0x2E) return ''; // '.'
      final idx = _hexDigit(ch);
      if (idx == null || idx >= b.palette.length) return '';
      return b.palette[idx];
  }
  return '';
}

/// Söz izleyen renk döngüsü (cycle, periodMs = 0): başlamış söz satırı
/// sayısı mod renk sayısı. Kural üç motor gerçeklemesinde birebirdir
/// (join.html JS, panel effectEval.ts) — değiştirirken üçünü değiştir.
String cycleLyricColor(EffectSpec e, String block, int lyricsStarted) {
  if (e.blocks.isNotEmpty && !e.blocks.contains(block)) return '';
  if (e.colors.isEmpty) return '';
  return e.colors[lyricsStarted % e.colors.length];
}

import 'package:flutter_test/flutter_test.dart';
import 'package:tekses_participant/core/package_store.dart';

/// Varlık URL kurulumu (F3.3): sunucu asset_base_url verdiyse indirme
/// CDN'den, vermediyse control-api'nin /assets ucundan yapılır. Mutlak
/// manifest_url zaten Uri.resolve ile olduğu gibi geçer.
void main() {
  final controlBase = Uri.parse('https://tekses.example/control');

  test('taban yokken /assets control-api üzerinden', () {
    expect(
      PackageStore.assetUri(controlBase, '', 'abc.mp3').toString(),
      'https://tekses.example/assets/abc.mp3',
    );
  });

  test('taban verilmişse varlık CDN tabanından iner', () {
    expect(
      PackageStore.assetUri(
              controlBase, 'https://cdn.tekses.example', 'abc.mp3')
          .toString(),
      'https://cdn.tekses.example/abc.mp3',
    );
  });

  test('mutlak URL, controlBase.resolve ile olduğu gibi geçer', () {
    // Paket indirme yolunun dayandığı Uri davranışı: manifest_url mutlaksa
    // control tabanı devre dışı kalır (join yanıtı CDN URL'si döndüğünde).
    expect(
      controlBase.resolve('https://cdn.tekses.example/x.json').toString(),
      'https://cdn.tekses.example/x.json',
    );
    expect(
      controlBase.resolve('/packages/x.json').toString(),
      'https://tekses.example/packages/x.json',
    );
  });
}

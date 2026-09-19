import 'dart:convert';
import 'dart:io';

import 'package:crypto/crypto.dart';
import 'package:http/http.dart' as http;
import 'package:path_provider/path_provider.dart';

import 'show_manifest.dart';

/// Katılım bilgisi + önceden indirilen gösteri paketi.
///
/// Yönetici ilke: ağ HAZIRLIK içindir. Odaya girerken paket iner ve SHA-256
/// ile doğrulanır; kue anında ağ gerekmez. Faz 1'de paket bellekte tutulur
/// (tek gösteri oturumu); disk önbelleği sonraki yineleme.
class JoinInfo {
  const JoinInfo({
    required this.roomId,
    required this.roomName,
    required this.eventName,
    required this.showVersionId,
    required this.sha256,
    required this.manifest,
    this.assetPaths = const {},
  });

  final String roomId;
  final String roomName;
  final String eventName;
  final String showVersionId; // '' = odada aktif gösteri yok
  final String sha256;
  final ShowManifest? manifest;

  /// asset_id → indirilen yerel dosya yolu (ses varlıkları; özet doğrulanmış).
  final Map<String, String> assetPaths;
}

class PackageStoreException implements Exception {
  PackageStoreException(this.message);
  final String message;
  @override
  String toString() => message;
}

class PackageStore {
  PackageStore({http.Client? client}) : _client = client ?? http.Client();

  final http.Client _client;

  /// Katılım kodunu control-api'den çözer ve aktif gösteri paketini indirir.
  Future<JoinInfo> join(Uri controlBase, String joinCode) async {
    final joinUri = controlBase.resolve('/api/v1/join/${joinCode.trim().toUpperCase()}');
    final resp = await _client.get(joinUri).timeout(const Duration(seconds: 10));
    if (resp.statusCode == 404) {
      throw PackageStoreException('Katılım kodu geçersiz');
    }
    if (resp.statusCode != 200) {
      throw PackageStoreException('Katılım başarısız (HTTP ${resp.statusCode})');
    }
    final body = jsonDecode(utf8.decode(resp.bodyBytes)) as Map<String, dynamic>;

    final sv = body['show_version'] as Map<String, dynamic>?;
    if (sv == null) {
      return JoinInfo(
        roomId: body['room_id'] as String? ?? '',
        roomName: body['room_name'] as String? ?? '',
        eventName: body['event_name'] as String? ?? '',
        showVersionId: '',
        sha256: '',
        manifest: null,
      );
    }

    final expectedSha = sv['sha256'] as String? ?? '';
    final manifestUrl = sv['manifest_url'] as String? ?? '';
    ShowManifest? manifest;

    if (manifestUrl.isNotEmpty) {
      // Sözleşme: paketi URL'den indir, HAM baytlar üzerinden özet doğrula.
      final pkgResp = await _client
          .get(controlBase.resolve(manifestUrl))
          .timeout(const Duration(seconds: 30));
      if (pkgResp.statusCode != 200) {
        throw PackageStoreException('Paket indirilemedi (HTTP ${pkgResp.statusCode})');
      }
      final actualSha = sha256.convert(pkgResp.bodyBytes).toString();
      if (expectedSha.isNotEmpty && actualSha != expectedSha) {
        throw PackageStoreException('Paket doğrulaması başarısız: özet uyuşmuyor');
      }
      manifest = ShowManifest.fromJson(
          jsonDecode(utf8.decode(pkgResp.bodyBytes)) as Map<String, dynamic>);
    } else if (sv['manifest'] is Map<String, dynamic>) {
      // Yedek yol: join yanıtındaki gömülü manifest (özet doğrulanamaz,
      // baytlar JSON içinde yeniden biçimlenmiştir).
      manifest = ShowManifest.fromJson(sv['manifest'] as Map<String, dynamic>);
    }

    // Ses varlıkları da odaya girerken iner (karar: ön yükleme) ve addaki
    // özetle doğrulanır; kue anında ağ gerekmez.
    var assetPaths = const <String, String>{};
    if (manifest != null) {
      assetPaths = await _downloadAssets(controlBase, manifest);
    }

    return JoinInfo(
      roomId: body['room_id'] as String? ?? '',
      roomName: body['room_name'] as String? ?? '',
      eventName: body['event_name'] as String? ?? '',
      showVersionId: sv['id'] as String? ?? '',
      sha256: expectedSha,
      manifest: manifest,
      assetPaths: assetPaths,
    );
  }

  Future<Map<String, String>> _downloadAssets(
      Uri controlBase, ShowManifest manifest) async {
    final ids = <String>{};
    for (final seq in manifest.sequences) {
      for (final lane in seq.cueLanes) {
        if (lane.kind != 'audio') continue;
        for (final cue in lane.cues) {
          if (cue.assetId.isNotEmpty) ids.add(cue.assetId);
        }
      }
    }
    if (ids.isEmpty) return const {};

    final dir = Directory(
        '${(await getApplicationSupportDirectory()).path}/tekses_assets');
    await dir.create(recursive: true);

    final paths = <String, String>{};
    for (final id in ids) {
      final file = File('${dir.path}/$id');
      final expectedSha = id.split('.').first;
      // Önbellek: dosya varsa ve özeti tutuyorsa yeniden indirilmez
      // (asset_id içerik adresli olduğu için ad = içerik garantisi).
      if (await file.exists()) {
        final existing = await file.readAsBytes();
        if (sha256.convert(existing).toString() == expectedSha) {
          paths[id] = file.path;
          continue;
        }
      }
      final resp = await _client
          .get(controlBase.resolve('/assets/$id'))
          .timeout(const Duration(minutes: 2));
      if (resp.statusCode != 200) {
        throw PackageStoreException(
            'Ses varlığı indirilemedi: $id (HTTP ${resp.statusCode})');
      }
      if (sha256.convert(resp.bodyBytes).toString() != expectedSha) {
        throw PackageStoreException('Ses varlığı doğrulanamadı: $id');
      }
      await file.writeAsBytes(resp.bodyBytes, flush: true);
      paths[id] = file.path;
    }
    return paths;
  }
}

/// v2 ikili tel kodeği: protobuf `Envelope` ↔ uygulamanın JSON-biçimli
/// mesaj haritaları.
///
/// Şemanın gerçeği packages/proto/tekses/v1/*.proto; Dart stub'ları
/// `lib/gen/` altına üretilir (packages/proto/README.md). Bu köprü, Go
/// tarafındaki wire/binary.go'nun eşleniğidir: çözülen ikili çerçeve,
/// JSON yolundakiyle AYNI (type, data) çiftine indirgenir ki üst katman
/// (messages.dart'taki fromJson'lar, realtime_client) kodekten bağımsız
/// kalsın. İki gerçekleme, packages/proto/wire/testdata/golden_frames.txt
/// altın baytlarına karşı birlikte test edilir — tel sapması orada yakalanır.
library;

import 'dart:typed_data';

import 'package:fixnum/fixnum.dart';

import '../gen/tekses/v1/clock.pb.dart' as pbclock;
import '../gen/tekses/v1/cue.pb.dart' as pbcue;
import '../gen/tekses/v1/cue.pbenum.dart' as pbenum;
import '../gen/tekses/v1/envelope.pb.dart' as pbenv;
import 'messages.dart';

const Map<pbenum.InterventionKind, String> _kindNames = {
  pbenum.InterventionKind.INTERVENTION_KIND_HOLD: 'HOLD',
  pbenum.InterventionKind.INTERVENTION_KIND_STOP: 'STOP',
  pbenum.InterventionKind.INTERVENTION_KIND_SKIP: 'SKIP',
  pbenum.InterventionKind.INTERVENTION_KIND_BLACKOUT: 'BLACKOUT',
};

/// İstemcinin GÖNDERDİĞİ mesajları ikili zarfa kodlar. Telefon yalnızca
/// hello ve clock_sync_request yollar; başka tür istemek programlama
/// hatasıdır ve ArgumentError fırlatır.
Uint8List encodeBinaryEnvelope(String type, Map<String, dynamic> data) {
  final env = pbenv.Envelope();
  switch (type) {
    case typeHello:
      env.hello = pbenv.Hello(
        protocolVersion: (data['protocol_version'] as int?) ?? 0,
        joinCode: (data['join_code'] as String?) ?? '',
        clientKind: (data['client_kind'] as String?) ?? '',
      );
    case typeClockSyncRequest:
      env.clockSyncRequest = pbclock.ClockSyncRequest(
        seq: (data['seq'] as int?) ?? 0,
        clientMonoMs: Int64((data['client_mono_ms'] as int?) ?? 0),
      );
    default:
      throw ArgumentError('istemci $type göndermez');
  }
  return env.writeToBuffer();
}

/// İkili zarfı JSON yolundakiyle aynı (type, data) çiftine çözer.
/// Bozuk baytlar ve bilinmeyen türler null döner (bağlantı düşürülmez;
/// gateway ileride yeni tür eklerse eski uygulama çerçeveyi atlar).
({String type, Map<String, dynamic> data})? decodeBinaryEnvelope(
    List<int> raw) {
  final pbenv.Envelope env;
  try {
    env = pbenv.Envelope.fromBuffer(raw);
  } catch (_) {
    return null;
  }
  switch (env.whichKind()) {
    case pbenv.Envelope_Kind.welcome:
      final w = env.welcome;
      return (type: typeWelcome, data: {
        'server_time_ms': w.serverTimeMs.toInt(),
        'protocol_version': w.protocolVersion,
        'room_id': w.roomId,
      });
    case pbenv.Envelope_Kind.clockSyncResponse:
      final r = env.clockSyncResponse;
      return (type: typeClockSyncResponse, data: {
        'seq': r.seq,
        'client_mono_ms': r.clientMonoMs.toInt(),
        'server_recv_ms': r.serverRecvMs.toInt(),
        'server_send_ms': r.serverSendMs.toInt(),
      });
    case pbenv.Envelope_Kind.cueStart:
      final c = env.cueStart;
      return (type: typeCueStart, data: {
        'run_id': c.runId,
        'cue_id': c.cueId,
        'fire_at_server_ms': c.fireAtServerMs.toInt(),
        'repeat_seq': c.repeatSeq,
        'payload': _payloadMap(c.payload),
      });
    case pbenv.Envelope_Kind.intervention:
      final iv = env.intervention;
      final kind = _kindNames[iv.kind];
      if (kind == null) return null;
      return (type: typeIntervention, data: {
        'run_id': iv.runId,
        'kind': kind,
        'issued_at_server_ms': iv.issuedAtServerMs.toInt(),
      });
    case pbenv.Envelope_Kind.showActivated:
      final sa = env.showActivated;
      return (type: typeShowActivated, data: {
        'room_id': sa.roomId,
        'show_version_id': sa.showVersionId,
      });
    default:
      // hello/clockSyncRequest sunucudan gelmez; notSet ve bilinmeyenler atlanır.
      return null;
  }
}

Map<String, dynamic> _payloadMap(pbcue.CuePayload p) => {
      'color': p.color,
      'torch': p.torch,
      'flash_hz': p.flashHz,
      'duration_ms': p.durationMs,
    };

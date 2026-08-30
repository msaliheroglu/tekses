import 'dart:convert';

/// Faz 0 JSON tel türleri. Şemanın gerçeği packages/proto/tekses/v1/*.proto;
/// alan adları (snake_case) proto'yu birebir izler ve Go tarafındaki
/// packages/proto/wire ile aynı tutulmalıdır.
library;

const int protocolVersion = 1;

const String typeHello = 'hello';
const String typeWelcome = 'welcome';
const String typeClockSyncRequest = 'clock_sync_request';
const String typeClockSyncResponse = 'clock_sync_response';
const String typeCueStart = 'cue_start';
const String typeIntervention = 'intervention';

/// Zarf: {"type": "...", "data": {...}}.
String encodeEnvelope(String type, Map<String, dynamic> data) =>
    jsonEncode({'type': type, 'data': data});

({String type, Map<String, dynamic> data})? decodeEnvelope(dynamic raw) {
  if (raw is! String) return null;
  final dynamic parsed;
  try {
    parsed = jsonDecode(raw);
  } on FormatException {
    return null;
  }
  if (parsed is! Map<String, dynamic>) return null;
  final type = parsed['type'];
  final data = parsed['data'];
  if (type is! String || data is! Map<String, dynamic>) return null;
  return (type: type, data: data);
}

class WelcomeMsg {
  const WelcomeMsg({required this.serverTimeMs, required this.roomId});

  final int serverTimeMs;
  final String roomId;

  static WelcomeMsg fromJson(Map<String, dynamic> j) => WelcomeMsg(
        serverTimeMs: (j['server_time_ms'] as num?)?.toInt() ?? 0,
        roomId: j['room_id'] as String? ?? '',
      );
}

class ClockSyncResponseMsg {
  const ClockSyncResponseMsg({
    required this.seq,
    required this.clientMonoMs,
    required this.serverRecvMs,
    required this.serverSendMs,
  });

  final int seq;
  final int clientMonoMs;
  final int serverRecvMs;
  final int serverSendMs;

  static ClockSyncResponseMsg fromJson(Map<String, dynamic> j) =>
      ClockSyncResponseMsg(
        seq: (j['seq'] as num?)?.toInt() ?? 0,
        clientMonoMs: (j['client_mono_ms'] as num?)?.toInt() ?? 0,
        serverRecvMs: (j['server_recv_ms'] as num?)?.toInt() ?? 0,
        serverSendMs: (j['server_send_ms'] as num?)?.toInt() ?? 0,
      );
}

class CuePayloadMsg {
  const CuePayloadMsg({
    required this.color,
    required this.torch,
    required this.flashHz,
    required this.durationMs,
  });

  /// #RRGGBB.
  final String color;
  final bool torch;
  final int flashHz;
  final int durationMs;

  static CuePayloadMsg fromJson(Map<String, dynamic> j) => CuePayloadMsg(
        color: j['color'] as String? ?? '#FFFFFF',
        torch: j['torch'] as bool? ?? false,
        flashHz: (j['flash_hz'] as num?)?.toInt() ?? 0,
        durationMs: (j['duration_ms'] as num?)?.toInt() ?? 3000,
      );
}

class CueStartMsg {
  const CueStartMsg({
    required this.runId,
    required this.cueId,
    required this.fireAtServerMs,
    required this.repeatSeq,
    required this.payload,
  });

  final String runId;
  final String cueId;
  final int fireAtServerMs;
  final int repeatSeq;
  final CuePayloadMsg payload;

  static CueStartMsg? fromJson(Map<String, dynamic> j) {
    final runId = j['run_id'] as String?;
    final fireAt = (j['fire_at_server_ms'] as num?)?.toInt();
    if (runId == null || runId.isEmpty || fireAt == null) return null;
    return CueStartMsg(
      runId: runId,
      cueId: j['cue_id'] as String? ?? '',
      fireAtServerMs: fireAt,
      repeatSeq: (j['repeat_seq'] as num?)?.toInt() ?? 0,
      payload: CuePayloadMsg.fromJson(
          (j['payload'] as Map<String, dynamic>?) ?? const {}),
    );
  }
}

class InterventionMsg {
  const InterventionMsg({required this.runId, required this.kind});

  final String runId;

  /// HOLD | STOP | SKIP | BLACKOUT.
  final String kind;

  static InterventionMsg? fromJson(Map<String, dynamic> j) {
    final kind = j['kind'] as String?;
    if (kind == null || kind.isEmpty) return null;
    return InterventionMsg(runId: j['run_id'] as String? ?? '', kind: kind);
  }
}

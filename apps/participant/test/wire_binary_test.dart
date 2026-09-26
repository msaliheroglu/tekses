import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:tekses_participant/core/messages.dart';
import 'package:tekses_participant/core/wire_binary.dart';

/// Altın çerçeve çapraz doğrulaması: Go referansının ürettiği baytlar
/// (packages/proto/wire/testdata/golden_frames.txt, üretici golden_test.go)
/// burada Dart kodeğine karşı sınanır. İki gerçekleme aynı mesaj için aynı
/// baytları üretmeli/çözmeli — tel sapması bu dosyada yakalanır.
void main() {
  final lines = File('../../packages/proto/wire/testdata/golden_frames.txt')
      .readAsLinesSync()
      .where((l) => l.trim().isNotEmpty && !l.startsWith('#'))
      .map((l) => l.split('\t'))
      .toList();
  final golden = {for (final p in lines) p[0]: _unhex(p[1])};

  test('altın dosya tüm türleri kapsıyor', () {
    expect(
        golden.keys.toSet(),
        {
          typeHello,
          typeWelcome,
          typeClockSyncRequest,
          typeClockSyncResponse,
          typeCueStart,
          typeIntervention,
          typeShowActivated,
        },
        reason: 'tür kümesi değiştiyse iki taraftaki testler birlikte güncellenmeli');
  });

  test('istemcinin gönderdikleri Go ile bayt-bayt aynı kodlanır', () {
    expect(
      encodeBinaryEnvelope(typeHello, {
        'protocol_version': 2,
        'join_code': 'ABC234',
        'client_kind': 'flutter',
      }),
      golden[typeHello],
    );
    expect(
      encodeBinaryEnvelope(typeClockSyncRequest, {
        'seq': 7,
        'client_mono_ms': 123456,
      }),
      golden[typeClockSyncRequest],
    );
  });

  test('sunucudan gelenler Go altın baytlarından çözülür', () {
    final welcome = decodeBinaryEnvelope(golden[typeWelcome]!)!;
    expect(welcome.type, typeWelcome);
    expect(welcome.data, {
      'server_time_ms': 1789816791704,
      'protocol_version': 2,
      'room_id': 'room_1',
    });

    final clock = decodeBinaryEnvelope(golden[typeClockSyncResponse]!)!;
    expect(clock.type, typeClockSyncResponse);
    final resp = ClockSyncResponseMsg.fromJson(clock.data);
    expect(resp.seq, 7);
    expect(resp.clientMonoMs, 123456);
    expect(resp.serverRecvMs, 1789816791800);
    expect(resp.serverSendMs, 1789816791801);

    final cueEnv = decodeBinaryEnvelope(golden[typeCueStart]!)!;
    expect(cueEnv.type, typeCueStart);
    final cue = CueStartMsg.fromJson(cueEnv.data)!;
    expect(cue.runId, 'c3a4f80219eb0db9');
    expect(cue.cueId, 'program');
    expect(cue.fireAtServerMs, 1789816792298);
    expect(cue.repeatSeq, 2);
    expect(cue.payload.color, '#FF2A2A');
    expect(cue.payload.torch, isTrue);
    expect(cue.payload.flashHz, 2);
    expect(cue.payload.durationMs, 4000);

    final ivEnv = decodeBinaryEnvelope(golden[typeIntervention]!)!;
    expect(ivEnv.type, typeIntervention);
    final iv = InterventionMsg.fromJson(ivEnv.data)!;
    expect(iv.runId, 'r1');
    expect(iv.kind, 'BLACKOUT');

    final saEnv = decodeBinaryEnvelope(golden[typeShowActivated]!)!;
    expect(saEnv.type, typeShowActivated);
    final sa = ShowActivatedMsg.fromJson(saEnv.data);
    expect(sa.roomId, 'room_1');
    expect(sa.showVersionId, 'sv_42');
  });

  test('bozuk baytlar ve boş zarf null döner, fırlatmaz', () {
    expect(decodeBinaryEnvelope([0xff, 0x00, 0x13, 0x37]), isNull);
    expect(decodeBinaryEnvelope(const []), isNull); // boş zarf: kind notSet
    // İstemcinin göndermediği bir türü kodlamak programlama hatasıdır.
    expect(() => encodeBinaryEnvelope(typeWelcome, const {}), throwsArgumentError);
  });

  test('ikili kue çerçevesi JSON eşleniğinden bariz küçük', () {
    final jsonBytes = utf8.encode(encodeEnvelope(typeCueStart, {
      'run_id': 'c3a4f80219eb0db9',
      'cue_id': 'program',
      'fire_at_server_ms': 1789816792298,
      'repeat_seq': 2,
      'payload': {
        'color': '#FF2A2A',
        'torch': true,
        'flash_hz': 2,
        'duration_ms': 4000,
      },
    }));
    final bin = golden[typeCueStart]!;
    expect(bin.length * 2, lessThan(jsonBytes.length),
        reason: 'ikili (${bin.length} B) JSON\'un (${jsonBytes.length} B) '
            'yarısından küçük olmalı');
  });
}

List<int> _unhex(String s) => [
      for (var i = 0; i < s.length; i += 2)
        int.parse(s.substring(i, i + 2), radix: 16),
    ];

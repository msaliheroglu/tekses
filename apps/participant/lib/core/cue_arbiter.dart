import 'messages.dart';

/// Kue adaylarının kaynağı.
enum CueSource { websocket, schedule, ultrasonic }

/// Cue Arbiter — karar: "ilk gelen kazanır, sonra sadık kal".
///
/// Bir runId için ilk geçerli adayın KAYNAĞINA kilitlenir; aynı runId'nin
/// tekrarları (paket kaybına karşı 3 kez yayınlanır) ve diğer kaynaklardan
/// gelen kopyaları yok sayılır. Faz 0'da tek kaynak WebSocket'tir; otomatik
/// program ve ultrasonik beacon eklendiğinde bu yapı olduğu gibi genişler
/// (WS ile ultrasonik ~250 ms içinde çakışırsa hassas olan kazanır).
class CueArbiter {
  CueArbiter({required this.onAccepted});

  final void Function(CueStartMsg cue, CueSource source) onAccepted;

  final Map<String, CueSource> _lockedRuns = {};

  void offer(CueStartMsg cue, CueSource source) {
    if (_lockedRuns.containsKey(cue.runId)) return;
    _lockedRuns[cue.runId] = source;
    onAccepted(cue, source);
  }

  CueSource? lockedSource(String runId) => _lockedRuns[runId];
}

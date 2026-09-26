// This is a generated file - do not edit.
//
// Generated from tekses/v1/cue.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'cue.pbenum.dart';

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

export 'cue.pbenum.dart';

class CueStart extends $pb.GeneratedMessage {
  factory CueStart({
    $core.String? runId,
    $core.String? cueId,
    $fixnum.Int64? fireAtServerMs,
    $core.int? repeatSeq,
    CuePayload? payload,
  }) {
    final result = CueStart._();
    if (runId != null) result.runId = runId;
    if (cueId != null) result.cueId = cueId;
    if (fireAtServerMs != null) result.fireAtServerMs = fireAtServerMs;
    if (repeatSeq != null) result.repeatSeq = repeatSeq;
    if (payload != null) result.payload = payload;
    return result;
  }

  CueStart._();

  factory CueStart.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CueStart()..mergeFromBuffer(data, registry);
  factory CueStart.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CueStart()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CueStart',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'tekses.v1'),
      createEmptyInstance: CueStart.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'runId')
    ..aOS(2, _omitFieldNames ? '' : 'cueId')
    ..aInt64(3, _omitFieldNames ? '' : 'fireAtServerMs')
    ..aI(4, _omitFieldNames ? '' : 'repeatSeq', fieldType: $pb.PbFieldType.OU3)
    ..aOM<CuePayload>(5, _omitFieldNames ? '' : 'payload',
        subBuilder: CuePayload.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CueStart clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CueStart copyWith(void Function(CueStart) updates) =>
      super.copyWith((message) => updates(message as CueStart)) as CueStart;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CueStart() / CueStart.new instead')
  static CueStart create() => CueStart._();
  static $pb.GeneratedMessage $_createMessage() => CueStart._();
  @$core.override
  CueStart createEmptyInstance() => CueStart._();
  @$core.pragma('dart2js:noInline')
  static CueStart getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CueStart>(CueStart.$_createMessage);
  static CueStart? _defaultInstance;

  /// Çalıştırma kimliği; tekilleştirme ve kaynak kilidi anahtarı.
  @$pb.TagNumber(1)
  $core.String get runId => $_getSZ(0);
  @$pb.TagNumber(1)
  set runId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRunId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRunId() => $_clearField(1);

  /// Gösteri manifestindeki kue kimliği.
  @$pb.TagNumber(2)
  $core.String get cueId => $_getSZ(1);
  @$pb.TagNumber(2)
  set cueId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasCueId() => $_has(1);
  @$pb.TagNumber(2)
  void clearCueId() => $_clearField(2);

  /// Ateşleme anı, sunucu saatinde (ms). İstemci saat ofsetiyle kendi
  /// monoton saatine çevirir; bu andan sonra ağa ihtiyaç yoktur.
  @$pb.TagNumber(3)
  $fixnum.Int64 get fireAtServerMs => $_getI64(2);
  @$pb.TagNumber(3)
  set fireAtServerMs($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasFireAtServerMs() => $_has(2);
  @$pb.TagNumber(3)
  void clearFireAtServerMs() => $_clearField(3);

  /// 1..N tekrar sayacı (tanılama için; tekilleştirme run_id iledir).
  @$pb.TagNumber(4)
  $core.int get repeatSeq => $_getIZ(3);
  @$pb.TagNumber(4)
  set repeatSeq($core.int value) => $_setUnsignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasRepeatSeq() => $_has(3);
  @$pb.TagNumber(4)
  void clearRepeatSeq() => $_clearField(4);

  /// Faz 0: görsel yük tetik içinde taşınır. Faz 1'de gösteri paketi
  /// referansına (show_version + sequence) dönüşecek.
  @$pb.TagNumber(5)
  CuePayload get payload => $_getN(4);
  @$pb.TagNumber(5)
  set payload(CuePayload value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasPayload() => $_has(4);
  @$pb.TagNumber(5)
  void clearPayload() => $_clearField(5);
  @$pb.TagNumber(5)
  CuePayload ensurePayload() => $_ensure(4);
}

class CuePayload extends $pb.GeneratedMessage {
  factory CuePayload({
    $core.String? color,
    $core.bool? torch,
    $core.int? flashHz,
    $core.int? durationMs,
  }) {
    final result = CuePayload._();
    if (color != null) result.color = color;
    if (torch != null) result.torch = torch;
    if (flashHz != null) result.flashHz = flashHz;
    if (durationMs != null) result.durationMs = durationMs;
    return result;
  }

  CuePayload._();

  factory CuePayload.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CuePayload()..mergeFromBuffer(data, registry);
  factory CuePayload.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      CuePayload()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'CuePayload',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'tekses.v1'),
      createEmptyInstance: CuePayload.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'color')
    ..aOB(2, _omitFieldNames ? '' : 'torch')
    ..aI(3, _omitFieldNames ? '' : 'flashHz', fieldType: $pb.PbFieldType.OU3)
    ..aI(4, _omitFieldNames ? '' : 'durationMs', fieldType: $pb.PbFieldType.OU3)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CuePayload clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  CuePayload copyWith(void Function(CuePayload) updates) =>
      super.copyWith((message) => updates(message as CuePayload)) as CuePayload;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use CuePayload() / CuePayload.new instead')
  static CuePayload create() => CuePayload._();
  static $pb.GeneratedMessage $_createMessage() => CuePayload._();
  @$core.override
  CuePayload createEmptyInstance() => CuePayload._();
  @$core.pragma('dart2js:noInline')
  static CuePayload getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<CuePayload>(CuePayload.$_createMessage);
  static CuePayload? _defaultInstance;

  /// Ekran rengi, #RRGGBB.
  @$pb.TagNumber(1)
  $core.String get color => $_getSZ(0);
  @$pb.TagNumber(1)
  set color($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasColor() => $_has(0);
  @$pb.TagNumber(1)
  void clearColor() => $_clearField(1);

  /// Telefon feneri kullanılsın mı.
  @$pb.TagNumber(2)
  $core.bool get torch => $_getBF(1);
  @$pb.TagNumber(2)
  set torch($core.bool value) => $_setBool(1, value);
  @$pb.TagNumber(2)
  $core.bool hasTorch() => $_has(1);
  @$pb.TagNumber(2)
  void clearTorch() => $_clearField(2);

  /// Yanıp sönme frekansı (tam döngü/sn). 0 = sabit yanar.
  /// Işığa duyarlılık kuralı: sürekli yanıp sönme <= 3 Hz.
  @$pb.TagNumber(3)
  $core.int get flashHz => $_getIZ(2);
  @$pb.TagNumber(3)
  set flashHz($core.int value) => $_setUnsignedInt32(2, value);
  @$pb.TagNumber(3)
  $core.bool hasFlashHz() => $_has(2);
  @$pb.TagNumber(3)
  void clearFlashHz() => $_clearField(3);

  /// Efekt süresi (ms).
  @$pb.TagNumber(4)
  $core.int get durationMs => $_getIZ(3);
  @$pb.TagNumber(4)
  set durationMs($core.int value) => $_setUnsignedInt32(3, value);
  @$pb.TagNumber(4)
  $core.bool hasDurationMs() => $_has(3);
  @$pb.TagNumber(4)
  void clearDurationMs() => $_clearField(4);
}

/// Canlı müdahale. Sekans başladıktan sonra telden giden tek şey budur.
class Intervention extends $pb.GeneratedMessage {
  factory Intervention({
    $core.String? runId,
    InterventionKind? kind,
    $fixnum.Int64? issuedAtServerMs,
  }) {
    final result = Intervention._();
    if (runId != null) result.runId = runId;
    if (kind != null) result.kind = kind;
    if (issuedAtServerMs != null) result.issuedAtServerMs = issuedAtServerMs;
    return result;
  }

  Intervention._();

  factory Intervention.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Intervention()..mergeFromBuffer(data, registry);
  factory Intervention.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Intervention()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Intervention',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'tekses.v1'),
      createEmptyInstance: Intervention.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'runId')
    ..aE<InterventionKind>(2, _omitFieldNames ? '' : 'kind',
        enumValues: InterventionKind.values)
    ..aInt64(3, _omitFieldNames ? '' : 'issuedAtServerMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Intervention clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Intervention copyWith(void Function(Intervention) updates) =>
      super.copyWith((message) => updates(message as Intervention))
          as Intervention;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Intervention() / Intervention.new instead')
  static Intervention create() => Intervention._();
  static $pb.GeneratedMessage $_createMessage() => Intervention._();
  @$core.override
  Intervention createEmptyInstance() => Intervention._();
  @$core.pragma('dart2js:noInline')
  static Intervention getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<Intervention>(
          Intervention.$_createMessage);
  static Intervention? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get runId => $_getSZ(0);
  @$pb.TagNumber(1)
  set runId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRunId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRunId() => $_clearField(1);

  @$pb.TagNumber(2)
  InterventionKind get kind => $_getN(1);
  @$pb.TagNumber(2)
  set kind(InterventionKind value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasKind() => $_has(1);
  @$pb.TagNumber(2)
  void clearKind() => $_clearField(2);

  @$pb.TagNumber(3)
  $fixnum.Int64 get issuedAtServerMs => $_getI64(2);
  @$pb.TagNumber(3)
  set issuedAtServerMs($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasIssuedAtServerMs() => $_has(2);
  @$pb.TagNumber(3)
  void clearIssuedAtServerMs() => $_clearField(3);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');

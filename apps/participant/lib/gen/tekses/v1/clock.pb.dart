// This is a generated file - do not edit.
//
// Generated from tekses/v1/clock.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

class ClockSyncRequest extends $pb.GeneratedMessage {
  factory ClockSyncRequest({
    $core.int? seq,
    $fixnum.Int64? clientMonoMs,
  }) {
    final result = ClockSyncRequest._();
    if (seq != null) result.seq = seq;
    if (clientMonoMs != null) result.clientMonoMs = clientMonoMs;
    return result;
  }

  ClockSyncRequest._();

  factory ClockSyncRequest.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClockSyncRequest()..mergeFromBuffer(data, registry);
  factory ClockSyncRequest.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClockSyncRequest()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClockSyncRequest',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'tekses.v1'),
      createEmptyInstance: ClockSyncRequest.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'seq', fieldType: $pb.PbFieldType.OU3)
    ..aInt64(2, _omitFieldNames ? '' : 'clientMonoMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClockSyncRequest clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClockSyncRequest copyWith(void Function(ClockSyncRequest) updates) =>
      super.copyWith((message) => updates(message as ClockSyncRequest))
          as ClockSyncRequest;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ClockSyncRequest() / ClockSyncRequest.new instead')
  static ClockSyncRequest create() => ClockSyncRequest._();
  static $pb.GeneratedMessage $_createMessage() => ClockSyncRequest._();
  @$core.override
  ClockSyncRequest createEmptyInstance() => ClockSyncRequest._();
  @$core.pragma('dart2js:noInline')
  static ClockSyncRequest getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ClockSyncRequest>(
          ClockSyncRequest.$_createMessage);
  static ClockSyncRequest? _defaultInstance;

  /// İstemcinin örnek sayacı; yanıt eşleştirmede kullanılır.
  @$pb.TagNumber(1)
  $core.int get seq => $_getIZ(0);
  @$pb.TagNumber(1)
  set seq($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSeq() => $_has(0);
  @$pb.TagNumber(1)
  void clearSeq() => $_clearField(1);

  /// t0: istemcinin monoton saatinden milisaniye. Sunucu yorumlamaz,
  /// yanıtta aynen geri yansıtır.
  @$pb.TagNumber(2)
  $fixnum.Int64 get clientMonoMs => $_getI64(1);
  @$pb.TagNumber(2)
  set clientMonoMs($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasClientMonoMs() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientMonoMs() => $_clearField(2);
}

class ClockSyncResponse extends $pb.GeneratedMessage {
  factory ClockSyncResponse({
    $core.int? seq,
    $fixnum.Int64? clientMonoMs,
    $fixnum.Int64? serverRecvMs,
    $fixnum.Int64? serverSendMs,
  }) {
    final result = ClockSyncResponse._();
    if (seq != null) result.seq = seq;
    if (clientMonoMs != null) result.clientMonoMs = clientMonoMs;
    if (serverRecvMs != null) result.serverRecvMs = serverRecvMs;
    if (serverSendMs != null) result.serverSendMs = serverSendMs;
    return result;
  }

  ClockSyncResponse._();

  factory ClockSyncResponse.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClockSyncResponse()..mergeFromBuffer(data, registry);
  factory ClockSyncResponse.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ClockSyncResponse()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ClockSyncResponse',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'tekses.v1'),
      createEmptyInstance: ClockSyncResponse.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'seq', fieldType: $pb.PbFieldType.OU3)
    ..aInt64(2, _omitFieldNames ? '' : 'clientMonoMs')
    ..aInt64(3, _omitFieldNames ? '' : 'serverRecvMs')
    ..aInt64(4, _omitFieldNames ? '' : 'serverSendMs')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClockSyncResponse clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ClockSyncResponse copyWith(void Function(ClockSyncResponse) updates) =>
      super.copyWith((message) => updates(message as ClockSyncResponse))
          as ClockSyncResponse;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ClockSyncResponse() / ClockSyncResponse.new instead')
  static ClockSyncResponse create() => ClockSyncResponse._();
  static $pb.GeneratedMessage $_createMessage() => ClockSyncResponse._();
  @$core.override
  ClockSyncResponse createEmptyInstance() => ClockSyncResponse._();
  @$core.pragma('dart2js:noInline')
  static ClockSyncResponse getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ClockSyncResponse>(
          ClockSyncResponse.$_createMessage);
  static ClockSyncResponse? _defaultInstance;

  @$pb.TagNumber(1)
  $core.int get seq => $_getIZ(0);
  @$pb.TagNumber(1)
  set seq($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasSeq() => $_has(0);
  @$pb.TagNumber(1)
  void clearSeq() => $_clearField(1);

  /// t0 yansıması.
  @$pb.TagNumber(2)
  $fixnum.Int64 get clientMonoMs => $_getI64(1);
  @$pb.TagNumber(2)
  set clientMonoMs($fixnum.Int64 value) => $_setInt64(1, value);
  @$pb.TagNumber(2)
  $core.bool hasClientMonoMs() => $_has(1);
  @$pb.TagNumber(2)
  void clearClientMonoMs() => $_clearField(2);

  /// t1: isteğin sunucuya ulaştığı an (sunucu saati, ms).
  @$pb.TagNumber(3)
  $fixnum.Int64 get serverRecvMs => $_getI64(2);
  @$pb.TagNumber(3)
  set serverRecvMs($fixnum.Int64 value) => $_setInt64(2, value);
  @$pb.TagNumber(3)
  $core.bool hasServerRecvMs() => $_has(2);
  @$pb.TagNumber(3)
  void clearServerRecvMs() => $_clearField(3);

  /// t2: yanıtın sunucudan çıktığı an (sunucu saati, ms).
  @$pb.TagNumber(4)
  $fixnum.Int64 get serverSendMs => $_getI64(3);
  @$pb.TagNumber(4)
  set serverSendMs($fixnum.Int64 value) => $_setInt64(3, value);
  @$pb.TagNumber(4)
  $core.bool hasServerSendMs() => $_has(3);
  @$pb.TagNumber(4)
  void clearServerSendMs() => $_clearField(4);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');

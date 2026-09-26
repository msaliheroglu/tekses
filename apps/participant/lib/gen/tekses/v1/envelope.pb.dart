// This is a generated file - do not edit.
//
// Generated from tekses/v1/envelope.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports

import 'dart:core' as $core;

import 'package:fixnum/fixnum.dart' as $fixnum;
import 'package:protobuf/protobuf.dart' as $pb;

import 'clock.pb.dart' as $0;
import 'cue.pb.dart' as $1;

export 'package:protobuf/protobuf.dart' show GeneratedMessageGenericExtensions;

/// Katılım el sıkışması. Faz 0'da oda tekildir; oda kodu Faz 1'de anlam kazanır.
class Hello extends $pb.GeneratedMessage {
  factory Hello({
    $core.int? protocolVersion,
    $core.String? joinCode,
    $core.String? clientKind,
  }) {
    final result = Hello._();
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (joinCode != null) result.joinCode = joinCode;
    if (clientKind != null) result.clientKind = clientKind;
    return result;
  }

  Hello._();

  factory Hello.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Hello()..mergeFromBuffer(data, registry);
  factory Hello.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Hello()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Hello',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'tekses.v1'),
      createEmptyInstance: Hello.$_createMessage)
    ..aI(1, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(2, _omitFieldNames ? '' : 'joinCode')
    ..aOS(3, _omitFieldNames ? '' : 'clientKind')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Hello clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Hello copyWith(void Function(Hello) updates) =>
      super.copyWith((message) => updates(message as Hello)) as Hello;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Hello() / Hello.new instead')
  static Hello create() => Hello._();
  static $pb.GeneratedMessage $_createMessage() => Hello._();
  @$core.override
  Hello createEmptyInstance() => Hello._();
  @$core.pragma('dart2js:noInline')
  static Hello getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Hello>(Hello.$_createMessage);
  static Hello? _defaultInstance;

  /// Tel protokolü sürümü; uyumsuzsa sunucu bağlantıyı Welcome yerine
  /// hata ile kapatır.
  @$pb.TagNumber(1)
  $core.int get protocolVersion => $_getIZ(0);
  @$pb.TagNumber(1)
  set protocolVersion($core.int value) => $_setUnsignedInt32(0, value);
  @$pb.TagNumber(1)
  $core.bool hasProtocolVersion() => $_has(0);
  @$pb.TagNumber(1)
  void clearProtocolVersion() => $_clearField(1);

  /// Katılım kodu (Faz 0'da boş bırakılabilir).
  @$pb.TagNumber(2)
  $core.String get joinCode => $_getSZ(1);
  @$pb.TagNumber(2)
  set joinCode($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasJoinCode() => $_has(1);
  @$pb.TagNumber(2)
  void clearJoinCode() => $_clearField(2);

  /// Tanılama için istemci türü: "flutter", "loadgen", "web".
  @$pb.TagNumber(3)
  $core.String get clientKind => $_getSZ(2);
  @$pb.TagNumber(3)
  set clientKind($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasClientKind() => $_has(2);
  @$pb.TagNumber(3)
  void clearClientKind() => $_clearField(3);
}

class Welcome extends $pb.GeneratedMessage {
  factory Welcome({
    $fixnum.Int64? serverTimeMs,
    $core.int? protocolVersion,
    $core.String? roomId,
  }) {
    final result = Welcome._();
    if (serverTimeMs != null) result.serverTimeMs = serverTimeMs;
    if (protocolVersion != null) result.protocolVersion = protocolVersion;
    if (roomId != null) result.roomId = roomId;
    return result;
  }

  Welcome._();

  factory Welcome.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Welcome()..mergeFromBuffer(data, registry);
  factory Welcome.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Welcome()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Welcome',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'tekses.v1'),
      createEmptyInstance: Welcome.$_createMessage)
    ..aInt64(1, _omitFieldNames ? '' : 'serverTimeMs')
    ..aI(2, _omitFieldNames ? '' : 'protocolVersion',
        fieldType: $pb.PbFieldType.OU3)
    ..aOS(3, _omitFieldNames ? '' : 'roomId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Welcome clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Welcome copyWith(void Function(Welcome) updates) =>
      super.copyWith((message) => updates(message as Welcome)) as Welcome;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Welcome() / Welcome.new instead')
  static Welcome create() => Welcome._();
  static $pb.GeneratedMessage $_createMessage() => Welcome._();
  @$core.override
  Welcome createEmptyInstance() => Welcome._();
  @$core.pragma('dart2js:noInline')
  static Welcome getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Welcome>(Welcome.$_createMessage);
  static Welcome? _defaultInstance;

  /// Sunucu saati (ms); ilk kaba ofset tahmini için.
  @$pb.TagNumber(1)
  $fixnum.Int64 get serverTimeMs => $_getI64(0);
  @$pb.TagNumber(1)
  set serverTimeMs($fixnum.Int64 value) => $_setInt64(0, value);
  @$pb.TagNumber(1)
  $core.bool hasServerTimeMs() => $_has(0);
  @$pb.TagNumber(1)
  void clearServerTimeMs() => $_clearField(1);

  /// Sunucunun kabul ettiği protokol sürümü.
  @$pb.TagNumber(2)
  $core.int get protocolVersion => $_getIZ(1);
  @$pb.TagNumber(2)
  set protocolVersion($core.int value) => $_setUnsignedInt32(1, value);
  @$pb.TagNumber(2)
  $core.bool hasProtocolVersion() => $_has(1);
  @$pb.TagNumber(2)
  void clearProtocolVersion() => $_clearField(2);

  /// Oda kimliği (Faz 0: "faz0").
  @$pb.TagNumber(3)
  $core.String get roomId => $_getSZ(2);
  @$pb.TagNumber(3)
  set roomId($core.String value) => $_setString(2, value);
  @$pb.TagNumber(3)
  $core.bool hasRoomId() => $_has(2);
  @$pb.TagNumber(3)
  void clearRoomId() => $_clearField(3);
}

/// Odada yeni gösteri sürümü etkinleştirildi: istemci katılım bilgisini
/// (manifest + varlıklar) yeniden indirmelidir; süren koreografi etkilenmez.
class ShowActivated extends $pb.GeneratedMessage {
  factory ShowActivated({
    $core.String? roomId,
    $core.String? showVersionId,
  }) {
    final result = ShowActivated._();
    if (roomId != null) result.roomId = roomId;
    if (showVersionId != null) result.showVersionId = showVersionId;
    return result;
  }

  ShowActivated._();

  factory ShowActivated.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ShowActivated()..mergeFromBuffer(data, registry);
  factory ShowActivated.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      ShowActivated()..mergeFromJson(json, registry);

  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'ShowActivated',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'tekses.v1'),
      createEmptyInstance: ShowActivated.$_createMessage)
    ..aOS(1, _omitFieldNames ? '' : 'roomId')
    ..aOS(2, _omitFieldNames ? '' : 'showVersionId')
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ShowActivated clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  ShowActivated copyWith(void Function(ShowActivated) updates) =>
      super.copyWith((message) => updates(message as ShowActivated))
          as ShowActivated;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use ShowActivated() / ShowActivated.new instead')
  static ShowActivated create() => ShowActivated._();
  static $pb.GeneratedMessage $_createMessage() => ShowActivated._();
  @$core.override
  ShowActivated createEmptyInstance() => ShowActivated._();
  @$core.pragma('dart2js:noInline')
  static ShowActivated getDefault() =>
      _defaultInstance ??= $pb.GeneratedMessage.$_defaultFor<ShowActivated>(
          ShowActivated.$_createMessage);
  static ShowActivated? _defaultInstance;

  @$pb.TagNumber(1)
  $core.String get roomId => $_getSZ(0);
  @$pb.TagNumber(1)
  set roomId($core.String value) => $_setString(0, value);
  @$pb.TagNumber(1)
  $core.bool hasRoomId() => $_has(0);
  @$pb.TagNumber(1)
  void clearRoomId() => $_clearField(1);

  /// Etkinleştirilen sürüm (tanılama için; istemci her durumda tazeler).
  @$pb.TagNumber(2)
  $core.String get showVersionId => $_getSZ(1);
  @$pb.TagNumber(2)
  set showVersionId($core.String value) => $_setString(1, value);
  @$pb.TagNumber(2)
  $core.bool hasShowVersionId() => $_has(1);
  @$pb.TagNumber(2)
  void clearShowVersionId() => $_clearField(2);
}

enum Envelope_Kind {
  hello,
  welcome,
  clockSyncRequest,
  clockSyncResponse,
  cueStart,
  intervention,
  showActivated,
  notSet
}

/// Sunucu ile istemci arasındaki tek WebSocket çerçevesi.
/// Faz 0'da tel JSON'dur ve bu şemayı alan adlarıyla birebir izler;
/// Faz 1'de ikili protobuf'a (~40 bayt) geçilir.
class Envelope extends $pb.GeneratedMessage {
  factory Envelope({
    Hello? hello,
    Welcome? welcome,
    $0.ClockSyncRequest? clockSyncRequest,
    $0.ClockSyncResponse? clockSyncResponse,
    $1.CueStart? cueStart,
    $1.Intervention? intervention,
    ShowActivated? showActivated,
  }) {
    final result = Envelope._();
    if (hello != null) result.hello = hello;
    if (welcome != null) result.welcome = welcome;
    if (clockSyncRequest != null) result.clockSyncRequest = clockSyncRequest;
    if (clockSyncResponse != null) result.clockSyncResponse = clockSyncResponse;
    if (cueStart != null) result.cueStart = cueStart;
    if (intervention != null) result.intervention = intervention;
    if (showActivated != null) result.showActivated = showActivated;
    return result;
  }

  Envelope._();

  factory Envelope.fromBuffer($core.List<$core.int> data,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Envelope()..mergeFromBuffer(data, registry);
  factory Envelope.fromJson($core.String json,
          [$pb.ExtensionRegistry registry = $pb.ExtensionRegistry.EMPTY]) =>
      Envelope()..mergeFromJson(json, registry);

  static const $core.Map<$core.int, Envelope_Kind> _Envelope_KindByTag = {
    1: Envelope_Kind.hello,
    2: Envelope_Kind.welcome,
    3: Envelope_Kind.clockSyncRequest,
    4: Envelope_Kind.clockSyncResponse,
    5: Envelope_Kind.cueStart,
    6: Envelope_Kind.intervention,
    7: Envelope_Kind.showActivated,
    0: Envelope_Kind.notSet
  };
  static final $pb.BuilderInfo _i = $pb.BuilderInfo(
      _omitMessageNames ? '' : 'Envelope',
      package: const $pb.PackageName(_omitMessageNames ? '' : 'tekses.v1'),
      createEmptyInstance: Envelope.$_createMessage)
    ..oo(0, [1, 2, 3, 4, 5, 6, 7])
    ..aOM<Hello>(1, _omitFieldNames ? '' : 'hello',
        subBuilder: Hello.$_createMessage)
    ..aOM<Welcome>(2, _omitFieldNames ? '' : 'welcome',
        subBuilder: Welcome.$_createMessage)
    ..aOM<$0.ClockSyncRequest>(3, _omitFieldNames ? '' : 'clockSyncRequest',
        subBuilder: $0.ClockSyncRequest.$_createMessage)
    ..aOM<$0.ClockSyncResponse>(4, _omitFieldNames ? '' : 'clockSyncResponse',
        subBuilder: $0.ClockSyncResponse.$_createMessage)
    ..aOM<$1.CueStart>(5, _omitFieldNames ? '' : 'cueStart',
        subBuilder: $1.CueStart.$_createMessage)
    ..aOM<$1.Intervention>(6, _omitFieldNames ? '' : 'intervention',
        subBuilder: $1.Intervention.$_createMessage)
    ..aOM<ShowActivated>(7, _omitFieldNames ? '' : 'showActivated',
        subBuilder: ShowActivated.$_createMessage)
    ..hasRequiredFields = false;

  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Envelope clone() => deepCopy();
  @$core.Deprecated('See https://github.com/google/protobuf.dart/issues/998.')
  Envelope copyWith(void Function(Envelope) updates) =>
      super.copyWith((message) => updates(message as Envelope)) as Envelope;

  @$core.override
  $pb.BuilderInfo get info_ => _i;

  @$core.pragma('dart2js:noInline')
  @$core.Deprecated('Use Envelope() / Envelope.new instead')
  static Envelope create() => Envelope._();
  static $pb.GeneratedMessage $_createMessage() => Envelope._();
  @$core.override
  Envelope createEmptyInstance() => Envelope._();
  @$core.pragma('dart2js:noInline')
  static Envelope getDefault() => _defaultInstance ??=
      $pb.GeneratedMessage.$_defaultFor<Envelope>(Envelope.$_createMessage);
  static Envelope? _defaultInstance;

  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  @$pb.TagNumber(6)
  @$pb.TagNumber(7)
  Envelope_Kind whichKind() => _Envelope_KindByTag[$_whichOneof(0)]!;
  @$pb.TagNumber(1)
  @$pb.TagNumber(2)
  @$pb.TagNumber(3)
  @$pb.TagNumber(4)
  @$pb.TagNumber(5)
  @$pb.TagNumber(6)
  @$pb.TagNumber(7)
  void clearKind() => $_clearField($_whichOneof(0));

  @$pb.TagNumber(1)
  Hello get hello => $_getN(0);
  @$pb.TagNumber(1)
  set hello(Hello value) => $_setField(1, value);
  @$pb.TagNumber(1)
  $core.bool hasHello() => $_has(0);
  @$pb.TagNumber(1)
  void clearHello() => $_clearField(1);
  @$pb.TagNumber(1)
  Hello ensureHello() => $_ensure(0);

  @$pb.TagNumber(2)
  Welcome get welcome => $_getN(1);
  @$pb.TagNumber(2)
  set welcome(Welcome value) => $_setField(2, value);
  @$pb.TagNumber(2)
  $core.bool hasWelcome() => $_has(1);
  @$pb.TagNumber(2)
  void clearWelcome() => $_clearField(2);
  @$pb.TagNumber(2)
  Welcome ensureWelcome() => $_ensure(1);

  @$pb.TagNumber(3)
  $0.ClockSyncRequest get clockSyncRequest => $_getN(2);
  @$pb.TagNumber(3)
  set clockSyncRequest($0.ClockSyncRequest value) => $_setField(3, value);
  @$pb.TagNumber(3)
  $core.bool hasClockSyncRequest() => $_has(2);
  @$pb.TagNumber(3)
  void clearClockSyncRequest() => $_clearField(3);
  @$pb.TagNumber(3)
  $0.ClockSyncRequest ensureClockSyncRequest() => $_ensure(2);

  @$pb.TagNumber(4)
  $0.ClockSyncResponse get clockSyncResponse => $_getN(3);
  @$pb.TagNumber(4)
  set clockSyncResponse($0.ClockSyncResponse value) => $_setField(4, value);
  @$pb.TagNumber(4)
  $core.bool hasClockSyncResponse() => $_has(3);
  @$pb.TagNumber(4)
  void clearClockSyncResponse() => $_clearField(4);
  @$pb.TagNumber(4)
  $0.ClockSyncResponse ensureClockSyncResponse() => $_ensure(3);

  @$pb.TagNumber(5)
  $1.CueStart get cueStart => $_getN(4);
  @$pb.TagNumber(5)
  set cueStart($1.CueStart value) => $_setField(5, value);
  @$pb.TagNumber(5)
  $core.bool hasCueStart() => $_has(4);
  @$pb.TagNumber(5)
  void clearCueStart() => $_clearField(5);
  @$pb.TagNumber(5)
  $1.CueStart ensureCueStart() => $_ensure(4);

  @$pb.TagNumber(6)
  $1.Intervention get intervention => $_getN(5);
  @$pb.TagNumber(6)
  set intervention($1.Intervention value) => $_setField(6, value);
  @$pb.TagNumber(6)
  $core.bool hasIntervention() => $_has(5);
  @$pb.TagNumber(6)
  void clearIntervention() => $_clearField(6);
  @$pb.TagNumber(6)
  $1.Intervention ensureIntervention() => $_ensure(5);

  @$pb.TagNumber(7)
  ShowActivated get showActivated => $_getN(6);
  @$pb.TagNumber(7)
  set showActivated(ShowActivated value) => $_setField(7, value);
  @$pb.TagNumber(7)
  $core.bool hasShowActivated() => $_has(6);
  @$pb.TagNumber(7)
  void clearShowActivated() => $_clearField(7);
  @$pb.TagNumber(7)
  ShowActivated ensureShowActivated() => $_ensure(6);
}

const $core.bool _omitFieldNames =
    $core.bool.fromEnvironment('protobuf.omit_field_names');
const $core.bool _omitMessageNames =
    $core.bool.fromEnvironment('protobuf.omit_message_names');

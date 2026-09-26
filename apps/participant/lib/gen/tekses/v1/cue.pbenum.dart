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

import 'package:protobuf/protobuf.dart' as $pb;

class InterventionKind extends $pb.ProtobufEnum {
  static const InterventionKind INTERVENTION_KIND_UNSPECIFIED =
      InterventionKind._(
          0, _omitEnumNames ? '' : 'INTERVENTION_KIND_UNSPECIFIED');

  /// Akışı duraklat; ekran son karede kalır.
  static const InterventionKind INTERVENTION_KIND_HOLD =
      InterventionKind._(1, _omitEnumNames ? '' : 'INTERVENTION_KIND_HOLD');

  /// Çalıştırmayı bitir; ekran karartılır, fener söner.
  static const InterventionKind INTERVENTION_KIND_STOP =
      InterventionKind._(2, _omitEnumNames ? '' : 'INTERVENTION_KIND_STOP');

  /// Sıradaki sekansa atla.
  static const InterventionKind INTERVENTION_KIND_SKIP =
      InterventionKind._(3, _omitEnumNames ? '' : 'INTERVENTION_KIND_SKIP');

  /// Acil karartma: tüm ışık ve ekran efektleri anında söner.
  static const InterventionKind INTERVENTION_KIND_BLACKOUT =
      InterventionKind._(4, _omitEnumNames ? '' : 'INTERVENTION_KIND_BLACKOUT');

  static const $core.List<InterventionKind> values = <InterventionKind>[
    INTERVENTION_KIND_UNSPECIFIED,
    INTERVENTION_KIND_HOLD,
    INTERVENTION_KIND_STOP,
    INTERVENTION_KIND_SKIP,
    INTERVENTION_KIND_BLACKOUT,
  ];

  static final $core.List<InterventionKind?> _byValue =
      $pb.ProtobufEnum.$_initByValueList(values, 4);
  static InterventionKind? valueOf($core.int value) =>
      value < 0 || value >= _byValue.length ? null : _byValue[value];

  const InterventionKind._(super.value, super.name);
}

const $core.bool _omitEnumNames =
    $core.bool.fromEnvironment('protobuf.omit_enum_names');

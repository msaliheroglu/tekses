// This is a generated file - do not edit.
//
// Generated from tekses/v1/cue.proto.

// @dart = 3.3

// ignore_for_file: annotate_overrides, camel_case_types, comment_references
// ignore_for_file: constant_identifier_names
// ignore_for_file: curly_braces_in_flow_control_structures
// ignore_for_file: deprecated_member_use_from_same_package, library_prefixes
// ignore_for_file: non_constant_identifier_names, prefer_relative_imports
// ignore_for_file: unused_import

import 'dart:convert' as $convert;
import 'dart:core' as $core;
import 'dart:typed_data' as $typed_data;

@$core.Deprecated('Use interventionKindDescriptor instead')
const InterventionKind$json = {
  '1': 'InterventionKind',
  '2': [
    {'1': 'INTERVENTION_KIND_UNSPECIFIED', '2': 0},
    {'1': 'INTERVENTION_KIND_HOLD', '2': 1},
    {'1': 'INTERVENTION_KIND_STOP', '2': 2},
    {'1': 'INTERVENTION_KIND_SKIP', '2': 3},
    {'1': 'INTERVENTION_KIND_BLACKOUT', '2': 4},
  ],
};

/// Descriptor for `InterventionKind`. Decode as a `google.protobuf.EnumDescriptorProto`.
final $typed_data.Uint8List interventionKindDescriptor = $convert.base64Decode(
    'ChBJbnRlcnZlbnRpb25LaW5kEiEKHUlOVEVSVkVOVElPTl9LSU5EX1VOU1BFQ0lGSUVEEAASGg'
    'oWSU5URVJWRU5USU9OX0tJTkRfSE9MRBABEhoKFklOVEVSVkVOVElPTl9LSU5EX1NUT1AQAhIa'
    'ChZJTlRFUlZFTlRJT05fS0lORF9TS0lQEAMSHgoaSU5URVJWRU5USU9OX0tJTkRfQkxBQ0tPVV'
    'QQBA==');

@$core.Deprecated('Use cueStartDescriptor instead')
const CueStart$json = {
  '1': 'CueStart',
  '2': [
    {'1': 'run_id', '3': 1, '4': 1, '5': 9, '10': 'runId'},
    {'1': 'cue_id', '3': 2, '4': 1, '5': 9, '10': 'cueId'},
    {'1': 'fire_at_server_ms', '3': 3, '4': 1, '5': 3, '10': 'fireAtServerMs'},
    {'1': 'repeat_seq', '3': 4, '4': 1, '5': 13, '10': 'repeatSeq'},
    {
      '1': 'payload',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.tekses.v1.CuePayload',
      '10': 'payload'
    },
  ],
};

/// Descriptor for `CueStart`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cueStartDescriptor = $convert.base64Decode(
    'CghDdWVTdGFydBIVCgZydW5faWQYASABKAlSBXJ1bklkEhUKBmN1ZV9pZBgCIAEoCVIFY3VlSW'
    'QSKQoRZmlyZV9hdF9zZXJ2ZXJfbXMYAyABKANSDmZpcmVBdFNlcnZlck1zEh0KCnJlcGVhdF9z'
    'ZXEYBCABKA1SCXJlcGVhdFNlcRIvCgdwYXlsb2FkGAUgASgLMhUudGVrc2VzLnYxLkN1ZVBheW'
    'xvYWRSB3BheWxvYWQ=');

@$core.Deprecated('Use cuePayloadDescriptor instead')
const CuePayload$json = {
  '1': 'CuePayload',
  '2': [
    {'1': 'color', '3': 1, '4': 1, '5': 9, '10': 'color'},
    {'1': 'torch', '3': 2, '4': 1, '5': 8, '10': 'torch'},
    {'1': 'flash_hz', '3': 3, '4': 1, '5': 13, '10': 'flashHz'},
    {'1': 'duration_ms', '3': 4, '4': 1, '5': 13, '10': 'durationMs'},
  ],
};

/// Descriptor for `CuePayload`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List cuePayloadDescriptor = $convert.base64Decode(
    'CgpDdWVQYXlsb2FkEhQKBWNvbG9yGAEgASgJUgVjb2xvchIUCgV0b3JjaBgCIAEoCFIFdG9yY2'
    'gSGQoIZmxhc2hfaHoYAyABKA1SB2ZsYXNoSHoSHwoLZHVyYXRpb25fbXMYBCABKA1SCmR1cmF0'
    'aW9uTXM=');

@$core.Deprecated('Use interventionDescriptor instead')
const Intervention$json = {
  '1': 'Intervention',
  '2': [
    {'1': 'run_id', '3': 1, '4': 1, '5': 9, '10': 'runId'},
    {
      '1': 'kind',
      '3': 2,
      '4': 1,
      '5': 14,
      '6': '.tekses.v1.InterventionKind',
      '10': 'kind'
    },
    {
      '1': 'issued_at_server_ms',
      '3': 3,
      '4': 1,
      '5': 3,
      '10': 'issuedAtServerMs'
    },
  ],
};

/// Descriptor for `Intervention`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List interventionDescriptor = $convert.base64Decode(
    'CgxJbnRlcnZlbnRpb24SFQoGcnVuX2lkGAEgASgJUgVydW5JZBIvCgRraW5kGAIgASgOMhsudG'
    'Vrc2VzLnYxLkludGVydmVudGlvbktpbmRSBGtpbmQSLQoTaXNzdWVkX2F0X3NlcnZlcl9tcxgD'
    'IAEoA1IQaXNzdWVkQXRTZXJ2ZXJNcw==');

// This is a generated file - do not edit.
//
// Generated from tekses/v1/envelope.proto.

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

@$core.Deprecated('Use helloDescriptor instead')
const Hello$json = {
  '1': 'Hello',
  '2': [
    {'1': 'protocol_version', '3': 1, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'join_code', '3': 2, '4': 1, '5': 9, '10': 'joinCode'},
    {'1': 'client_kind', '3': 3, '4': 1, '5': 9, '10': 'clientKind'},
  ],
};

/// Descriptor for `Hello`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List helloDescriptor = $convert.base64Decode(
    'CgVIZWxsbxIpChBwcm90b2NvbF92ZXJzaW9uGAEgASgNUg9wcm90b2NvbFZlcnNpb24SGwoJam'
    '9pbl9jb2RlGAIgASgJUghqb2luQ29kZRIfCgtjbGllbnRfa2luZBgDIAEoCVIKY2xpZW50S2lu'
    'ZA==');

@$core.Deprecated('Use welcomeDescriptor instead')
const Welcome$json = {
  '1': 'Welcome',
  '2': [
    {'1': 'server_time_ms', '3': 1, '4': 1, '5': 3, '10': 'serverTimeMs'},
    {'1': 'protocol_version', '3': 2, '4': 1, '5': 13, '10': 'protocolVersion'},
    {'1': 'room_id', '3': 3, '4': 1, '5': 9, '10': 'roomId'},
  ],
};

/// Descriptor for `Welcome`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List welcomeDescriptor = $convert.base64Decode(
    'CgdXZWxjb21lEiQKDnNlcnZlcl90aW1lX21zGAEgASgDUgxzZXJ2ZXJUaW1lTXMSKQoQcHJvdG'
    '9jb2xfdmVyc2lvbhgCIAEoDVIPcHJvdG9jb2xWZXJzaW9uEhcKB3Jvb21faWQYAyABKAlSBnJv'
    'b21JZA==');

@$core.Deprecated('Use showActivatedDescriptor instead')
const ShowActivated$json = {
  '1': 'ShowActivated',
  '2': [
    {'1': 'room_id', '3': 1, '4': 1, '5': 9, '10': 'roomId'},
    {'1': 'show_version_id', '3': 2, '4': 1, '5': 9, '10': 'showVersionId'},
  ],
};

/// Descriptor for `ShowActivated`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List showActivatedDescriptor = $convert.base64Decode(
    'Cg1TaG93QWN0aXZhdGVkEhcKB3Jvb21faWQYASABKAlSBnJvb21JZBImCg9zaG93X3ZlcnNpb2'
    '5faWQYAiABKAlSDXNob3dWZXJzaW9uSWQ=');

@$core.Deprecated('Use envelopeDescriptor instead')
const Envelope$json = {
  '1': 'Envelope',
  '2': [
    {
      '1': 'hello',
      '3': 1,
      '4': 1,
      '5': 11,
      '6': '.tekses.v1.Hello',
      '9': 0,
      '10': 'hello'
    },
    {
      '1': 'welcome',
      '3': 2,
      '4': 1,
      '5': 11,
      '6': '.tekses.v1.Welcome',
      '9': 0,
      '10': 'welcome'
    },
    {
      '1': 'clock_sync_request',
      '3': 3,
      '4': 1,
      '5': 11,
      '6': '.tekses.v1.ClockSyncRequest',
      '9': 0,
      '10': 'clockSyncRequest'
    },
    {
      '1': 'clock_sync_response',
      '3': 4,
      '4': 1,
      '5': 11,
      '6': '.tekses.v1.ClockSyncResponse',
      '9': 0,
      '10': 'clockSyncResponse'
    },
    {
      '1': 'cue_start',
      '3': 5,
      '4': 1,
      '5': 11,
      '6': '.tekses.v1.CueStart',
      '9': 0,
      '10': 'cueStart'
    },
    {
      '1': 'intervention',
      '3': 6,
      '4': 1,
      '5': 11,
      '6': '.tekses.v1.Intervention',
      '9': 0,
      '10': 'intervention'
    },
    {
      '1': 'show_activated',
      '3': 7,
      '4': 1,
      '5': 11,
      '6': '.tekses.v1.ShowActivated',
      '9': 0,
      '10': 'showActivated'
    },
  ],
  '8': [
    {'1': 'kind'},
  ],
};

/// Descriptor for `Envelope`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List envelopeDescriptor = $convert.base64Decode(
    'CghFbnZlbG9wZRIoCgVoZWxsbxgBIAEoCzIQLnRla3Nlcy52MS5IZWxsb0gAUgVoZWxsbxIuCg'
    'd3ZWxjb21lGAIgASgLMhIudGVrc2VzLnYxLldlbGNvbWVIAFIHd2VsY29tZRJLChJjbG9ja19z'
    'eW5jX3JlcXVlc3QYAyABKAsyGy50ZWtzZXMudjEuQ2xvY2tTeW5jUmVxdWVzdEgAUhBjbG9ja1'
    'N5bmNSZXF1ZXN0Ek4KE2Nsb2NrX3N5bmNfcmVzcG9uc2UYBCABKAsyHC50ZWtzZXMudjEuQ2xv'
    'Y2tTeW5jUmVzcG9uc2VIAFIRY2xvY2tTeW5jUmVzcG9uc2USMgoJY3VlX3N0YXJ0GAUgASgLMh'
    'MudGVrc2VzLnYxLkN1ZVN0YXJ0SABSCGN1ZVN0YXJ0Ej0KDGludGVydmVudGlvbhgGIAEoCzIX'
    'LnRla3Nlcy52MS5JbnRlcnZlbnRpb25IAFIMaW50ZXJ2ZW50aW9uEkEKDnNob3dfYWN0aXZhdG'
    'VkGAcgASgLMhgudGVrc2VzLnYxLlNob3dBY3RpdmF0ZWRIAFINc2hvd0FjdGl2YXRlZEIGCgRr'
    'aW5k');

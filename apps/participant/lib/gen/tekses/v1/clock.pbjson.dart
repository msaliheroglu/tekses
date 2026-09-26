// This is a generated file - do not edit.
//
// Generated from tekses/v1/clock.proto.

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

@$core.Deprecated('Use clockSyncRequestDescriptor instead')
const ClockSyncRequest$json = {
  '1': 'ClockSyncRequest',
  '2': [
    {'1': 'seq', '3': 1, '4': 1, '5': 13, '10': 'seq'},
    {'1': 'client_mono_ms', '3': 2, '4': 1, '5': 3, '10': 'clientMonoMs'},
  ],
};

/// Descriptor for `ClockSyncRequest`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clockSyncRequestDescriptor = $convert.base64Decode(
    'ChBDbG9ja1N5bmNSZXF1ZXN0EhAKA3NlcRgBIAEoDVIDc2VxEiQKDmNsaWVudF9tb25vX21zGA'
    'IgASgDUgxjbGllbnRNb25vTXM=');

@$core.Deprecated('Use clockSyncResponseDescriptor instead')
const ClockSyncResponse$json = {
  '1': 'ClockSyncResponse',
  '2': [
    {'1': 'seq', '3': 1, '4': 1, '5': 13, '10': 'seq'},
    {'1': 'client_mono_ms', '3': 2, '4': 1, '5': 3, '10': 'clientMonoMs'},
    {'1': 'server_recv_ms', '3': 3, '4': 1, '5': 3, '10': 'serverRecvMs'},
    {'1': 'server_send_ms', '3': 4, '4': 1, '5': 3, '10': 'serverSendMs'},
  ],
};

/// Descriptor for `ClockSyncResponse`. Decode as a `google.protobuf.DescriptorProto`.
final $typed_data.Uint8List clockSyncResponseDescriptor = $convert.base64Decode(
    'ChFDbG9ja1N5bmNSZXNwb25zZRIQCgNzZXEYASABKA1SA3NlcRIkCg5jbGllbnRfbW9ub19tcx'
    'gCIAEoA1IMY2xpZW50TW9ub01zEiQKDnNlcnZlcl9yZWN2X21zGAMgASgDUgxzZXJ2ZXJSZWN2'
    'TXMSJAoOc2VydmVyX3NlbmRfbXMYBCABKANSDHNlcnZlclNlbmRNcw==');

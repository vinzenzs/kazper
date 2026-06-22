import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../domain/garmin.dart';
import 'app_providers.dart';

/// Garmin connect flow phases. The bridge holds the credentials, so the app
/// only triggers login and (when Garmin demands it) relays the MFA code:
/// idle → triggering → (awaitingMfa | connected | disabled | error),
/// awaitingMfa → submittingMfa → (connected | awaitingMfa-with-error | error).
enum GarminConnectPhase {
  idle,
  triggering,
  awaitingMfa,
  submittingMfa,
  connected,
  disabled,
  error,
}

class GarminConnectState {
  final GarminConnectPhase phase;
  final String? error;

  const GarminConnectState({this.phase = GarminConnectPhase.idle, this.error});

  bool get isBusy =>
      phase == GarminConnectPhase.triggering ||
      phase == GarminConnectPhase.submittingMfa;
}

/// Drives the existing login proxy (`POST /garmin/login`, `POST /garmin/login/mfa`).
/// Never collects email/password — those live in the bridge's config.
class GarminConnectNotifier extends Notifier<GarminConnectState> {
  @override
  GarminConnectState build() => const GarminConnectState();

  Dio get _dio => ref.read(apiClientProvider).dio;

  static final _opts = Options(validateStatus: (_) => true);

  /// Trigger login. A `{needs_mfa:true}` response moves to MFA entry; a
  /// `{logged_in:true}` connects directly.
  Future<void> connect() async {
    state = const GarminConnectState(phase: GarminConnectPhase.triggering);
    try {
      final resp = await _dio.post<Map<String, dynamic>>('/garmin/login', options: _opts);
      _apply(resp, fromMfa: false);
    } catch (_) {
      state = const GarminConnectState(
        phase: GarminConnectPhase.error,
        error: "Couldn't reach the server.",
      );
    }
  }

  /// Submit the 6-digit MFA code the user read from their authenticator.
  Future<void> submitMfa(String code) async {
    state = const GarminConnectState(phase: GarminConnectPhase.submittingMfa);
    try {
      final resp = await _dio.post<Map<String, dynamic>>(
        '/garmin/login/mfa',
        data: {'code': code.trim()},
        options: _opts,
      );
      _apply(resp, fromMfa: true);
    } catch (_) {
      state = const GarminConnectState(
        phase: GarminConnectPhase.error,
        error: "Couldn't reach the server.",
      );
    }
  }

  void _apply(Response<Map<String, dynamic>> resp, {required bool fromMfa}) {
    final code = resp.statusCode ?? 0;
    final body = resp.data ?? const <String, dynamic>{};
    if (code == 503) {
      state = const GarminConnectState(phase: GarminConnectPhase.disabled);
      return;
    }
    if (code >= 200 && code < 300) {
      if (body['needs_mfa'] == true) {
        state = const GarminConnectState(phase: GarminConnectPhase.awaitingMfa);
        return;
      }
      if (body['logged_in'] == true) {
        state = const GarminConnectState(phase: GarminConnectPhase.connected);
        ref.invalidate(garminSyncProvider); // refresh freshness after reconnect
        return;
      }
    }
    // Error: map the bridge's typed codes; a wrong MFA code stays on the entry
    // screen so the user can retype it.
    final err = body['error'] as String?;
    state = GarminConnectState(
      phase: (fromMfa && err == 'mfa_invalid')
          ? GarminConnectPhase.awaitingMfa
          : GarminConnectPhase.error,
      error: _messageFor(err),
    );
  }

  String _messageFor(String? err) {
    switch (err) {
      case 'mfa_invalid':
        return 'That code was wrong or expired — try again.';
      case 'bad_credentials':
        return 'Garmin rejected the stored credentials.';
      case 'locked_out':
        return 'Garmin temporarily locked the account. Wait and retry.';
      case 'garmin_disabled':
        return "Garmin isn't configured on the server.";
      default:
        return 'Garmin login failed. Try again.';
    }
  }

  void reset() => state = const GarminConnectState();
}

final garminConnectProvider =
    NotifierProvider<GarminConnectNotifier, GarminConnectState>(
  GarminConnectNotifier.new,
);

/// Read-only Garmin sync status. Returns null when the integration is
/// unconfigured (`503 garmin_disabled`) so the UI can show "not configured".
class GarminSyncNotifier extends AsyncNotifier<GarminSyncStatus?> {
  @override
  Future<GarminSyncStatus?> build() => _fetch();

  Future<GarminSyncStatus?> _fetch() async {
    final resp = await ref.read(apiClientProvider).dio.get<Map<String, dynamic>>(
          '/garmin/sync-status',
          options: Options(validateStatus: (_) => true),
        );
    if (resp.statusCode == 503) return null;
    if (resp.statusCode != 200 || resp.data == null) {
      throw Exception('sync status ${resp.statusCode}');
    }
    return GarminSyncStatus.fromJson(resp.data!);
  }

  Future<void> refresh() async {
    state = await AsyncValue.guard(_fetch);
  }
}

final garminSyncProvider =
    AsyncNotifierProvider<GarminSyncNotifier, GarminSyncStatus?>(
  GarminSyncNotifier.new,
);

import 'dart:async';

import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';

import '../data/push/push_messaging.dart';
import 'app_providers.dart';

/// What a received push asks the app to do. Forward-compatible: anything the
/// app doesn't recognise maps to [none] and is ignored without error.
enum PushIntent { garminRelogin, none }

/// Pure routing: maps a push to an intent off the backend's `data.action` key,
/// so app and server stay decoupled from the notification's display copy.
PushIntent intentFor(PushMessage m) =>
    m.action == 'garmin_relogin' ? PushIntent.garminRelogin : PushIntent.none;

class PushState {
  /// Whether the OS notification permission is granted. Optimistically true
  /// until [PushNotifier.register] resolves the permission, so the settings
  /// "notifications off" hint doesn't flash before the gate is known.
  final bool notificationsEnabled;

  const PushState({this.notificationsEnabled = true});

  PushState copyWith({bool? notificationsEnabled}) => PushState(
        notificationsEnabled: notificationsEnabled ?? this.notificationsEnabled,
      );
}

/// Owns FCM token registration tied to the pairing lifecycle. On pair it
/// requests permission, obtains the token, and `POST`s it; re-`POST`s on token
/// refresh; `DELETE`s on unpair. Registration proceeds even when permission is
/// denied (so enabling notifications later needs no re-pair). Mirrors the Garmin
/// provider's direct-dio precedent for the `/push/tokens` calls.
class PushNotifier extends Notifier<PushState> {
  static final _opts = Options(validateStatus: (_) => true);

  String? _lastToken;
  StreamSubscription<String>? _refreshSub;
  bool _wired = false;

  @override
  PushState build() {
    ref.onDispose(() => _refreshSub?.cancel());
    return const PushState();
  }

  Dio get _dio => ref.read(apiClientProvider).dio;
  PushMessaging get _messaging => ref.read(pushMessagingProvider);

  /// Called from the paired branch. Resolves the notification permission,
  /// registers the current token, and wires re-registration on token refresh.
  /// Idempotent — safe to call on every launch (the server upserts by token).
  Future<void> register() async {
    final perm = await _messaging.requestPermission();
    state = state.copyWith(
        notificationsEnabled: perm == PushPermission.granted);

    // Register regardless of the permission outcome.
    final token = await _messaging.getToken();
    if (token != null && token.isNotEmpty) {
      await _post(token);
    }

    if (!_wired) {
      _wired = true;
      _refreshSub = _messaging.onTokenRefresh.listen(_post);
    }
  }

  /// Drops the last-registered token on unpair. Best-effort.
  Future<void> deregister() async {
    final token = _lastToken;
    if (token == null) return;
    try {
      await _dio.delete<dynamic>('/push/tokens',
          data: {'token': token}, options: _opts);
    } catch (_) {
      // Best-effort: an unreachable server just leaves a stale token the
      // backend will prune when delivery fails.
    }
    _lastToken = null;
  }

  Future<void> _post(String token) async {
    _lastToken = token;
    try {
      await _dio.post<dynamic>('/push/tokens',
          data: {'token': token, 'platform': 'android'}, options: _opts);
    } catch (_) {
      // Best-effort: re-registered on the next launch (the recovery path).
    }
  }
}

final pushProvider =
    NotifierProvider<PushNotifier, PushState>(PushNotifier.new);
